package service

import (
	"errors"
	"time"

	"github.com/shopspring/decimal"
	"github.com/unicsul-finops/financial-operations-service/internal/domain"
)

const precisaoPotencia = 20

// f: fração já decorrida do período que termina no primeiro vencimento depois da data (RN-16).
// zero no dia de um vencimento e depois do último.
func fracaoDoPeriodo(calendario []domain.Parcela, dataLiberacao string, data time.Time) (decimal.Decimal, error) {
	dia := data.Format(time.DateOnly)
	for i, p := range calendario {
		if p.Vencimento <= dia {
			continue
		}
		anterior := dataLiberacao
		if i > 0 {
			anterior = calendario[i-1].Vencimento
		}
		inicio, err1 := time.ParseInLocation(time.DateOnly, anterior, fusoJuros)
		fim, err2 := time.ParseInLocation(time.DateOnly, p.Vencimento, fusoJuros)
		if err := errors.Join(err1, err2); err != nil {
			return decimal.Zero, err
		}
		return decimal.NewFromInt(int64(diasEntre(inicio, data))).Div(decimal.NewFromInt(int64(diasEntre(inicio, fim)))), nil
	}
	return decimal.Zero, nil
}

// (1 + i) ^ expoente, com o expoente fracionário.
func potenciaDaTaxa(taxa, expoente decimal.Decimal) (decimal.Decimal, error) {
	ln, err := decimal.NewFromInt(1).Add(taxa).Ln(precisaoPotencia)
	if err != nil {
		return decimal.Zero, err
	}
	return ln.Mul(expoente).ExpTaylor(precisaoPotencia)
}

// RN-16: VP = valor / (1 + i) ^ (k − f), contando períodos entre vencimentos.
// recebe as parcelas não pagas em ordem; a que vence na própria data entra pelo valor em aberto, sem desconto,
// e a contagem k começa na seguinte. cada valor presente é arredondado ao centavo, como no exemplo da RN-17.
func calcularValoresPresentes(calendario []domain.Parcela, aVencer []domain.Parcela, emAberto map[int]decimal.Decimal,
	taxa decimal.Decimal, dataLiberacao string, data time.Time) ([]domain.ParcelaAVencerLiquidacao, error) {
	resultado := make([]domain.ParcelaAVencerLiquidacao, 0, len(aVencer))
	dia := data.Format(time.DateOnly)

	fracao, err := fracaoDoPeriodo(calendario, dataLiberacao, data)
	if err != nil {
		return nil, err
	}
	k := 0
	for _, p := range aVencer {
		aberto := emAberto[p.Numero]
		vp := aberto
		if p.Vencimento > dia {
			k++
			fator, err := potenciaDaTaxa(taxa, decimal.NewFromInt(int64(k)).Sub(fracao))
			if err != nil {
				return nil, err
			}
			vp = aberto.Div(fator).Round(2)
		}
		resultado = append(resultado, domain.ParcelaAVencerLiquidacao{
			Numero: p.Numero, Vencimento: p.Vencimento, ValorEmAberto: aberto, ValorPresente: vp, Desconto: aberto.Sub(vp),
		})
	}
	return resultado, nil
}
