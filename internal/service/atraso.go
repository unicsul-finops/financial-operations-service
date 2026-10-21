package service

import (
	"errors"
	"time"

	"github.com/shopspring/decimal"
	"github.com/unicsul-finops/financial-operations-service/internal/domain"
)

func interpretarDataReferencia(valor *string, agora time.Time) (time.Time, error) {
	if valor == nil {
		return time.ParseInLocation(time.DateOnly, agora.In(fusoJuros).Format(time.DateOnly), fusoJuros)
	}
	data, err := time.ParseInLocation(time.DateOnly, *valor, fusoJuros)
	if err != nil || data.Format(time.DateOnly) != *valor || data.Year() < 1 {
		return time.Time{}, errors.New("dataReferencia deve estar no formato YYYY-MM-DD")
	}
	return data, nil
}

// recebe e devolve cópias: a situação consultada nunca altera o estado persistido.
func calcularSituacaoParcela(parcela domain.Parcela, referencia time.Time) (domain.Parcela, error) {
	vencimento, err := time.ParseInLocation(time.DateOnly, parcela.Vencimento, fusoJuros)
	if err != nil {
		return domain.Parcela{}, err
	}
	parcela.DiasEmAtraso = 0
	parcela.ValorEmAberto = decimal.Zero
	if parcela.Status == domain.StatusParcelaPaga {
		return parcela, nil
	}
	parcela.ValorEmAberto = parcela.Juros.Add(parcela.Amortizacao).
		Sub(parcela.JurosPagos).Sub(parcela.AmortizacaoPaga).Sub(parcela.DescontoConcedido)
	if parcela.ValorEmAberto.IsPositive() && referencia.After(vencimento) {
		parcela.DiasEmAtraso = int((referencia.Unix() - vencimento.Unix()) / 86400)
		parcela.Status = domain.StatusParcelaVencida
	}
	return parcela, nil
}
