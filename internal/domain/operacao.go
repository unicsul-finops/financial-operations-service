package domain

import (
	"time"

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

// estado da operação no serviço de juros. só a operação definida (fechada com o cliente) vira contrato aqui.
const EstadoJurosDefinida = "DEFINIDA"

// NumeroOperacao é o identificador do serviço de juros, o mesmo em toda a esteira. o ID continua sendo a chave técnica.
// todas as taxas ficam em fração (0.0185 = 1,85%).
type Operacao struct {
	ID                 int64
	NumeroOperacao     string
	ClienteID          string
	TipoPessoa         string
	Status             string
	DefinidoEm         string
	DataLiberacao      string
	ProdutoCodigo      string
	ProdutoNome        string
	ModalidadeBcb      string
	ValorAprovado      decimal.Decimal
	PrazoMeses         int
	QuantidadeParcelas int
	TaxaJuros          decimal.Decimal
	TaxaJurosAnual     decimal.Decimal
	FaixaRisco         string
	TetoAplicado       bool
	SistemaAmortizacao string
	PrimeiroVencimento string
	SaldoDevedor       decimal.Decimal
	IOFDiario          decimal.Decimal
	IOFAdicional       decimal.Decimal
	IOFTotal           decimal.Decimal
	TarifaCadastro     decimal.Decimal
	EncargosTotal      decimal.Decimal
	ValorLiberado      decimal.Decimal
	CETMensal          decimal.Decimal
	CETAnual           decimal.Decimal
	CriadoEm           string
}

// operação como chega do serviço de juros. as taxas vêm em percentual e o service preenche os campos derivados.
type NovaOperacao struct {
	Identificador string
	Estado        string
	DefinidoEm    string

	ClienteID  string
	TipoPessoa string

	ProdutoCodigo string
	ProdutoNome   string
	ModalidadeBcb string

	ValorAprovado decimal.Decimal
	PrazoMeses    int
	DataLiberacao string

	// nullable pra diferenciar taxa ausente de taxa zero, senão faltar o campo viraria empréstimo sem juros.
	TaxaMensalPercentual decimal.NullDecimal
	TaxaAnualPercentual  decimal.NullDecimal
	FaixaRisco           string
	TetoAplicado         bool

	SistemaAmortizacao string

	// o resumo não é gravado, só serve pra conferir o calendário.
	TotalJuros decimal.Decimal
	TotalPago  decimal.Decimal

	IOFDiario      decimal.Decimal
	IOFAdicional   decimal.Decimal
	IOFTotal       decimal.Decimal
	TarifaCadastro decimal.Decimal
	EncargosTotal  decimal.Decimal
	ValorLiberado  decimal.Decimal

	CETMensalPercentual decimal.NullDecimal
	CETAnualPercentual  decimal.NullDecimal

	Parcelas []NovaParcela

	// derivados, preenchidos pela validação.
	DefinidoEmUTC      time.Time
	QuantidadeParcelas int
	PrimeiroVencimento string
	TaxaJuros          decimal.Decimal
	TaxaJurosAnual     decimal.Decimal
	CETMensal          decimal.Decimal
	CETAnual           decimal.Decimal
}
