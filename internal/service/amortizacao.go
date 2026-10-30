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

// amortização antecipada parcial (RN-16 e RN-17): quita as últimas N parcelas em aberto pelo valor presente.

var errQuantidadeInvalida = errors.New("quantidadeParcelas deve ser um inteiro maior que zero")

// quantidadeParcelas da query: ausente e inválida têm mensagens diferentes.
func interpretarQuantidade(valor *string) (int, error) {
	if valor == nil {
		return 0, errors.New("quantidadeParcelas é obrigatório")
	}
	n, err := strconv.Atoi(strings.TrimSpace(*valor))
	if err != nil || n < 1 {
		return 0, errQuantidadeInvalida
	}
	return n, nil
}

var errModoInvalido = errors.New("modo deve ser ULTIMAS ou PROXIMAS")

// sem o campo vale ULTIMAS, o comportamento de antes do modo existir.
func interpretarModo(valor *string) (string, error) {
	if valor == nil || strings.TrimSpace(*valor) == "" {
		return domain.ModoUltimas, nil
	}
	modo := strings.ToUpper(strings.TrimSpace(*valor))
	if modo != domain.ModoUltimas && modo != domain.ModoProximas {
		return "", errModoInvalido
	}
	return modo, nil
}

func (s *OperacaoService) SimularAmortizacao(identificador string, quantidade, modo, referencia *string) (domain.SimulacaoAmortizacao, error) {
	id, numero, err := identificarOperacao(identificador)
	if err != nil {
		return domain.SimulacaoAmortizacao{}, err
	}
	n, err := interpretarQuantidade(quantidade)
	if err != nil {
		return domain.SimulacaoAmortizacao{}, err
	}
	m, err := interpretarModo(modo)
	if err != nil {
		return domain.SimulacaoAmortizacao{}, err
	}
	data, err := interpretarDataReferencia(referencia, s.agora())
	if err != nil {
		return domain.SimulacaoAmortizacao{}, err
	}
	c, err := s.movimentacoes.ConsultarCarteira(id, numero)
	if err != nil {
		return domain.SimulacaoAmortizacao{}, err
	}
	if c.UltimaMovimentacao != "" && data.Format(time.DateOnly) < c.UltimaMovimentacao {
		return domain.SimulacaoAmortizacao{}, fmt.Errorf("dataReferencia não pode ser anterior à data do último pagamento (%s)", c.UltimaMovimentacao)
	}
	return calcularAmortizacao(c, n, m, data)
}

// RN-17: sem parcela vencida em aberto nem paga em parte, e nunca todas as parcelas (isso é liquidação).
// ULTIMAS quita o fim do calendário e reduz o prazo. PROXIMAS quita as primeiras em aberto, que têm menos desconto.
func calcularAmortizacao(c domain.Carteira, quantidade int, modo string, data time.Time) (domain.SimulacaoAmortizacao, error) {
	op := c.Operacao
	if op.Status == domain.StatusOperacaoLiquidada {
		return domain.SimulacaoAmortizacao{}, errors.New("a operação já está liquidada, não há parcelas para antecipar")
	}
	dia := data.Format(time.DateOnly)
	emAberto := []domain.Parcela{}
	valores := map[int]decimal.Decimal{}
	for _, p := range c.Parcelas {
		if p.Status == domain.StatusParcelaPaga {
			continue
		}
		if p.Vencimento < dia {
			return domain.SimulacaoAmortizacao{}, fmt.Errorf("a parcela %d está vencida e em aberto, pague o atraso antes de antecipar parcelas", p.Numero)
		}
		if p.Status == domain.StatusParcelaParcialmentePaga {
			return domain.SimulacaoAmortizacao{}, fmt.Errorf("a parcela %d está parcialmente paga, quite a parcela antes de antecipar parcelas", p.Numero)
		}
		emAberto = append(emAberto, p)
		valores[p.Numero] = p.Juros.Add(p.Amortizacao)
	}
	if quantidade > len(emAberto) {
		return domain.SimulacaoAmortizacao{}, fmt.Errorf("quantidadeParcelas não pode ser maior que as %d parcelas em aberto", len(emAberto))
	}
	if quantidade == len(emAberto) {
		return domain.SimulacaoAmortizacao{}, fmt.Errorf("quantidadeParcelas cobre todas as %d parcelas em aberto, para quitar tudo use a liquidação da operação", len(emAberto))
	}

	// k conta entre todas as parcelas em aberto; só depois fica com as últimas.
	presentes, err := calcularValoresPresentes(c.Parcelas, emAberto, valores, op.TaxaJuros, op.DataLiberacao, data)
	if err != nil {
		return domain.SimulacaoAmortizacao{}, err
	}
	fracao, err := fracaoDoPeriodo(c.Parcelas, op.DataLiberacao, data)
	if err != nil {
		return domain.SimulacaoAmortizacao{}, err
	}
	fator, err := potenciaDaTaxa(op.TaxaJuros, fracao)
	if err != nil {
		return domain.SimulacaoAmortizacao{}, err
	}

	restantes := len(emAberto) - quantidade
	r := domain.SimulacaoAmortizacao{
		NumeroOperacao:              op.NumeroOperacao,
		Modo:                        modo,
		DataReferencia:              dia,
		QuantidadeParcelas:          quantidade,
		Parcelas:                    []domain.ParcelaAmortizada{},
		SaldoDevedorAntes:           op.SaldoDevedor,
		QuantidadeParcelasRestantes: restantes,
	}
	// quais parcelas saem e quais continuam em aberto.
	quitadas, quitadasEmAberto, continuam := presentes[restantes:], emAberto[restantes:], emAberto[:restantes]
	if modo == domain.ModoProximas {
		quitadas, quitadasEmAberto, continuam = presentes[:quantidade], emAberto[:quantidade], emAberto[quantidade:]
	}

	// a parte de principal do valor presente é VP ÷ (1 + i)^f. o resto são os juros do período em curso.
	for _, p := range quitadas {
		amortizacao := p.ValorPresente.Div(fator).Round(2)
		juros := p.ValorPresente.Sub(amortizacao)
		r.Parcelas = append(r.Parcelas, domain.ParcelaAmortizada{
			Numero: p.Numero, Vencimento: p.Vencimento, ValorParcela: p.ValorEmAberto, ValorPresente: p.ValorPresente,
			Desconto: p.Desconto, Amortizacao: amortizacao, Juros: juros,
		})
		r.Totais.ValorParcelas = r.Totais.ValorParcelas.Add(p.ValorEmAberto)
		r.Totais.ValorPresente = r.Totais.ValorPresente.Add(p.ValorPresente)
		r.Totais.Desconto = r.Totais.Desconto.Add(p.Desconto)
		r.Totais.Amortizacao = r.Totais.Amortizacao.Add(amortizacao)
		r.Totais.Juros = r.Totais.Juros.Add(juros)
	}
	r.Totais.ValorAPagar = r.Totais.ValorPresente
	r.SaldoDevedorApos = op.SaldoDevedor.Sub(r.Totais.Amortizacao)

	// a primeira parcela que continua só vence depois dos períodos das que foram quitadas, e o juros deles corre sobre o saldo.
	periodos := 1
	for _, p := range quitadasEmAberto {
		if p.Vencimento > dia {
			periodos++
		}
	}
	if modo == domain.ModoUltimas {
		periodos = 1
	}
	r.Recalculadas, err = recalcularRestantes(continuam, op.TaxaJuros, r.SaldoDevedorApos, periodos)
	if err != nil {
		return domain.SimulacaoAmortizacao{}, err
	}
	return r, nil
}

// RN-17: valor e vencimento ficam; a divisão entre juros e amortização é refeita pelo saldo menor (juros = saldo × i).
// a primeira parcela pode estar vários períodos à frente, quando as próximas é que foram quitadas: nesse caso o juros dela
// é o de todos os períodos, saldo × ((1 + i)^períodos − 1).
// a última parcela absorve os centavos: amortiza o que sobrou do saldo e o juros fecha o valor da parcela.
func recalcularRestantes(parcelas []domain.Parcela, taxa, saldo decimal.Decimal, periodos int) ([]domain.ParcelaRecalculada, error) {
	recalculadas := make([]domain.ParcelaRecalculada, 0, len(parcelas))
	for i, p := range parcelas {
		valor := p.Juros.Add(p.Amortizacao)
		juros := saldo.Mul(taxa).Round(2)
		if i == 0 && periodos > 1 {
			fator, err := potenciaDaTaxa(taxa, decimal.NewFromInt(int64(periodos)))
			if err != nil {
				return nil, err
			}
			juros = saldo.Mul(fator.Sub(decimal.NewFromInt(1))).Round(2)
		}
		amortizacao := valor.Sub(juros)
		if i == len(parcelas)-1 {
			amortizacao = saldo
			juros = valor.Sub(amortizacao)
		}
		saldo = saldo.Sub(amortizacao)
		recalculadas = append(recalculadas, domain.ParcelaRecalculada{
			Numero: p.Numero,
			Antes:  domain.ReparticaoParcela{Juros: p.Juros, Amortizacao: p.Amortizacao, SaldoDevedor: p.SaldoDevedor},
			Depois: domain.ReparticaoParcela{Juros: juros, Amortizacao: amortizacao, SaldoDevedor: saldo},
		})
	}
	return recalculadas, nil
}

func (s *OperacaoService) RegistrarAmortizacao(identificador string, pedido domain.PedidoAmortizacao) (domain.Amortizacao, bool, error) {
	id, numeroOperacao, err := identificarOperacao(identificador)
	if err != nil {
		return domain.Amortizacao{}, false, err
	}
	if !pedido.QuantidadeParcelas.Valid {
		return domain.Amortizacao{}, false, errors.New("quantidadeParcelas é obrigatório")
	}
	quantidade := pedido.QuantidadeParcelas.Decimal
	if !quantidade.IsInteger() || !quantidade.IsPositive() || quantidade.GreaterThan(decimal.NewFromInt(1_000_000)) {
		return domain.Amortizacao{}, false, errQuantidadeInvalida
	}
	modo, err := interpretarModo(pedido.Modo)
	if err != nil {
		return domain.Amortizacao{}, false, err
	}
	m, err := s.validarPedidoSemValor(pedido.DataPagamento, pedido.IdentificadorPagamento)
	if err != nil {
		return domain.Amortizacao{}, false, err
	}

	var amortizacao domain.Amortizacao
	gravado, novo, err := s.movimentacoes.Movimentar(id, numeroOperacao, m.identificador, func(c domain.Carteira) (domain.NovoEvento, error) {
		var evento domain.NovoEvento
		amortizacao, evento, err = decidirAmortizacao(c, int(quantidade.IntPart()), modo, m)
		return evento, err
	})
	if err != nil {
		return domain.Amortizacao{}, false, err
	}
	if !novo {
		if gravado.Tipo != domain.EventoAmortizacao {
			return domain.Amortizacao{}, false, errIdentificadorJaUsado
		}
		if err := json.Unmarshal(gravado.Detalhe, &amortizacao); err != nil {
			return domain.Amortizacao{}, false, err
		}
		// eventos gravados antes do modo existir eram sempre das últimas parcelas.
		if amortizacao.Modo == "" {
			amortizacao.Modo = domain.ModoUltimas
		}
	}
	amortizacao.ID = gravado.ID
	amortizacao.ParcelasRecalculadas = nil
	return amortizacao, novo, nil
}

func decidirAmortizacao(c domain.Carteira, quantidade int, modo string, m movimento) (domain.Amortizacao, domain.NovoEvento, error) {
	if m.dia() < c.Operacao.DataLiberacao {
		return domain.Amortizacao{}, domain.NovoEvento{}, fmt.Errorf("dataPagamento não pode ser anterior à dataLiberacao (%s)", c.Operacao.DataLiberacao)
	}
	if c.UltimaMovimentacao != "" && m.dia() < c.UltimaMovimentacao {
		return domain.Amortizacao{}, domain.NovoEvento{}, fmt.Errorf("dataPagamento não pode ser anterior à data do último pagamento (%s)", c.UltimaMovimentacao)
	}
	r, err := calcularAmortizacao(c, quantidade, modo, m.data)
	if err != nil {
		return domain.Amortizacao{}, domain.NovoEvento{}, err
	}

	amortizacao := domain.Amortizacao{
		NumeroOperacao:              r.NumeroOperacao,
		Modo:                        r.Modo,
		IdentificadorPagamento:      m.identificador,
		DataPagamento:               m.dia(),
		QuantidadeParcelas:          quantidade,
		Lancamentos:                 r.Parcelas,
		Totais:                      r.Totais,
		SaldoDevedorAntes:           r.SaldoDevedorAntes,
		SaldoDevedorApos:            r.SaldoDevedorApos,
		QuantidadeParcelasRestantes: r.QuantidadeParcelasRestantes,
		ParcelasRecalculadas:        r.Recalculadas,
	}
	evento := domain.NovoEvento{
		Tipo:                 domain.EventoAmortizacao,
		Data:                 m.dia(),
		Valor:                r.Totais.ValorAPagar,
		IdentificadorExterno: m.identificador,
		Detalhe:              amortizacao,
		StatusParcelas:       map[int]string{},
		SaldoDevedor:         r.SaldoDevedorApos,
		ParcelasRecalculadas: r.Recalculadas,
	}
	// um lançamento por parcela quitada: amortização, juros pro rata e o desconto concedido.
	for _, p := range r.Parcelas {
		evento.Lancamentos = append(evento.Lancamentos, domain.NovoLancamento{
			NumeroParcela: p.Numero, Distribuicao: domain.Distribuicao{Juros: p.Juros, Amortizacao: p.Amortizacao}, Desconto: p.Desconto,
		})
		evento.StatusParcelas[p.Numero] = domain.StatusParcelaPaga
	}
	return amortizacao, evento, nil
}
