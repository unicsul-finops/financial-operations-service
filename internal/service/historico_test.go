package service

import (
	"testing"

	"github.com/shopspring/decimal"

	"github.com/unicsul-finops/financial-operations-service/internal/domain"
)

func tiposEDatas(itens []domain.ItemHistorico) []string {
	resumo := []string{}
	for _, i := range itens {
		resumo = append(resumo, i.Tipo+" "+i.Origem+" "+i.DataEfeito)
	}
	return resumo
}

func igualTexto(t *testing.T, obtido, esperado []string) {
	t.Helper()
	if len(obtido) != len(esperado) {
		t.Fatalf("itens: %v", obtido)
	}
	for i := range esperado {
		if obtido[i] != esperado[i] {
			t.Fatalf("item %d: %q, esperado %q (todos: %v)", i, obtido[i], esperado[i], obtido)
		}
	}
}

func historicoDe(c domain.Carteira, eventos ...domain.EventoHistorico) domain.Historico {
	registro := domain.EventoHistorico{ID: 1, Tipo: domain.EventoOperacaoRegistrada, DataEfeito: "2026-10-30", RegistradoEm: "2026-10-08T18:13:02Z"}
	return domain.Historico{Carteira: c, Eventos: append([]domain.EventoHistorico{registro}, eventos...)}
}

// RN-21: parcela 1 vencida em 01/12/2026 e operação inadimplente em 01/03/2027, sem pagamentos.
func TestHistoricoAtrasoEInadimplencia(t *testing.T) {
	c := carteiraExemplo()
	itens, err := montarHistorico(historicoDe(c), dia("2026-12-01"))
	if err != nil {
		t.Fatal(err)
	}
	igualTexto(t, tiposEDatas(itens), []string{"OPERACAO_REGISTRADA REGISTRADO 2026-10-30", "PARCELA_VENCIDA CALCULADO 2026-12-01"})
	if itens[1].Descricao != "parcela 1 vencida em 2026-12-01" || *itens[1].NumeroParcela != 1 {
		t.Errorf("descrição: %q", itens[1].Descricao)
	}

	// 90 dias de atraso ainda não é inadimplência; 91 é (RN-12).
	itens, _ = montarHistorico(historicoDe(c), dia("2027-02-28"))
	for _, i := range itens {
		if i.Tipo == domain.ItemOperacaoInadimplente {
			t.Fatalf("inadimplente com 90 dias: %v", tiposEDatas(itens))
		}
	}
	itens, _ = montarHistorico(historicoDe(c), dia("2027-03-01"))
	ultimo := itens[len(itens)-1]
	if ultimo.Tipo != domain.ItemOperacaoInadimplente || ultimo.DataEfeito != "2027-03-01" || ultimo.Descricao != "operação inadimplente em 2027-03-01" {
		t.Fatalf("inadimplência: %+v", ultimo)
	}
	if itens[0].Tipo != domain.EventoOperacaoRegistrada || ultimo.RegistradoEm != "" || ultimo.ID != 0 || ultimo.Valor != nil {
		t.Errorf("item calculado não pode ter dados de evento gravado: %+v", ultimo)
	}
}

// parcela paga com atraso aparece como vencida, mas só até o dia do pagamento; paga em dia não aparece.
func TestHistoricoComPagamentos(t *testing.T) {
	c := carteiraExemplo()
	pagar(t, &c, 1, "500", "2026-12-10")
	pagar(t, &c, 1, "460.30", "2026-12-20")
	pagar(t, &c, 2, "936.91", "2026-12-30")

	h := historicoDe(c,
		domain.EventoHistorico{ID: 2, Tipo: domain.EventoPagamento, DataEfeito: "2026-12-10", RegistradoEm: "2026-12-10T13:20:05Z", Valor: decNull("500"),
			IdentificadorPagamento: "app-1", Origem: "CLIENTE", Lancamentos: []domain.LancamentoEvento{{NumeroParcela: 1,
				Distribuicao: domain.Distribuicao{Multa: dec("18.74"), JurosMora: dec("3.12"), Juros: dec("185"), Amortizacao: dec("293.14")}}}},
		domain.EventoHistorico{ID: 3, Tipo: domain.EventoPagamento, DataEfeito: "2026-12-20", RegistradoEm: "2026-12-20T14:02:11Z", Valor: decNull("460.30")},
		domain.EventoHistorico{ID: 4, Tipo: domain.EventoPagamento, DataEfeito: "2026-12-30", RegistradoEm: "2026-12-30T10:00:00Z", Valor: decNull("936.91")},
	)
	itens, err := montarHistorico(h, dia("2026-12-31"))
	if err != nil {
		t.Fatal(err)
	}
	igualTexto(t, tiposEDatas(itens), []string{
		"OPERACAO_REGISTRADA REGISTRADO 2026-10-30",
		"PARCELA_VENCIDA CALCULADO 2026-12-01",
		"PAGAMENTO REGISTRADO 2026-12-10",
		"PAGAMENTO REGISTRADO 2026-12-20",
		"PAGAMENTO REGISTRADO 2026-12-30",
	})

	pagamento := itens[2]
	iguais(t, "valor", *pagamento.Valor, "500")
	if pagamento.ID != 2 || pagamento.IdentificadorPagamento != "app-1" || pagamento.OrigemPagamento != "CLIENTE" || pagamento.RegistradoEm == "" {
		t.Errorf("pagamento: %+v", pagamento)
	}
	if len(pagamento.Lancamentos) != 1 || *pagamento.Lancamentos[0].NumeroParcela != 1 {
		t.Fatalf("lançamentos: %+v", pagamento.Lancamentos)
	}
	iguais(t, "multa", pagamento.Lancamentos[0].Multa, "18.74")
	iguais(t, "amortização", pagamento.Lancamentos[0].Amortizacao, "293.14")
	if itens[0].Valor != nil || itens[0].Lancamentos == nil || len(itens[0].Lancamentos) != 0 {
		t.Errorf("registro da operação não movimenta dinheiro: %+v", itens[0])
	}

	// a parcela 2 (vence em 30/12) pagas em dia: sem item de atraso.
	for _, i := range itens {
		if i.Tipo == domain.ItemParcelaVencida && *i.NumeroParcela != 1 {
			t.Errorf("parcela %d não esteve vencida", *i.NumeroParcela)
		}
	}
}

// liquidação: o lançamento da operação (sem parcela) sai com numeroParcela nulo e as a vencer não ficam vencidas.
func TestHistoricoLiquidacao(t *testing.T) {
	c := carteiraComDuasPagas()
	_, e, err := decidirLiquidacao(c, movimento{valor: dec("8558"), data: dia("2027-01-15")})
	if err != nil {
		t.Fatal(err)
	}
	gravar(&c, e)
	liquidacao := domain.EventoHistorico{ID: 5, Tipo: domain.EventoLiquidacao, DataEfeito: "2027-01-15", RegistradoEm: "2027-01-15T12:00:00Z",
		Valor: decNull("8558"), Lancamentos: []domain.LancamentoEvento{{Distribuicao: domain.Distribuicao{Juros: dec("75.73"), Amortizacao: dec("8482.27")}, Desconto: dec("811.06")}}}
	itens, err := montarHistorico(historicoDe(c, liquidacao), dia("2027-01-15"))
	if err != nil {
		t.Fatal(err)
	}
	igualTexto(t, tiposEDatas(itens), []string{"OPERACAO_REGISTRADA REGISTRADO 2026-10-30", "LIQUIDACAO REGISTRADO 2027-01-15"})
	if itens[1].Lancamentos[0].NumeroParcela != nil {
		t.Errorf("lançamento da operação com parcela %d", *itens[1].Lancamentos[0].NumeroParcela)
	}
	iguais(t, "desconto", itens[1].Lancamentos[0].Desconto, "811.06")
}

func decNull(valor string) decimal.NullDecimal {
	return decimal.NullDecimal{Decimal: dec(valor), Valid: true}
}
