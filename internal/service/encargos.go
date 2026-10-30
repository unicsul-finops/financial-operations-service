package service

import (
	"time"

	"github.com/shopspring/decimal"
	"github.com/unicsul-finops/financial-operations-service/internal/domain"
)

// PROVISÓRIO: o cálculo de multa e mora é da #7. quando ela entrar, este arquivo dá lugar à função dela.

var (
	percentualMulta = decimal.RequireFromString("0.02")
	moraPorDia      = decimal.RequireFromString("0.01").Div(decimal.NewFromInt(30))
)

type encargosParcela struct {
	// juros + amortização que faltam, sem encargos.
	valorEmAberto     decimal.Decimal
	jurosEmAberto     decimal.Decimal
	multaEmAberto     decimal.Decimal
	moraEmAberto      decimal.Decimal
	diasEmAtraso      int
	jurosPagos        decimal.Decimal
	amortizacaoPaga   decimal.Decimal
	descontoConcedido decimal.Decimal
}

func (e encargosParcela) totalParaQuitar() decimal.Decimal {
	return e.valorEmAberto.Add(e.multaEmAberto).Add(e.moraEmAberto)
}

// RN-13: multa de 2% fixada no 1º dia de atraso e mora de 1% ao mês pro rata die sobre o valor em aberto de cada dia.
// a base da mora diminui a partir da data de cada pagamento, e o arredondamento é só no resultado.
func calcularEncargos(parcela domain.Parcela, lancamentos []domain.LancamentoParcela, data time.Time) (encargosParcela, error) {
	vencimento, err := time.ParseInLocation(time.DateOnly, parcela.Vencimento, fusoJuros)
	if err != nil {
		return encargosParcela{}, err
	}

	var e encargosParcela
	multaPaga, moraPaga := decimal.Zero, decimal.Zero
	for _, l := range lancamentos {
		e.jurosPagos = e.jurosPagos.Add(l.Juros)
		e.amortizacaoPaga = e.amortizacaoPaga.Add(l.Amortizacao)
		e.descontoConcedido = e.descontoConcedido.Add(l.Desconto)
		multaPaga = multaPaga.Add(l.Multa)
		moraPaga = moraPaga.Add(l.JurosMora)
	}
	e.jurosEmAberto = parcela.Juros.Sub(e.jurosPagos)
	e.valorEmAberto = parcela.Juros.Add(parcela.Amortizacao).Sub(e.jurosPagos).Sub(e.amortizacaoPaga).Sub(e.descontoConcedido)
	if parcela.Status == domain.StatusParcelaPaga || !data.After(vencimento) || !e.valorEmAberto.IsPositive() {
		e.valorEmAberto = decimal.Max(e.valorEmAberto, decimal.Zero)
		return e, nil
	}
	e.diasEmAtraso = diasEntre(vencimento, data)

	// valor em aberto no fim de um dia: o pagamento feito no dia já reduz a base dali em diante.
	emAbertoAte := func(dia time.Time) decimal.Decimal {
		aberto := parcela.Juros.Add(parcela.Amortizacao)
		for _, l := range lancamentos {
			if l.Data <= dia.Format(time.DateOnly) {
				aberto = aberto.Sub(l.Juros).Sub(l.Amortizacao).Sub(l.Desconto)
			}
		}
		return aberto
	}

	multa := emAbertoAte(vencimento).Mul(percentualMulta).Round(2)

	// trechos entre o vencimento, cada pagamento depois dele e a data.
	mora := decimal.Zero
	inicio := vencimento
	for _, l := range lancamentos {
		dia, err := time.ParseInLocation(time.DateOnly, l.Data, fusoJuros)
		if err != nil {
			return encargosParcela{}, err
		}
		if !dia.After(inicio) || !dia.Before(data) {
			continue
		}
		mora = mora.Add(emAbertoAte(inicio).Mul(moraPorDia).Mul(decimal.NewFromInt(int64(diasEntre(inicio, dia)))))
		inicio = dia
	}
	mora = mora.Add(emAbertoAte(inicio).Mul(moraPorDia).Mul(decimal.NewFromInt(int64(diasEntre(inicio, data))))).Round(2)

	e.multaEmAberto = decimal.Max(multa.Sub(multaPaga), decimal.Zero)
	e.moraEmAberto = decimal.Max(mora.Sub(moraPaga), decimal.Zero)
	return e, nil
}

func diasEntre(de, ate time.Time) int {
	return int((ate.Unix() - de.Unix()) / 86400)
}
