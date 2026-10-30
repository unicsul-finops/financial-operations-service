package service

import (
	"strings"
	"testing"
	"time"

	"github.com/shopspring/decimal"
	"github.com/unicsul-finops/financial-operations-service/internal/domain"
)

func dia(valor string) time.Time {
	d, err := time.ParseInLocation(time.DateOnly, valor, fusoJuros)
	if err != nil {
		panic(err)
	}
	return d
}

// operação PRICE do exemplo das RNs, sem nenhum pagamento.
func carteiraExemplo() domain.Carteira {
	c := domain.Carteira{Operacao: domain.Operacao{
		NumeroOperacao: "OP-2026-0001", Status: domain.StatusOperacaoAtiva, DataLiberacao: "2026-10-30",
		TaxaJuros: dec("0.0185"), SaldoDevedor: dec("10000"),
	}}
	for _, p := range calendarioExemplo() {
		c.Parcelas = append(c.Parcelas, domain.Parcela{Numero: p.Numero, Vencimento: p.Vencimento, ValorParcela: p.ValorParcela,
			Amortizacao: p.Amortizacao, Juros: p.Juros, SaldoDevedor: p.SaldoDevedor, Status: domain.StatusParcelaPendente})
	}
	return c
}

// aplica na carteira o que o repositório gravaria.
func gravar(c *domain.Carteira, e domain.NovoEvento) {
	for _, l := range e.Lancamentos {
		if l.NumeroParcela > 0 {
			c.Lancamentos = append(c.Lancamentos, domain.LancamentoParcela{NumeroParcela: l.NumeroParcela, Data: e.Data, Distribuicao: l.Distribuicao, Desconto: l.Desconto})
			continue
		}
		o := &c.LancamentoOperacao
		o.Multa, o.JurosMora, o.Juros, o.Amortizacao = o.Multa.Add(l.Multa), o.JurosMora.Add(l.JurosMora), o.Juros.Add(l.Juros), o.Amortizacao.Add(l.Amortizacao)
		o.Desconto = o.Desconto.Add(l.Desconto)
	}
	for i := range c.Parcelas {
		if status, ok := e.StatusParcelas[c.Parcelas[i].Numero]; ok {
			c.Parcelas[i].Status = status
		}
	}
	for _, r := range e.ParcelasRecalculadas {
		for i := range c.Parcelas {
			if c.Parcelas[i].Numero == r.Numero {
				c.Parcelas[i].Juros, c.Parcelas[i].Amortizacao, c.Parcelas[i].SaldoDevedor = r.Depois.Juros, r.Depois.Amortizacao, r.Depois.SaldoDevedor
			}
		}
	}
	c.Operacao.SaldoDevedor = e.SaldoDevedor
	if e.Liquidar {
		c.Operacao.Status = domain.StatusOperacaoLiquidada
	}
	c.UltimaMovimentacao = e.Data
}

func pagar(t *testing.T, c *domain.Carteira, numero int, valor, data string) domain.Pagamento {
	t.Helper()
	p, e, err := decidirPagamento(*c, numero, movimento{valor: dec(valor), data: dia(data)})
	if err != nil {
		t.Fatalf("pagamento de %s na parcela %d em %s: %v", valor, numero, data, err)
	}
	gravar(c, e)
	return p
}

func iguais(t *testing.T, nome string, obtido decimal.Decimal, esperado string) {
	t.Helper()
	if !obtido.Equal(dec(esperado)) {
		t.Errorf("%s: %s, esperado %s", nome, obtido, esperado)
	}
}

// RN-13 e RN-14: a parcela 1 paga em duas vezes, com 10 dias de atraso e depois mais 10.
func TestPagamentoParcialEQuitacaoRN14(t *testing.T) {
	c := carteiraExemplo()

	p := pagar(t, &c, 1, "500", "2026-12-10")
	iguais(t, "multa", p.Distribuicao.Multa, "18.74")
	iguais(t, "mora", p.Distribuicao.JurosMora, "3.12")
	iguais(t, "juros", p.Distribuicao.Juros, "185")
	iguais(t, "amortização", p.Distribuicao.Amortizacao, "293.14")
	iguais(t, "em aberto", p.Parcela.ValorEmAberto, "458.77")
	iguais(t, "encargos em aberto", p.Parcela.EncargosEmAberto, "0")
	iguais(t, "saldo", p.Operacao.SaldoDevedor, "9706.86")
	if p.Parcela.Status != domain.StatusParcelaParcialmentePaga || p.Operacao.Status != domain.StatusOperacaoEmAtraso {
		t.Errorf("status depois do parcial: %s / %s", p.Parcela.Status, p.Operacao.Status)
	}

	// a mora dos 10 dias seguintes corre só sobre os 458,77: 1,53. para quitar, 460,30.
	if _, _, err := decidirPagamento(c, 1, movimento{valor: dec("460.31"), data: dia("2026-12-20")}); err == nil ||
		err.Error() != "valor maior que o total para quitar a parcela 1 em 2026-12-20, que é 460.30" {
		t.Fatalf("aceitou mais que o devido: %v", err)
	}
	p = pagar(t, &c, 1, "460.30", "2026-12-20")
	iguais(t, "multa", p.Distribuicao.Multa, "0")
	iguais(t, "mora", p.Distribuicao.JurosMora, "1.53")
	iguais(t, "juros", p.Distribuicao.Juros, "0")
	iguais(t, "amortização", p.Distribuicao.Amortizacao, "458.77")
	iguais(t, "saldo", p.Operacao.SaldoDevedor, "9248.09")
	if p.Parcela.Status != domain.StatusParcelaPaga || p.Operacao.Status != domain.StatusOperacaoAtiva {
		t.Errorf("status depois da quitação: %s / %s", p.Parcela.Status, p.Operacao.Status)
	}
}

func TestPagamentoRecusado(t *testing.T) {
	c := carteiraExemplo()
	for _, caso := range []struct {
		numero      int
		valor, data string
		erro        string
	}{
		{1, "958.78", "2026-12-10", "valor maior que o total para quitar a parcela 1 em 2026-12-10, que é 958.77"},
		{3, "100", "2026-12-10", "a parcela 3 ainda não pode ser paga, só as vencidas e a próxima a vencer"},
		{1, "100", "2026-10-29", "dataPagamento não pode ser anterior à dataLiberacao da operação"},
		{13, "100", "2026-12-10", "parcela não encontrada"},
	} {
		if _, _, err := decidirPagamento(c, caso.numero, movimento{valor: dec(caso.valor), data: dia(caso.data)}); err == nil || err.Error() != caso.erro {
			t.Errorf("parcela %d, %s em %s: %v", caso.numero, caso.valor, caso.data, err)
		}
	}

	pagar(t, &c, 1, "958.77", "2026-12-10")
	for _, caso := range []struct {
		data, erro string
	}{
		{"2026-12-09", "dataPagamento não pode ser anterior à data da última movimentação financeira da operação"},
		{"2026-12-10", "parcela 1 já está paga"},
	} {
		if _, _, err := decidirPagamento(c, 1, movimento{valor: dec("1"), data: dia(caso.data)}); err == nil || err.Error() != caso.erro {
			t.Errorf("%s: %v", caso.data, err)
		}
	}

	c.Operacao.Status = domain.StatusOperacaoLiquidada
	if _, _, err := decidirPagamento(c, 2, movimento{valor: dec("1"), data: dia("2026-12-10")}); err == nil || err.Error() != "a operação está liquidada e não aceita mais pagamentos" {
		t.Errorf("liquidada: %v", err)
	}
}

// pagamento pequeno: a ordem da RN-14 aparece, a multa sai inteira e a mora só em parte, sem tocar nos juros.
func TestOrdemDeAbatimento(t *testing.T) {
	c := carteiraExemplo()
	p := pagar(t, &c, 1, "20", "2026-12-10")
	iguais(t, "multa", p.Distribuicao.Multa, "18.74")
	iguais(t, "mora", p.Distribuicao.JurosMora, "1.26")
	iguais(t, "juros", p.Distribuicao.Juros, "0")
	iguais(t, "amortização", p.Distribuicao.Amortizacao, "0")
	iguais(t, "encargos em aberto", p.Parcela.EncargosEmAberto, "1.86")
	iguais(t, "saldo", p.Operacao.SaldoDevedor, "10000")
}

// RN-19: pagar a última parcela em aberto liquida a operação e grava o evento de encerramento.
func TestUltimaParcelaLiquidaRN19(t *testing.T) {
	c := carteiraExemplo()
	for i, p := range c.Parcelas[:11] {
		pagar(t, &c, i+1, p.ValorParcela.String(), p.Vencimento)
	}
	p, e, err := decidirPagamento(c, 12, movimento{valor: dec("936.87"), data: dia("2027-11-01")})
	if err != nil {
		t.Fatal(err)
	}
	if !e.Liquidar || !e.EventoEncerramento || p.Operacao.Status != domain.StatusOperacaoLiquidada {
		t.Errorf("não liquidou: %+v", e)
	}
	iguais(t, "saldo", p.Operacao.SaldoDevedor, "0")
}

// RN-18: parcelas 1 e 2 pagas em dia.
func TestValorParaQuitarRN18(t *testing.T) {
	c := carteiraExemplo()
	pagar(t, &c, 1, "936.91", "2026-11-30")
	pagar(t, &c, 2, "936.91", "2026-12-30")

	for _, caso := range []struct{ data, quitar, desconto, juros string }{
		{"2026-12-30", "8482.27", "886.79", "0"},
		{"2027-01-15", "8558.00", "811.06", "75.73"},
	} {
		v, err := calcularLiquidacao(c, dia(caso.data))
		if err != nil {
			t.Fatal(err)
		}
		iguais(t, caso.data+" valor para quitar", v.ValorParaQuitar, caso.quitar)
		iguais(t, caso.data+" nominal", v.ValorNominal, "9369.06")
		iguais(t, caso.data+" desconto", v.Desconto, caso.desconto)
		iguais(t, caso.data+" amortização", v.Amortizacao, "8482.27")
		iguais(t, caso.data+" juros", v.Juros, caso.juros)
		if len(v.Vencidas) != 0 || len(v.AVencer) != 10 {
			t.Errorf("%s: %d vencidas, %d a vencer", caso.data, len(v.Vencidas), len(v.AVencer))
		}
	}
	// o valor presente das últimas, igual ao exemplo da RN-17.
	v, _ := calcularLiquidacao(c, dia("2026-12-30"))
	iguais(t, "VP parcela 11", v.AVencer[8].ValorPresente, "794.42")
	iguais(t, "VP parcela 12", v.AVencer[9].ValorPresente, "779.95")
}

func TestLiquidacao(t *testing.T) {
	c := carteiraExemplo()
	pagar(t, &c, 1, "936.91", "2026-11-30")
	pagar(t, &c, 2, "936.91", "2026-12-30")

	if _, _, err := decidirLiquidacao(c, movimento{valor: dec("8558.01"), data: dia("2027-01-15")}); err == nil ||
		err.Error() != "valor deve ser igual ao valor para quitar em 2027-01-15, que é 8558.00" {
		t.Fatalf("valor diferente: %v", err)
	}
	l, e, err := decidirLiquidacao(c, movimento{valor: dec("8558"), data: dia("2027-01-15")})
	if err != nil {
		t.Fatal(err)
	}
	if !e.Liquidar || len(e.StatusParcelas) != 10 || len(e.Lancamentos) != 1 || e.Lancamentos[0].NumeroParcela != 0 {
		t.Fatalf("evento: %+v", e)
	}
	iguais(t, "lançamento juros", e.Lancamentos[0].Juros, "75.73")
	iguais(t, "lançamento amortização", e.Lancamentos[0].Amortizacao, "8482.27")
	iguais(t, "lançamento desconto", e.Lancamentos[0].Desconto, "811.06")
	iguais(t, "saldo", e.SaldoDevedor, "0")
	if l.Operacao.Status != domain.StatusOperacaoLiquidada || len(l.AVencer.Parcelas) != 10 {
		t.Errorf("liquidação: %+v", l)
	}

	gravar(&c, e)
	if _, _, err := decidirLiquidacao(c, movimento{valor: dec("1"), data: dia("2027-01-15")}); err == nil || err.Error() != "a operação já está liquidada" {
		t.Errorf("liquidou de novo: %v", err)
	}
}

// vencida com encargos, sem desconto; a que vence no dia, pelo valor em aberto.
func TestLiquidacaoComVencidaEParcelaDoDia(t *testing.T) {
	c := carteiraExemplo()
	v, err := calcularLiquidacao(c, dia("2026-12-30"))
	if err != nil {
		t.Fatal(err)
	}
	if len(v.Vencidas) != 1 || v.Vencidas[0].DiasEmAtraso != 30 {
		t.Fatalf("vencidas: %+v", v.Vencidas)
	}
	// 936,91 + 18,74 de multa + 9,37 de mora (936,91 × 1% × 30/30).
	iguais(t, "total da vencida", v.TotalVencidas, "965.02")
	hoje := v.AVencer[0]
	if hoje.Numero != 2 || !hoje.ValorPresente.Equal(dec("936.91")) || !hoje.Desconto.IsZero() {
		t.Errorf("parcela que vence no dia: %+v", hoje)
	}
	// no dia de um vencimento, o VP de todas as a vencer é o saldo delas: 8482,27 das parcelas 3 a 12 + 936,91 da 2.
	iguais(t, "valor presente", v.ValorPresente, "9419.18")
	iguais(t, "juros", v.Juros, "171.09")
	iguais(t, "valor para quitar", v.ValorParaQuitar, "10384.20")
}

func TestValidarPedido(t *testing.T) {
	s := &OperacaoService{agora: func() time.Time { return dia("2026-12-10").Add(12 * time.Hour) }}
	texto := func(v string) *string { return &v }
	valor := func(v string) decimal.NullDecimal { return decimal.NewNullDecimal(dec(v)) }
	for _, caso := range []struct {
		pedido domain.PedidoMovimentacao
		erro   string
	}{
		{domain.PedidoMovimentacao{}, "valor é obrigatório"},
		{domain.PedidoMovimentacao{Valor: valor("0")}, "valor deve ser maior que zero"},
		{domain.PedidoMovimentacao{Valor: valor("1.001")}, "valor deve ter no máximo 2 casas decimais"},
		{domain.PedidoMovimentacao{Valor: valor("1"), DataPagamento: texto("10/12/2026")}, "dataPagamento deve estar no formato YYYY-MM-DD"},
		{domain.PedidoMovimentacao{Valor: valor("1"), DataPagamento: texto("2026-12-11")}, "dataPagamento não pode ser futura"},
		{domain.PedidoMovimentacao{Valor: valor("1"), IdentificadorPagamento: texto(" ")}, "identificadorPagamento não pode ser vazio"},
		{domain.PedidoMovimentacao{Valor: valor("1"), IdentificadorPagamento: texto(strings.Repeat("x", 51))}, "identificadorPagamento deve ter no máximo 50 caracteres"},
		{domain.PedidoMovimentacao{Valor: valor("1"), Origem: texto(strings.Repeat("x", 31))}, "origem deve ter no máximo 30 caracteres"},
	} {
		if _, err := s.validarPedido(caso.pedido); err == nil || err.Error() != caso.erro {
			t.Errorf("esperado %q: %v", caso.erro, err)
		}
	}
	m, err := s.validarPedido(domain.PedidoMovimentacao{Valor: valor("500")})
	if err != nil || m.dia() != "2026-12-10" {
		t.Errorf("data omitida deveria ser hoje em GMT-03: %v %v", m.dia(), err)
	}
}
