package service

import (
	"testing"

	"github.com/unicsul-finops/financial-operations-service/internal/domain"
)

// exemplos da RN-13 e da RN-14, parcela 1 de 936,91 vencida em 30/11/2026.
func TestEncargosNaConsulta(t *testing.T) {
	pago500 := domain.LancamentoParcela{NumeroParcela: 1, Data: "2026-12-10",
		Distribuicao: domain.Distribuicao{Multa: dec("18.74"), JurosMora: dec("3.12"), Juros: dec("185"), Amortizacao: dec("293.14")}}
	for _, c := range []struct {
		nome, data, status string
		lancamentos        []domain.LancamentoParcela
		multa, mora, total string
	}{
		{"no vencimento", "2026-11-30", "PENDENTE", nil, "0", "0", "936.91"},
		{"antes do vencimento", "2026-11-01", "PENDENTE", nil, "0", "0", "936.91"},
		{"primeiro dia", "2026-12-01", "PENDENTE", nil, "18.74", "0.31", "955.96"},
		{"dez dias", "2026-12-10", "PENDENTE", nil, "18.74", "3.12", "958.77"},
		{"após pagamento parcial", "2026-12-20", "PARCIALMENTE_PAGA", []domain.LancamentoParcela{pago500}, "0", "1.53", "460.3"},
		{"parcial no dia do pagamento", "2026-12-10", "PARCIALMENTE_PAGA", []domain.LancamentoParcela{pago500}, "0", "0", "458.77"},
	} {
		t.Run(c.nome, func(t *testing.T) {
			p := parcelaExemplo()
			p.Status = c.status
			p.Lancamentos = c.lancamentos
			for _, l := range c.lancamentos {
				p.JurosPagos, p.AmortizacaoPaga = p.JurosPagos.Add(l.Juros), p.AmortizacaoPaga.Add(l.Amortizacao)
			}
			got, err := calcularSituacaoParcela(p, dia(c.data))
			if err != nil {
				t.Fatal(err)
			}
			if !got.MultaEmAberto.Equal(dec(c.multa)) || !got.MoraEmAberto.Equal(dec(c.mora)) || !got.TotalParaQuitar.Equal(dec(c.total)) {
				t.Fatalf("multa %s, mora %s, total para quitar %s", got.MultaEmAberto, got.MoraEmAberto, got.TotalParaQuitar)
			}
		})
	}
}

// RN-13: parcela PAGA fica com encargos e total para quitar zerados, inclusive quando paga com desconto.
func TestEncargosParcelaPaga(t *testing.T) {
	p := parcelaExemplo()
	p.Status = domain.StatusParcelaPaga
	p.Lancamentos = []domain.LancamentoParcela{{NumeroParcela: 1, Data: "2026-12-10",
		Distribuicao: domain.Distribuicao{Amortizacao: dec("751.91")}, Desconto: dec("185")}}
	got, err := calcularSituacaoParcela(p, dia("2027-03-01"))
	if err != nil {
		t.Fatal(err)
	}
	if !got.MultaEmAberto.IsZero() || !got.MoraEmAberto.IsZero() || !got.TotalParaQuitar.IsZero() || !got.ValorEmAberto.IsZero() {
		t.Fatalf("PAGA com encargos: %s %s %s", got.MultaEmAberto, got.MoraEmAberto, got.TotalParaQuitar)
	}
}
