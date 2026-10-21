package domain

import "github.com/shopspring/decimal"

// o status fica gravado porque PARCIALMENTE_PAGA (fase 2) não dá pra derivar só pela data.
const (
	StatusParcelaPendente         = "PENDENTE"
	StatusParcelaPaga             = "PAGA"
	StatusParcelaParcialmentePaga = "PARCIALMENTE_PAGA"
	StatusParcelaVencida          = "VENCIDA"
)

type Parcela struct {
	ID                int64
	OperacaoID        int64
	Numero            int
	Vencimento        string
	ValorParcela      decimal.Decimal
	Amortizacao       decimal.Decimal
	Juros             decimal.Decimal
	SaldoDevedor      decimal.Decimal
	Status            string
	JurosPagos        decimal.Decimal
	AmortizacaoPaga   decimal.Decimal
	DescontoConcedido decimal.Decimal
	ValorEmAberto     decimal.Decimal
	DiasEmAtraso      int
}

type NovaParcela struct {
	Numero       int
	Vencimento   string
	ValorParcela decimal.Decimal
	Amortizacao  decimal.Decimal
	Juros        decimal.Decimal
	SaldoDevedor decimal.Decimal
}
