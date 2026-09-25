package service

import (
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

// confere o que todo cronograma precisa garantir, independente do sistema.
func conferirFechamento(t *testing.T, parcelas []domain.NovaParcela, valor decimal.Decimal, quantidade int) {
	t.Helper()

	if len(parcelas) != quantidade {
		t.Fatalf("esperava %d parcelas, veio %d", quantidade, len(parcelas))
	}

	somaAmortizacao := decimal.Zero
	for _, parcela := range parcelas {
		somaAmortizacao = somaAmortizacao.Add(parcela.Amortizacao)

		if !parcela.ValorParcela.Equal(parcela.Amortizacao.Add(parcela.Juros)) {
			t.Errorf("parcela %d: valorParcela %s != amortizacao %s + juros %s",
				parcela.Numero, parcela.ValorParcela, parcela.Amortizacao, parcela.Juros)
		}
		if parcela.SaldoDevedor.IsNegative() {
			t.Errorf("parcela %d: saldo negativo %s", parcela.Numero, parcela.SaldoDevedor)
		}
	}

	if !somaAmortizacao.Equal(valor) {
		t.Errorf("soma da amortização %s != valor %s", somaAmortizacao, valor)
	}
	if ultimo := parcelas[len(parcelas)-1].SaldoDevedor; !ultimo.IsZero() {
		t.Errorf("saldo final deveria ser zero, veio %s", ultimo)
	}
}

func TestGerarCronogramaPricePayloadExemplo(t *testing.T) {
	parcelas, err := GerarCronograma(dec("15000.00"), dec("0.0199"), 24, data("2026-10-25"), domain.SistemaPrice)
	if err != nil {
		t.Fatal(err)
	}

	conferirFechamento(t, parcelas, dec("15000.00"), 24)

	// gabarito ao centavo, sem tolerância.
	esperado := []struct {
		vencimento, parcela, amortizacao, juros, saldo string
	}{
		{"2026-10-25", "792.17", "493.67", "298.50", "14506.33"},
		{"2026-11-25", "792.17", "503.49", "288.68", "14002.84"},
		{"2026-12-25", "792.17", "513.51", "278.66", "13489.33"},
	}

	for i, e := range esperado {
		p := parcelas[i]
		if p.Numero != i+1 || p.Vencimento != e.vencimento ||
			!p.ValorParcela.Equal(dec(e.parcela)) || !p.Amortizacao.Equal(dec(e.amortizacao)) ||
			!p.Juros.Equal(dec(e.juros)) || !p.SaldoDevedor.Equal(dec(e.saldo)) {
			t.Errorf("parcela %d: veio %s %s/%s/%s/%s, esperava %s %s/%s/%s/%s", i+1,
				p.Vencimento, p.ValorParcela, p.Amortizacao, p.Juros, p.SaldoDevedor,
				e.vencimento, e.parcela, e.amortizacao, e.juros, e.saldo)
		}
	}

	// só a última pode variar uns centavos, todas as outras têm o PMT exato.
	for _, p := range parcelas[:23] {
		if !p.ValorParcela.Equal(dec("792.17")) {
			t.Errorf("parcela %d deveria ser 792.17, veio %s", p.Numero, p.ValorParcela)
		}
	}
	if diferenca := parcelas[23].ValorParcela.Sub(dec("792.17")).Abs(); diferenca.GreaterThan(dec("0.10")) {
		t.Errorf("última parcela %s ficou longe demais do PMT", parcelas[23].ValorParcela)
	}
	if parcelas[23].Vencimento != "2028-09-25" {
		t.Errorf("último vencimento deveria ser 2028-09-25, veio %s", parcelas[23].Vencimento)
	}
}

func TestGerarCronogramaSAC(t *testing.T) {
	parcelas, err := GerarCronograma(dec("15000.00"), dec("0.0199"), 24, data("2026-10-25"), domain.SistemaSAC)
	if err != nil {
		t.Fatal(err)
	}

	conferirFechamento(t, parcelas, dec("15000.00"), 24)

	// 15000 / 24 = 625 exato, então aqui nem a última precisa de ajuste.
	for _, p := range parcelas {
		if !p.Amortizacao.Equal(dec("625.00")) {
			t.Errorf("parcela %d: amortização deveria ser 625.00, veio %s", p.Numero, p.Amortizacao)
		}
	}

	if !parcelas[0].Juros.Equal(dec("298.50")) || !parcelas[0].ValorParcela.Equal(dec("923.50")) {
		t.Errorf("primeira parcela: juros %s valor %s, esperava 298.50 e 923.50", parcelas[0].Juros, parcelas[0].ValorParcela)
	}
	// na última o saldo é 625, juros = 625 * 0.0199 = 12.4375 -> 12.44
	if !parcelas[23].Juros.Equal(dec("12.44")) || !parcelas[23].ValorParcela.Equal(dec("637.44")) {
		t.Errorf("última parcela: juros %s valor %s, esperava 12.44 e 637.44", parcelas[23].Juros, parcelas[23].ValorParcela)
	}

	for i := 1; i < len(parcelas); i++ {
		if !parcelas[i].ValorParcela.LessThan(parcelas[i-1].ValorParcela) {
			t.Errorf("SAC deveria ter parcela decrescente: %d=%s, %d=%s",
				i, parcelas[i-1].ValorParcela, i+1, parcelas[i].ValorParcela)
		}
	}
}

func TestGerarCronogramaSACAmortizacaoNaoExata(t *testing.T) {
	// 1000 / 3 = 333.33..., a última absorve o centavo que sobra.
	parcelas, err := GerarCronograma(dec("1000.00"), dec("0.01"), 3, data("2026-10-25"), domain.SistemaSAC)
	if err != nil {
		t.Fatal(err)
	}

	conferirFechamento(t, parcelas, dec("1000.00"), 3)

	if !parcelas[2].Amortizacao.Equal(dec("333.34")) {
		t.Errorf("última amortização deveria ser 333.34, veio %s", parcelas[2].Amortizacao)
	}
}

func TestGerarCronogramaTaxaZero(t *testing.T) {
	for _, sistema := range []string{domain.SistemaPrice, domain.SistemaSAC} {
		parcelas, err := GerarCronograma(dec("1000.00"), decimal.Zero, 3, data("2026-10-25"), sistema)
		if err != nil {
			t.Fatalf("%s: %v", sistema, err)
		}

		conferirFechamento(t, parcelas, dec("1000.00"), 3)

		esperado := []string{"333.33", "333.33", "333.34"}
		for i, p := range parcelas {
			if !p.Juros.IsZero() || !p.ValorParcela.Equal(dec(esperado[i])) {
				t.Errorf("%s parcela %d: juros %s valor %s, esperava 0 e %s", sistema, p.Numero, p.Juros, p.ValorParcela, esperado[i])
			}
		}
	}
}

func TestGerarCronogramaFechaEmVariosCenarios(t *testing.T) {
	cenarios := []struct {
		valor, taxa string
		quantidade  int
	}{
		{"0.01", "0.0199", 1},
		{"100.00", "0.05", 7},
		{"999999.99", "0.012345", 420},
		{"12345.67", "0.000001", 60},
	}

	for _, sistema := range []string{domain.SistemaPrice, domain.SistemaSAC} {
		for _, c := range cenarios {
			parcelas, err := GerarCronograma(dec(c.valor), dec(c.taxa), c.quantidade, data("2026-10-25"), sistema)
			if err != nil {
				t.Fatalf("%s %+v: %v", sistema, c, err)
			}
			conferirFechamento(t, parcelas, dec(c.valor), c.quantidade)
		}
	}
}

func TestGerarCronogramaSistemaInvalido(t *testing.T) {
	if _, err := GerarCronograma(dec("1000"), dec("0.01"), 3, data("2026-10-25"), "SACRE"); err == nil {
		t.Error("esperava erro para sistema inválido")
	}
}

func TestAdicionarMeses(t *testing.T) {
	casos := []struct {
		base     string
		meses    int
		esperado string
	}{
		{"2026-01-31", 0, "2026-01-31"},
		{"2026-01-31", 1, "2026-02-28"},
		{"2026-01-31", 2, "2026-03-31"},
		{"2026-01-31", 3, "2026-04-30"},
		{"2027-01-31", 13, "2028-02-29"},
		{"2026-10-25", 3, "2027-01-25"},
		{"2026-12-15", 1, "2027-01-15"},
	}

	for _, c := range casos {
		if veio := adicionarMeses(data(c.base), c.meses).Format("2006-01-02"); veio != c.esperado {
			t.Errorf("adicionarMeses(%s, %d) = %s, esperava %s", c.base, c.meses, veio, c.esperado)
		}
	}
}
