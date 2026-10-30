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

func calcularSituacaoParcelas(parcelas []domain.Parcela, referencia time.Time) ([]domain.Parcela, error) {
	calculadas := make([]domain.Parcela, 0, len(parcelas))
	for _, parcela := range parcelas {
		calculada, err := calcularSituacaoParcela(parcela, referencia)
		if err != nil {
			return nil, err
		}
		calculadas = append(calculadas, calculada)
	}
	return calculadas, nil
}

// RN-12: inadimplente com mais de 90 dias de atraso.
const limiteInadimplencia = 90

// RN-11 e RN-12: o status de atraso é calculado na consulta e nunca gravado. LIQUIDADA continua LIQUIDADA.
// as parcelas já vêm com a situação calculada para a mesma data.
func calcularSituacaoOperacao(operacao domain.Operacao, parcelas []domain.Parcela) domain.Operacao {
	operacao.DiasEmAtraso = 0
	for _, parcela := range parcelas {
		if parcela.DiasEmAtraso > operacao.DiasEmAtraso {
			operacao.DiasEmAtraso = parcela.DiasEmAtraso
		}
	}
	if operacao.Status != domain.StatusOperacaoAtiva {
		return operacao
	}
	switch {
	case operacao.DiasEmAtraso > limiteInadimplencia:
		operacao.Status = domain.StatusOperacaoInadimplente
	case operacao.DiasEmAtraso > 0:
		operacao.Status = domain.StatusOperacaoEmAtraso
	}
	return operacao
}

// recebe e devolve cópias: a situação consultada nunca altera o estado persistido.
func calcularSituacaoParcela(parcela domain.Parcela, referencia time.Time) (domain.Parcela, error) {
	vencimento, err := time.ParseInLocation(time.DateOnly, parcela.Vencimento, fusoJuros)
	if err != nil {
		return domain.Parcela{}, err
	}
	parcela.DiasEmAtraso = 0
	parcela.ValorEmAberto = decimal.Zero
	parcela.MultaEmAberto, parcela.MoraEmAberto, parcela.TotalParaQuitar = decimal.Zero, decimal.Zero, decimal.Zero
	if parcela.Status == domain.StatusParcelaPaga {
		return parcela, nil
	}
	parcela.ValorEmAberto = parcela.Juros.Add(parcela.Amortizacao).
		Sub(parcela.JurosPagos).Sub(parcela.AmortizacaoPaga).Sub(parcela.DescontoConcedido)
	encargos, err := calcularEncargos(parcela, parcela.Lancamentos, referencia)
	if err != nil {
		return domain.Parcela{}, err
	}
	parcela.MultaEmAberto = encargos.multaEmAberto
	parcela.MoraEmAberto = encargos.moraEmAberto
	parcela.TotalParaQuitar = encargos.totalParaQuitar()
	if parcela.ValorEmAberto.IsPositive() && referencia.After(vencimento) {
		parcela.DiasEmAtraso = int((referencia.Unix() - vencimento.Unix()) / 86400)
		parcela.Status = domain.StatusParcelaVencida
	}
	return parcela, nil
}
