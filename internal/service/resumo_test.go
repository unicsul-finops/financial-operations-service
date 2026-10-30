package service

import (
	"testing"

	"github.com/unicsul-finops/financial-operations-service/internal/domain"
)

func conferirResumo(t *testing.T, r domain.Resumo) {
	t.Helper()
	// a conferência que vale sempre (RN-20).
	iguais(t, "totalPago = amortizado + juros + encargos", r.TotalPago, r.ValorAmortizado.Add(r.JurosPagos).Add(r.EncargosPagos).String())
}

// RN-14: depois do pagamento parcial de 500,00 em 10/12/2026.
func TestResumoAposPagamentoParcial(t *testing.T) {
	c := carteiraExemplo()
	c.Operacao.ValorAprovado = dec("10000")
	pagar(t, &c, 1, "500", "2026-12-10")

	r, err := calcularResumo(c, dia("2026-12-10"))
	if err != nil {
		t.Fatal(err)
	}
	conferirResumo(t, r)
	iguais(t, "totalContratado", r.TotalContratado, "10000")
	iguais(t, "totalPrevisto", r.TotalPrevisto, "11242.88")
	iguais(t, "totalPago", r.TotalPago, "500")
	iguais(t, "encargosPagos", r.EncargosPagos, "21.86")
	iguais(t, "jurosPagos", r.JurosPagos, "185")
	iguais(t, "valorAmortizado", r.ValorAmortizado, "293.14")
	iguais(t, "descontoAntecipacao", r.DescontoAntecipacao, "0")
	iguais(t, "saldoDevedor", r.SaldoDevedor, "9706.86")
	iguais(t, "valorVencido", r.ValorVencido, "458.77")
	iguais(t, "encargosEmAberto", r.EncargosEmAberto, "0")
	if r.Status != domain.StatusOperacaoEmAtraso || r.DiasEmAtraso != 10 {
		t.Errorf("status %s, %d dias", r.Status, r.DiasEmAtraso)
	}

	// o valor para quitar é o da liquidação na mesma data.
	v, err := calcularLiquidacao(c, dia("2026-12-10"))
	if err != nil {
		t.Fatal(err)
	}
	iguais(t, "valorParaQuitar", r.ValorParaQuitar, v.ValorParaQuitar.String())

	// dez dias depois a mora volta a correr sobre o que está vencido.
	r, err = calcularResumo(c, dia("2026-12-20"))
	if err != nil {
		t.Fatal(err)
	}
	iguais(t, "valorVencido em 20/12", r.ValorVencido, "458.77")
	iguais(t, "encargosEmAberto em 20/12", r.EncargosEmAberto, "1.53")
	if r.DiasEmAtraso != 20 {
		t.Errorf("%d dias", r.DiasEmAtraso)
	}
}

func TestResumoSemMovimentacao(t *testing.T) {
	c := carteiraExemplo()
	c.Operacao.ValorAprovado = dec("10000")
	for _, caso := range []struct{ data, status, vencido, encargos string }{
		{"2026-11-30", "ATIVA", "0", "0"},
		{"2026-12-10", "EM_ATRASO", "936.91", "21.86"},
		{"2027-03-01", "INADIMPLENTE", "3747.64", "0"},
	} {
		r, err := calcularResumo(c, dia(caso.data))
		if err != nil {
			t.Fatal(err)
		}
		conferirResumo(t, r)
		if r.Status != caso.status {
			t.Errorf("%s: status %s", caso.data, r.Status)
		}
		iguais(t, caso.data+" totalPago", r.TotalPago, "0")
		iguais(t, caso.data+" saldo", r.SaldoDevedor, "10000")
		if caso.data != "2027-03-01" {
			iguais(t, caso.data+" valorVencido", r.ValorVencido, caso.vencido)
			iguais(t, caso.data+" encargosEmAberto", r.EncargosEmAberto, caso.encargos)
		}
	}
}

// RN-18: depois da liquidação o resumo fecha com o que foi pago, o desconto e o saldo zerado.
func TestResumoAposLiquidacao(t *testing.T) {
	c := carteiraComDuasPagas()
	c.Operacao.ValorAprovado = dec("10000")
	_, e, err := decidirLiquidacao(c, movimento{valor: dec("8558"), data: dia("2027-01-15")})
	if err != nil {
		t.Fatal(err)
	}
	gravar(&c, e)

	r, err := calcularResumo(c, dia("2027-01-15"))
	if err != nil {
		t.Fatal(err)
	}
	conferirResumo(t, r)
	iguais(t, "totalPago", r.TotalPago, "10431.82")
	iguais(t, "valorAmortizado", r.ValorAmortizado, "10000")
	iguais(t, "jurosPagos", r.JurosPagos, "431.82")
	iguais(t, "descontoAntecipacao", r.DescontoAntecipacao, "811.06")
	iguais(t, "saldoDevedor", r.SaldoDevedor, "0")
	iguais(t, "valorParaQuitar", r.ValorParaQuitar, "0")
	iguais(t, "valorVencido", r.ValorVencido, "0")
	if r.Status != domain.StatusOperacaoLiquidada || r.DiasEmAtraso != 0 {
		t.Errorf("status %s, %d dias", r.Status, r.DiasEmAtraso)
	}
}
