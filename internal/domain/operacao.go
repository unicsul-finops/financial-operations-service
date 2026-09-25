package domain

import (
	"database/sql"

	"github.com/shopspring/decimal"
)

// os demais status ja existem no banco, mas so passam a ser usados nas proximas fases.
const (
	StatusOperacaoAtiva        = "ATIVA"
	StatusOperacaoEmAtraso     = "EM_ATRASO"
	StatusOperacaoInadimplente = "INADIMPLENTE"
	StatusOperacaoRenegociada  = "RENEGOCIADA"
	StatusOperacaoLiquidada    = "LIQUIDADA"
)

const (
	SistemaPrice = "PRICE"
	SistemaSAC   = "SAC"
)

// NumeroOperacao é só pra exibição, quem identifica a operação nas urls continua sendo o ID.
type Operacao struct {
	ID                 string
	NumeroOperacao     string
	ClienteID          string
	TipoPessoa         string
	Status             string
	DataAprovacao      string
	ValorAprovado      decimal.Decimal
	PrazoMeses         int
	QuantidadeParcelas int
	TaxaJuros          decimal.Decimal
	SistemaAmortizacao string
	PrimeiroVencimento string
	SaldoDevedor       decimal.Decimal
	ContextoScore      ContextoScore
	CriadoEm           string
}

// o contexto do score é opcional no payload, por isso os campos aceitam nulo.
type ContextoScore struct {
	ScoreFinal           sql.NullInt32
	FaixaRisco           sql.NullString
	ProbabilidadeDefault decimal.NullDecimal
	Modelo               sql.NullString
	Motivo               sql.NullString
}

type NovaOperacao struct {
	ClienteID          string
	TipoPessoa         string
	Decisao            string
	DataAprovacao      string
	ValorAprovado      decimal.Decimal
	PrazoMeses         int
	QuantidadeParcelas int
	// nullable pra diferenciar taxa ausente de taxa zero, senão faltar o campo viraria empréstimo sem juros.
	TaxaJuros          decimal.NullDecimal
	SistemaAmortizacao string
	PrimeiroVencimento string
	ContextoScore      ContextoScore
}
