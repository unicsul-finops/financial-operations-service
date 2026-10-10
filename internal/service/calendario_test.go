package service

import (
	"strings"
	"testing"
	"time"

	"github.com/shopspring/decimal"
	"github.com/unicsul-finops/financial-operations-service/internal/domain"
)

func dec(valor string) decimal.Decimal {
	return decimal.RequireFromString(valor)
}

func data(valor string) time.Time {
	d, err := time.Parse("2006-01-02", valor)
	if err != nil {
		panic(err)
	}
	return d
}

// calendário PRICE do payload de exemplo do juros: 10.000,00 em 12x a 1,85% a.m.
// os vencimentos caem no dia útil seguinte, regra do serviço de juros.
func calendarioExemplo() []domain.NovaParcela {
	linhas := []struct {
		vencimento, valor, amortizacao, juros, saldo string
	}{
		{"2026-11-30", "936.91", "751.91", "185.00", "9248.09"},
		{"2026-12-30", "936.91", "765.82", "171.09", "8482.27"},
		{"2027-02-01", "936.91", "779.99", "156.92", "7702.28"},
		{"2027-03-01", "936.91", "794.42", "142.49", "6907.86"},
		{"2027-03-30", "936.91", "809.11", "127.80", "6098.75"},
		{"2027-04-30", "936.91", "824.08", "112.83", "5274.67"},
		{"2027-05-31", "936.91", "839.33", "97.58", "4435.34"},
		{"2027-06-30", "936.91", "854.86", "82.05", "3580.48"},
		{"2027-07-30", "936.91", "870.67", "66.24", "2709.81"},
		{"2027-08-30", "936.91", "886.78", "50.13", "1823.03"},
		{"2027-09-30", "936.91", "903.18", "33.73", "919.85"},
		{"2027-11-01", "936.87", "919.85", "17.02", "0"},
	}

	parcelas := make([]domain.NovaParcela, 0, len(linhas))
	for i, l := range linhas {
		parcelas = append(parcelas, domain.NovaParcela{
			Numero:       i + 1,
			Vencimento:   l.vencimento,
			ValorParcela: dec(l.valor),
			Amortizacao:  dec(l.amortizacao),
			Juros:        dec(l.juros),
			SaldoDevedor: dec(l.saldo),
		})
	}

	return parcelas
}

func TestValidarCalendarioExemplo(t *testing.T) {
	parcelas := calendarioExemplo()
	parcelas[0].Vencimento = " 2026-11-30 "

	if err := validarCalendario(parcelas, dec("10000"), 12, data("2026-10-30")); err != nil {
		t.Fatalf("calendário do juros deveria fechar: %v", err)
	}
	if parcelas[0].Vencimento != "2026-11-30" {
		t.Errorf("vencimento não foi normalizado: %q", parcelas[0].Vencimento)
	}
}

func TestValidarCalendarioErros(t *testing.T) {
	casos := []struct {
		nome     string
		alterar  func([]domain.NovaParcela) []domain.NovaParcela
		prazo    int
		mensagem string
	}{
		{"vazio", func(p []domain.NovaParcela) []domain.NovaParcela { return nil }, 12, "calendario não pode ser vazio"},
		{"parcelas a menos que o prazo", func(p []domain.NovaParcela) []domain.NovaParcela { return p[:11] }, 12, "calendario deve ter uma parcela por mês de condicoes.prazoMeses"},
		{"numero fora de ordem", func(p []domain.NovaParcela) []domain.NovaParcela {
			p[1].Numero = 3
			return p
		}, 12, "calendario: parcela na posição 2 deveria ter numero 2"},
		{"vencimento invalido", func(p []domain.NovaParcela) []domain.NovaParcela {
			p[2].Vencimento = "01/02/2027"
			return p
		}, 12, "calendario: parcela 3 com vencimento fora do formato YYYY-MM-DD"},
		{"vencimento antes da liberacao", func(p []domain.NovaParcela) []domain.NovaParcela {
			p[0].Vencimento = "2026-10-30"
			return p
		}, 12, "calendario: parcela 1 deve vencer depois da anterior e da dataLiberacao"},
		{"vencimento fora de ordem", func(p []domain.NovaParcela) []domain.NovaParcela {
			p[4].Vencimento = "2027-02-15"
			return p
		}, 12, "calendario: parcela 5 deve vencer depois da anterior e da dataLiberacao"},
		{"fracao de centavo", func(p []domain.NovaParcela) []domain.NovaParcela {
			p[0].Juros = dec("185.001")
			return p
		}, 12, "calendario: parcela 1 com valor negativo ou com mais de 2 casas decimais"},
		{"valor diferente da soma", func(p []domain.NovaParcela) []domain.NovaParcela {
			p[3].ValorParcela = dec("936.90")
			return p
		}, 12, "calendario: parcela 4 com valor diferente de amortizacao + juros"},
		{"saldo nao encadeia", func(p []domain.NovaParcela) []domain.NovaParcela {
			p[4].SaldoDevedor = dec("6098.70")
			return p
		}, 12, "calendario: parcela 5 com saldoDevedor 6098.7, esperado 6098.75"},
	}

	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			parcelas := c.alterar(calendarioExemplo())

			err := validarCalendario(parcelas, dec("10000"), c.prazo, data("2026-10-30"))
			if err == nil {
				t.Fatalf("esperava erro %q, não veio erro", c.mensagem)
			}
			if err.Error() != c.mensagem {
				t.Errorf("mensagem %q, esperava %q", err.Error(), c.mensagem)
			}
		})
	}
}

// se as amortizações não somam o valor, o saldo encadeado não chega a zero na última.
func TestValidarCalendarioSaldoFinal(t *testing.T) {
	parcelas := calendarioExemplo()

	err := validarCalendario(parcelas, dec("10000.01"), 12, data("2026-10-30"))
	if err == nil || !strings.HasPrefix(err.Error(), "calendario: parcela 1 com saldoDevedor") {
		t.Fatalf("esperava erro de saldo na primeira parcela, veio %v", err)
	}

	// todas as parcelas coerentes, mas a última deixa saldo.
	parcelas = calendarioExemplo()
	parcelas[11].Amortizacao = dec("919.84")
	parcelas[11].ValorParcela = dec("936.86")
	parcelas[11].SaldoDevedor = dec("0.01")

	err = validarCalendario(parcelas, dec("10000"), 12, data("2026-10-30"))
	if err == nil || err.Error() != "calendario: soma das amortizações 9999.99 diferente de condicoes.valor 10000" {
		t.Fatalf("esperava erro de saldo final, veio %v", err)
	}
}
