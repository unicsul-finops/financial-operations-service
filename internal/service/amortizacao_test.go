package service

import (
	"strings"
	"testing"

	"github.com/unicsul-finops/financial-operations-service/internal/domain"
)

func carteiraComDuasPagas() domain.Carteira {
	c := carteiraExemplo()
	for _, p := range []struct {
		numero int
		data   string
	}{{1, "2026-11-30"}, {2, "2026-12-30"}} {
		_, e, err := decidirPagamento(c, p.numero, movimento{valor: dec("936.91"), data: dia(p.data)})
		if err != nil {
			panic(err)
		}
		gravar(&c, e)
	}
	return c
}

// RN-17: antecipar as 2 últimas em 30/12/2026 e em 15/01/2027, com as parcelas 1 e 2 pagas.
func TestAmortizacaoRN17(t *testing.T) {
	for _, caso := range []struct {
		data, p11, p12, vp, desconto, amortizacao, juros, saldo string
	}{
		{"2026-12-30", "794.42", "779.95", "1574.37", "299.41", "1574.37", "0", "6907.9"},
		{"2027-01-15", "801.51", "786.92", "1588.43", "285.35", "1574.38", "14.05", "6907.89"},
	} {
		t.Run(caso.data, func(t *testing.T) {
			r, err := calcularAmortizacao(carteiraComDuasPagas(), 2, domain.ModoUltimas, dia(caso.data))
			if err != nil {
				t.Fatal(err)
			}
			if len(r.Parcelas) != 2 || r.Parcelas[0].Numero != 11 || r.Parcelas[1].Numero != 12 || r.QuantidadeParcelasRestantes != 8 {
				t.Fatalf("parcelas: %+v", r.Parcelas)
			}
			iguais(t, "VP 11", r.Parcelas[0].ValorPresente, caso.p11)
			iguais(t, "VP 12", r.Parcelas[1].ValorPresente, caso.p12)
			iguais(t, "valor das parcelas", r.Totais.ValorParcelas, "1873.78")
			iguais(t, "valor a pagar", r.Totais.ValorAPagar, caso.vp)
			iguais(t, "desconto", r.Totais.Desconto, caso.desconto)
			iguais(t, "amortização", r.Totais.Amortizacao, caso.amortizacao)
			iguais(t, "juros", r.Totais.Juros, caso.juros)
			iguais(t, "saldo antes", r.SaldoDevedorAntes, "8482.27")
			iguais(t, "saldo depois", r.SaldoDevedorApos, caso.saldo)
			for _, p := range r.Parcelas {
				iguais(t, "VP = amortização + juros", p.Amortizacao.Add(p.Juros), p.ValorPresente.String())
				iguais(t, "desconto = valor − VP", p.ValorParcela.Sub(p.ValorPresente), p.Desconto.String())
			}
		})
	}
}

// RN-17: as parcelas 3 a 10 continuam com 936,91, a repartição é refeita e o saldo termina em zero na parcela 10.
func TestAmortizacaoRecalculaRestantes(t *testing.T) {
	for _, data := range []string{"2026-12-30", "2027-01-15"} {
		c := carteiraComDuasPagas()
		_, e, err := decidirAmortizacao(c, 2, domain.ModoUltimas, movimento{data: dia(data)})
		if err != nil {
			t.Fatal(err)
		}
		if len(e.ParcelasRecalculadas) != 8 || len(e.Lancamentos) != 2 {
			t.Fatalf("%d recalculadas, %d lançamentos", len(e.ParcelasRecalculadas), len(e.Lancamentos))
		}
		gravar(&c, e)
		saldo := c.Operacao.SaldoDevedor
		for i, p := range c.Parcelas[2:10] {
			iguais(t, "valor da parcela", p.Juros.Add(p.Amortizacao), "936.91")
			// a última parcela absorve os centavos: o juros fecha o valor da parcela.
			if i < 7 {
				iguais(t, "juros = saldo × i", p.Juros, saldo.Mul(dec("0.0185")).Round(2).String())
			}
			saldo = saldo.Sub(p.Amortizacao)
			iguais(t, "saldo da parcela", p.SaldoDevedor, saldo.String())
		}
		iguais(t, "saldo final", saldo, "0")
		for _, p := range c.Parcelas[10:] {
			if p.Status != domain.StatusParcelaPaga {
				t.Errorf("parcela %d: %s", p.Numero, p.Status)
			}
		}
		// o evento carrega o dinheiro e o desconto de cada parcela; o saldo cai só pela amortização.
		iguais(t, "valor do evento", e.Valor, e.Lancamentos[0].Total().Add(e.Lancamentos[1].Total()).String())
		iguais(t, "saldo devedor", c.Operacao.SaldoDevedor, e.SaldoDevedor.String())
	}
}

func TestAmortizacaoRecusada(t *testing.T) {
	parcial := carteiraComDuasPagas()
	parcial.Parcelas[4].Status = domain.StatusParcelaParcialmentePaga
	liquidada := carteiraComDuasPagas()
	liquidada.Operacao.Status = domain.StatusOperacaoLiquidada

	for _, caso := range []struct {
		nome       string
		c          domain.Carteira
		quantidade int
		data, erro string
	}{
		{"parcela vencida", carteiraExemplo(), 2, "2026-12-01", "a parcela 1 está vencida e em aberto, pague o atraso antes de antecipar parcelas"},
		{"parcela parcial", parcial, 2, "2027-01-15", "a parcela 5 está parcialmente paga, quite a parcela antes de antecipar parcelas"},
		{"todas as parcelas", carteiraComDuasPagas(), 10, "2027-01-15", "quantidadeParcelas cobre todas as 10 parcelas em aberto, para quitar tudo use a liquidação da operação"},
		{"mais que as em aberto", carteiraComDuasPagas(), 11, "2027-01-15", "quantidadeParcelas não pode ser maior que as 10 parcelas em aberto"},
		{"liquidada", liquidada, 2, "2027-01-15", "a operação já está liquidada, não há parcelas para antecipar"},
	} {
		t.Run(caso.nome, func(t *testing.T) {
			_, err := calcularAmortizacao(caso.c, caso.quantidade, domain.ModoUltimas, dia(caso.data))
			if err == nil || err.Error() != caso.erro {
				t.Fatalf("erro: %v", err)
			}
		})
	}

	c := carteiraComDuasPagas()
	for _, caso := range []struct{ data, erro string }{
		{"2026-10-29", "dataPagamento não pode ser anterior à dataLiberacao (2026-10-30)"},
		{"2026-12-29", "dataPagamento não pode ser anterior à data do último pagamento (2026-12-30)"},
	} {
		if _, _, err := decidirAmortizacao(c, 2, domain.ModoUltimas, movimento{data: dia(caso.data)}); err == nil || err.Error() != caso.erro {
			t.Errorf("%s: %v", caso.data, err)
		}
	}
}

// a parcela que vence no dia não tem desconto e a contagem k começa na seguinte (RN-16): a parcela 4 vale 936,91 ÷ 1,0185.
func TestAmortizacaoParcelaDoDia(t *testing.T) {
	r, err := calcularAmortizacao(carteiraComDuasPagas(), 9, domain.ModoUltimas, dia("2027-02-01"))
	if err != nil {
		t.Fatal(err)
	}
	if r.Parcelas[0].Numero != 4 || r.QuantidadeParcelasRestantes != 1 {
		t.Fatalf("primeira antecipada: %d, restantes %d", r.Parcelas[0].Numero, r.QuantidadeParcelasRestantes)
	}
	iguais(t, "valor presente", r.Parcelas[0].ValorPresente, "919.89")
	iguais(t, "desconto", r.Parcelas[0].Desconto, "17.02")
	iguais(t, "sem juros", r.Parcelas[0].Juros, "0")
	// a parcela do dia fica em aberto, com o saldo todo: juros do saldo e o resto de amortização.
	if len(r.Recalculadas) != 1 || r.Recalculadas[0].Numero != 3 {
		t.Fatalf("recalculadas: %+v", r.Recalculadas)
	}
	iguais(t, "saldo da parcela 3", r.Recalculadas[0].Depois.SaldoDevedor, "0")
}

func TestQuantidadeParcelas(t *testing.T) {
	for valor, esperado := range map[string]string{"": "quantidadeParcelas deve ser um inteiro maior que zero", "0": "quantidadeParcelas deve ser um inteiro maior que zero",
		"-1": "quantidadeParcelas deve ser um inteiro maior que zero", "1.5": "quantidadeParcelas deve ser um inteiro maior que zero", "abc": "quantidadeParcelas deve ser um inteiro maior que zero"} {
		v := valor
		if _, err := interpretarQuantidade(&v); err == nil || err.Error() != esperado {
			t.Errorf("%q: %v", valor, err)
		}
	}
	if _, err := interpretarQuantidade(nil); err == nil || !strings.Contains(err.Error(), "obrigatório") {
		t.Errorf("ausente: %v", err)
	}
	if n, err := interpretarQuantidade(ptr("3")); err != nil || n != 3 {
		t.Errorf("3: %d %v", n, err)
	}
}

func ptr(s string) *string { return &s }

// modo PROXIMAS: quita as primeiras em aberto, que têm bem menos desconto que as últimas.
func TestAmortizacaoProximas(t *testing.T) {
	for _, caso := range []struct {
		data, p3, p4, pagar, desconto, amortizacao, juros, saldo string
	}{
		{"2026-12-30", "919.89", "903.18", "1823.07", "50.75", "1823.07", "0", "6659.2"},
		{"2027-01-15", "928.1", "911.25", "1839.35", "34.47", "1823.08", "16.27", "6659.19"},
	} {
		t.Run(caso.data, func(t *testing.T) {
			r, err := calcularAmortizacao(carteiraComDuasPagas(), 2, domain.ModoProximas, dia(caso.data))
			if err != nil {
				t.Fatal(err)
			}
			if r.Modo != domain.ModoProximas || len(r.Parcelas) != 2 || r.Parcelas[0].Numero != 3 || r.Parcelas[1].Numero != 4 || r.QuantidadeParcelasRestantes != 8 {
				t.Fatalf("modo %s, parcelas %+v", r.Modo, r.Parcelas)
			}
			iguais(t, "VP 3", r.Parcelas[0].ValorPresente, caso.p3)
			iguais(t, "VP 4", r.Parcelas[1].ValorPresente, caso.p4)
			iguais(t, "valor a pagar", r.Totais.ValorAPagar, caso.pagar)
			iguais(t, "desconto", r.Totais.Desconto, caso.desconto)
			iguais(t, "amortização", r.Totais.Amortizacao, caso.amortizacao)
			iguais(t, "juros", r.Totais.Juros, caso.juros)
			iguais(t, "saldo depois", r.SaldoDevedorApos, caso.saldo)

			// as parcelas 5 a 12 continuam com o calendário original: o saldo depois de cada uma só difere em centavos,
			// mesmo com a primeira dois períodos adiante da data.
			if len(r.Recalculadas) != 8 || r.Recalculadas[0].Numero != 5 {
				t.Fatalf("recalculadas: %+v", r.Recalculadas)
			}
			for _, p := range r.Recalculadas {
				diferenca := p.Depois.SaldoDevedor.Sub(p.Antes.SaldoDevedor).Abs()
				if diferenca.GreaterThan(dec("0.05")) {
					t.Errorf("parcela %d: saldo %s, o original era %s", p.Numero, p.Depois.SaldoDevedor, p.Antes.SaldoDevedor)
				}
				iguais(t, "valor da parcela", p.Depois.Juros.Add(p.Depois.Amortizacao), p.Antes.Juros.Add(p.Antes.Amortizacao).String())
			}
			iguais(t, "saldo final", r.Recalculadas[7].Depois.SaldoDevedor, "0")
		})
	}
}

func TestAmortizacaoProximasGravaParcelasEStatus(t *testing.T) {
	c := carteiraComDuasPagas()
	_, e, err := decidirAmortizacao(c, 2, domain.ModoProximas, movimento{data: dia("2026-12-30")})
	if err != nil {
		t.Fatal(err)
	}
	gravar(&c, e)
	for _, p := range c.Parcelas {
		quitada := p.Numero <= 4
		if quitada != (p.Status == domain.StatusParcelaPaga) {
			t.Errorf("parcela %d: %s", p.Numero, p.Status)
		}
	}
	iguais(t, "saldo devedor", c.Operacao.SaldoDevedor, "6659.2")
	// 3 e 4 são lançadas com o desconto de cada uma; o resto do calendário não muda de vencimento.
	if len(e.Lancamentos) != 2 || e.Lancamentos[0].NumeroParcela != 3 || e.Lancamentos[1].NumeroParcela != 4 {
		t.Fatalf("lançamentos: %+v", e.Lancamentos)
	}
	iguais(t, "desconto da 4", e.Lancamentos[1].Desconto, "33.73")
}

// o mesmo pedido custa bem mais nas próximas do que nas últimas, e as recusas valem nos dois modos.
func TestAmortizacaoModosEMensagens(t *testing.T) {
	ultimas, _ := calcularAmortizacao(carteiraComDuasPagas(), 2, domain.ModoUltimas, dia("2026-12-30"))
	proximas, _ := calcularAmortizacao(carteiraComDuasPagas(), 2, domain.ModoProximas, dia("2026-12-30"))
	if !proximas.Totais.Desconto.LessThan(ultimas.Totais.Desconto) || !proximas.Totais.ValorAPagar.GreaterThan(ultimas.Totais.ValorAPagar) {
		t.Errorf("próximas: desconto %s, pagar %s; últimas: desconto %s, pagar %s", proximas.Totais.Desconto, proximas.Totais.ValorAPagar, ultimas.Totais.Desconto, ultimas.Totais.ValorAPagar)
	}

	if _, err := calcularAmortizacao(carteiraExemplo(), 2, domain.ModoProximas, dia("2026-12-01")); err == nil || err.Error() != "a parcela 1 está vencida e em aberto, pague o atraso antes de antecipar parcelas" {
		t.Errorf("parcela vencida: %v", err)
	}
	if _, err := calcularAmortizacao(carteiraComDuasPagas(), 10, domain.ModoProximas, dia("2026-12-30")); err == nil || err.Error() != "quantidadeParcelas cobre todas as 10 parcelas em aberto, para quitar tudo use a liquidação da operação" {
		t.Errorf("todas as parcelas: %v", err)
	}
}

func TestInterpretarModo(t *testing.T) {
	for valor, esperado := range map[string]string{"": "ULTIMAS", "  ": "ULTIMAS", "ultimas": "ULTIMAS", "ULTIMAS": "ULTIMAS", " proximas ": "PROXIMAS", "PROXIMAS": "PROXIMAS"} {
		v := valor
		if modo, err := interpretarModo(&v); err != nil || modo != esperado {
			t.Errorf("%q: %q %v", valor, modo, err)
		}
	}
	if modo, err := interpretarModo(nil); err != nil || modo != "ULTIMAS" {
		t.Errorf("ausente: %q %v", modo, err)
	}
	invalido := "TODAS"
	if _, err := interpretarModo(&invalido); err == nil || err.Error() != "modo deve ser ULTIMAS ou PROXIMAS" {
		t.Errorf("inválido: %v", err)
	}
}
