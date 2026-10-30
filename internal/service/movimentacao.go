package service

import (
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/shopspring/decimal"
	"github.com/unicsul-finops/financial-operations-service/internal/domain"
)

// pagamento de parcela (RN-14, RN-15, RN-19) e liquidação antecipada (RN-18).
type RepositorioMovimentacoes interface {
	ConsultarCarteira(id int64, numero string) (domain.Carteira, error)
	// trava a operação, procura o reenvio pelo identificador e, se não for reenvio, grava o que decidir devolver.
	// tudo numa transação, pra dois pagamentos ao mesmo tempo não calcularem em cima do mesmo saldo.
	Movimentar(id int64, numero, identificadorExterno string, decidir func(domain.Carteira) (domain.NovoEvento, error)) (domain.EventoGravado, bool, error)
}

var errIdentificadorJaUsado = errors.New("identificadorPagamento já foi usado em outra movimentação da operação")

type movimento struct {
	valor         decimal.Decimal
	data          time.Time
	identificador string
	origem        string
}

func (m movimento) dia() string { return m.data.Format(time.DateOnly) }

// valida o corpo antes de abrir a transação. as regras que dependem da operação ficam no decidir.
func (s *OperacaoService) validarPedido(pedido domain.PedidoMovimentacao) (movimento, error) {
	var m movimento
	if !pedido.Valor.Valid {
		return m, errors.New("valor é obrigatório")
	}
	m.valor = pedido.Valor.Decimal
	if !m.valor.IsPositive() {
		return m, errors.New("valor deve ser maior que zero")
	}
	if !m.valor.Equal(m.valor.Round(2)) {
		return m, errors.New("valor deve ter no máximo 2 casas decimais")
	}

	base, err := s.validarPedidoSemValor(pedido.DataPagamento, pedido.IdentificadorPagamento)
	if err != nil {
		return m, err
	}
	base.valor = m.valor
	if pedido.Origem != nil {
		base.origem = strings.TrimSpace(*pedido.Origem)
		if len([]rune(base.origem)) > 30 {
			return base, errors.New("origem deve ter no máximo 30 caracteres")
		}
	}
	return base, nil
}

// data e identificador, comuns a tudo o que movimenta dinheiro (RN-09 e RN-14).
func (s *OperacaoService) validarPedidoSemValor(dataPagamento, identificadorPagamento *string) (movimento, error) {
	var m movimento
	agora := s.agora()
	data, err := interpretarDataReferencia(dataPagamento, agora)
	if err != nil {
		return m, errors.New("dataPagamento deve estar no formato YYYY-MM-DD")
	}
	if hoje, _ := interpretarDataReferencia(nil, agora); data.After(hoje) {
		return m, errors.New("dataPagamento não pode ser futura")
	}
	m.data = data

	if identificadorPagamento != nil {
		m.identificador = strings.TrimSpace(*identificadorPagamento)
		if err := validarTexto("identificadorPagamento", m.identificador, 50); err != nil {
			return m, err
		}
	}
	return m, nil
}

// RN-09: a data do dinheiro não volta antes da liberação nem de uma movimentação já gravada.
func validarDataMovimento(m movimento, c domain.Carteira) error {
	if m.dia() < c.Operacao.DataLiberacao {
		return errors.New("dataPagamento não pode ser anterior à dataLiberacao da operação")
	}
	if c.UltimaMovimentacao != "" && m.dia() < c.UltimaMovimentacao {
		return errors.New("dataPagamento não pode ser anterior à data da última movimentação financeira da operação")
	}
	return nil
}

func lancamentosDaParcela(c domain.Carteira, numero int) []domain.LancamentoParcela {
	lancamentos := []domain.LancamentoParcela{}
	for _, l := range c.Lancamentos {
		if l.NumeroParcela == numero {
			lancamentos = append(lancamentos, l)
		}
	}
	return lancamentos
}

func (s *OperacaoService) PagarParcela(identificador, numeroParcela string, pedido domain.PedidoMovimentacao) (domain.Pagamento, bool, error) {
	id, numeroOperacao, err := identificarOperacao(identificador)
	if err != nil {
		return domain.Pagamento{}, false, err
	}
	numero, err := strconv.Atoi(numeroParcela)
	if err != nil || numero < 1 {
		return domain.Pagamento{}, false, errors.New("numero da parcela inválido, use um inteiro a partir de 1")
	}
	m, err := s.validarPedido(pedido)
	if err != nil {
		return domain.Pagamento{}, false, err
	}

	var pagamento domain.Pagamento
	gravado, novo, err := s.movimentacoes.Movimentar(id, numeroOperacao, m.identificador, func(c domain.Carteira) (domain.NovoEvento, error) {
		var evento domain.NovoEvento
		pagamento, evento, err = decidirPagamento(c, numero, m)
		return evento, err
	})
	if err != nil {
		return domain.Pagamento{}, false, err
	}
	if !novo {
		if gravado.Tipo != domain.EventoPagamento || gravado.NumeroParcela != numero {
			return domain.Pagamento{}, false, errIdentificadorJaUsado
		}
		if err := json.Unmarshal(gravado.Detalhe, &pagamento); err != nil {
			return domain.Pagamento{}, false, err
		}
	}
	pagamento.ID, pagamento.RegistradoEm = gravado.ID, gravado.RegistradoEm
	return pagamento, novo, nil
}

func decidirPagamento(c domain.Carteira, numero int, m movimento) (domain.Pagamento, domain.NovoEvento, error) {
	op := c.Operacao
	if op.Status == domain.StatusOperacaoLiquidada {
		return domain.Pagamento{}, domain.NovoEvento{}, errors.New("a operação está liquidada e não aceita mais pagamentos")
	}
	indice := -1
	for i, p := range c.Parcelas {
		if p.Numero == numero {
			indice = i
		}
	}
	if indice < 0 {
		return domain.Pagamento{}, domain.NovoEvento{}, domain.ErrParcelaNaoEncontrada
	}
	if err := validarDataMovimento(m, c); err != nil {
		return domain.Pagamento{}, domain.NovoEvento{}, err
	}
	parcela := c.Parcelas[indice]
	if parcela.Status == domain.StatusParcelaPaga {
		return domain.Pagamento{}, domain.NovoEvento{}, fmt.Errorf("parcela %d já está paga", numero)
	}

	// só as vencidas e a próxima a vencer; pagar mais longe é antecipação, que tem desconto (RN-17).
	liberada := parcela.Vencimento < m.dia()
	for _, p := range c.Parcelas {
		if p.Status != domain.StatusParcelaPaga && p.Vencimento >= m.dia() {
			liberada = liberada || p.Numero == numero
			break
		}
	}
	if !liberada {
		return domain.Pagamento{}, domain.NovoEvento{}, fmt.Errorf("a parcela %d ainda não pode ser paga, só as vencidas e a próxima a vencer", numero)
	}

	e, err := calcularEncargos(parcela, lancamentosDaParcela(c, numero), m.data)
	if err != nil {
		return domain.Pagamento{}, domain.NovoEvento{}, err
	}
	total := e.totalParaQuitar()
	if m.valor.GreaterThan(total) {
		return domain.Pagamento{}, domain.NovoEvento{}, fmt.Errorf("valor maior que o total para quitar a parcela %d em %s, que é %s", numero, m.dia(), total.StringFixed(2))
	}

	// RN-14: multa → mora → juros → amortização.
	restante := m.valor
	abater := func(emAberto decimal.Decimal) decimal.Decimal {
		parte := decimal.Min(restante, emAberto)
		restante = restante.Sub(parte)
		return parte
	}
	var d domain.Distribuicao
	d.Multa = abater(e.multaEmAberto)
	d.JurosMora = abater(e.moraEmAberto)
	d.Juros = abater(e.jurosEmAberto)
	d.Amortizacao = restante

	status := domain.StatusParcelaParcialmentePaga
	if m.valor.Equal(total) {
		status = domain.StatusParcelaPaga
	}
	saldo := op.SaldoDevedor.Sub(d.Amortizacao)

	// RN-19: quitar a última parcela em aberto liquida a operação. se não, o status sai calculado na data (RN-11).
	statusOperacao, liquidar := domain.StatusOperacaoAtiva, status == domain.StatusParcelaPaga
	for _, p := range c.Parcelas {
		if p.Numero == numero {
			p.Status = status
		}
		if p.Status != domain.StatusParcelaPaga {
			liquidar = false
			if p.Vencimento < m.dia() {
				statusOperacao = domain.StatusOperacaoEmAtraso
			}
		}
	}
	if liquidar {
		statusOperacao = domain.StatusOperacaoLiquidada
	}

	pagamento := domain.Pagamento{
		DataPagamento:          m.dia(),
		Valor:                  m.valor,
		IdentificadorPagamento: m.identificador,
		Origem:                 m.origem,
		Distribuicao:           d,
		Parcela: domain.ParcelaAposPagar{
			Numero:           numero,
			Status:           status,
			ValorEmAberto:    e.valorEmAberto.Sub(d.Juros).Sub(d.Amortizacao),
			EncargosEmAberto: e.multaEmAberto.Add(e.moraEmAberto).Sub(d.Multa).Sub(d.JurosMora),
		},
		Operacao: domain.OperacaoAposMover{NumeroOperacao: op.NumeroOperacao, Status: statusOperacao, SaldoDevedor: saldo},
	}
	evento := domain.NovoEvento{
		Tipo:                 domain.EventoPagamento,
		Data:                 m.dia(),
		Valor:                m.valor,
		IdentificadorExterno: m.identificador,
		Origem:               m.origem,
		Detalhe:              pagamento,
		Lancamentos:          []domain.NovoLancamento{{NumeroParcela: numero, Distribuicao: d}},
		StatusParcelas:       map[int]string{numero: status},
		SaldoDevedor:         saldo,
		Liquidar:             liquidar,
		EventoEncerramento:   liquidar,
	}
	return pagamento, evento, nil
}

func (s *OperacaoService) ConsultarLiquidacao(identificador string, referencia *string) (domain.ValorLiquidacao, error) {
	id, numero, err := identificarOperacao(identificador)
	if err != nil {
		return domain.ValorLiquidacao{}, err
	}
	data, err := interpretarDataReferencia(referencia, s.agora())
	if err != nil {
		return domain.ValorLiquidacao{}, err
	}
	c, err := s.movimentacoes.ConsultarCarteira(id, numero)
	if err != nil {
		return domain.ValorLiquidacao{}, err
	}
	if c.UltimaMovimentacao != "" && data.Format(time.DateOnly) < c.UltimaMovimentacao {
		return domain.ValorLiquidacao{}, errors.New("dataReferencia não pode ser anterior à data da última movimentação financeira da operação")
	}
	if c.Operacao.Status == domain.StatusOperacaoLiquidada {
		return domain.ValorLiquidacao{}, errors.New("a operação já está liquidada")
	}
	return calcularLiquidacao(c, data)
}

// RN-18: vencidas com encargos, sem desconto, + a vencer pelo valor presente (RN-16). sem tarifa.
func calcularLiquidacao(c domain.Carteira, data time.Time) (domain.ValorLiquidacao, error) {
	op := c.Operacao
	v := domain.ValorLiquidacao{
		NumeroOperacao: op.NumeroOperacao,
		DataReferencia: data.Format(time.DateOnly),
		SaldoDevedor:   op.SaldoDevedor,
		Vencidas:       []domain.ParcelaVencidaLiquidacao{},
		AVencer:        []domain.ParcelaAVencerLiquidacao{},
	}
	aVencer := []domain.Parcela{}
	emAberto := map[int]decimal.Decimal{}
	for _, p := range c.Parcelas {
		if p.Status == domain.StatusParcelaPaga {
			continue
		}
		e, err := calcularEncargos(p, lancamentosDaParcela(c, p.Numero), data)
		if err != nil {
			return domain.ValorLiquidacao{}, err
		}
		if p.Vencimento >= v.DataReferencia {
			aVencer = append(aVencer, p)
			emAberto[p.Numero] = e.valorEmAberto
			v.Amortizacao = v.Amortizacao.Add(e.valorEmAberto.Sub(e.jurosEmAberto))
			continue
		}
		d := domain.Distribuicao{Multa: e.multaEmAberto, JurosMora: e.moraEmAberto, Juros: e.jurosEmAberto, Amortizacao: e.valorEmAberto.Sub(e.jurosEmAberto)}
		v.Vencidas = append(v.Vencidas, domain.ParcelaVencidaLiquidacao{
			Numero: p.Numero, Vencimento: p.Vencimento, DiasEmAtraso: e.diasEmAtraso, Distribuicao: d, ValorEmAberto: e.valorEmAberto,
		})
		v.TotalVencidas = v.TotalVencidas.Add(d.Total())
	}

	presentes, err := calcularValoresPresentes(c.Parcelas, aVencer, emAberto, op.TaxaJuros, op.DataLiberacao, data)
	if err != nil {
		return domain.ValorLiquidacao{}, err
	}
	v.AVencer = presentes
	for _, p := range presentes {
		v.ValorNominal = v.ValorNominal.Add(p.ValorEmAberto)
		v.ValorPresente = v.ValorPresente.Add(p.ValorPresente)
	}
	v.Desconto = v.ValorNominal.Sub(v.ValorPresente)
	// os centavos do arredondamento por parcela não podem deixar juros negativos, que o banco recusa.
	v.Amortizacao = decimal.Min(v.Amortizacao, v.ValorPresente)
	v.Juros = v.ValorPresente.Sub(v.Amortizacao)
	v.ValorParaQuitar = v.TotalVencidas.Add(v.ValorPresente)
	return v, nil
}

func (s *OperacaoService) Liquidar(identificador string, pedido domain.PedidoMovimentacao) (domain.Liquidacao, bool, error) {
	id, numeroOperacao, err := identificarOperacao(identificador)
	if err != nil {
		return domain.Liquidacao{}, false, err
	}
	m, err := s.validarPedido(pedido)
	if err != nil {
		return domain.Liquidacao{}, false, err
	}

	var liquidacao domain.Liquidacao
	gravado, novo, err := s.movimentacoes.Movimentar(id, numeroOperacao, m.identificador, func(c domain.Carteira) (domain.NovoEvento, error) {
		var evento domain.NovoEvento
		liquidacao, evento, err = decidirLiquidacao(c, m)
		return evento, err
	})
	if err != nil {
		return domain.Liquidacao{}, false, err
	}
	if !novo {
		if gravado.Tipo != domain.EventoLiquidacao {
			return domain.Liquidacao{}, false, errIdentificadorJaUsado
		}
		if err := json.Unmarshal(gravado.Detalhe, &liquidacao); err != nil {
			return domain.Liquidacao{}, false, err
		}
	}
	liquidacao.ID, liquidacao.RegistradoEm = gravado.ID, gravado.RegistradoEm
	return liquidacao, novo, nil
}

func decidirLiquidacao(c domain.Carteira, m movimento) (domain.Liquidacao, domain.NovoEvento, error) {
	if c.Operacao.Status == domain.StatusOperacaoLiquidada {
		return domain.Liquidacao{}, domain.NovoEvento{}, errors.New("a operação já está liquidada")
	}
	if err := validarDataMovimento(m, c); err != nil {
		return domain.Liquidacao{}, domain.NovoEvento{}, err
	}
	v, err := calcularLiquidacao(c, m.data)
	if err != nil {
		return domain.Liquidacao{}, domain.NovoEvento{}, err
	}
	// o valor tem que ser o da data, pra ninguém quitar com um número velho.
	if !m.valor.Equal(v.ValorParaQuitar) {
		return domain.Liquidacao{}, domain.NovoEvento{}, fmt.Errorf("valor deve ser igual ao valor para quitar em %s, que é %s", m.dia(), v.ValorParaQuitar.StringFixed(2))
	}

	liquidacao := domain.Liquidacao{
		DataPagamento:          m.dia(),
		Valor:                  m.valor,
		IdentificadorPagamento: m.identificador,
		Origem:                 m.origem,
		Vencidas:               []domain.LiquidacaoVencida{},
		AVencer:                domain.LiquidacaoAVencer{Parcelas: []int{}, ValorNominal: v.ValorNominal, Desconto: v.Desconto, Amortizacao: v.Amortizacao, Juros: v.Juros, Valor: v.ValorPresente},
		Operacao:               domain.OperacaoAposMover{NumeroOperacao: c.Operacao.NumeroOperacao, Status: domain.StatusOperacaoLiquidada, SaldoDevedor: decimal.Zero},
	}
	evento := domain.NovoEvento{
		Tipo:                 domain.EventoLiquidacao,
		Data:                 m.dia(),
		Valor:                m.valor,
		IdentificadorExterno: m.identificador,
		Origem:               m.origem,
		StatusParcelas:       map[int]string{},
		SaldoDevedor:         decimal.Zero,
		Liquidar:             true,
	}
	// vencidas entram como pagamento de parcela; as a vencer, num lançamento só da operação (RN-18).
	for _, p := range v.Vencidas {
		liquidacao.Vencidas = append(liquidacao.Vencidas, domain.LiquidacaoVencida{Numero: p.Numero, Distribuicao: p.Distribuicao})
		evento.Lancamentos = append(evento.Lancamentos, domain.NovoLancamento{NumeroParcela: p.Numero, Distribuicao: p.Distribuicao})
		evento.StatusParcelas[p.Numero] = domain.StatusParcelaPaga
	}
	for _, p := range v.AVencer {
		liquidacao.AVencer.Parcelas = append(liquidacao.AVencer.Parcelas, p.Numero)
		evento.StatusParcelas[p.Numero] = domain.StatusParcelaPaga
	}
	if v.ValorPresente.IsPositive() {
		evento.Lancamentos = append(evento.Lancamentos, domain.NovoLancamento{
			Distribuicao: domain.Distribuicao{Juros: v.Juros, Amortizacao: v.Amortizacao}, Desconto: v.Desconto,
		})
	}
	evento.Detalhe = liquidacao
	return liquidacao, evento, nil
}
