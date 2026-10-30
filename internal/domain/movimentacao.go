package domain

import "github.com/shopspring/decimal"

const (
	EventoPagamento  = "PAGAMENTO"
	EventoLiquidacao = "LIQUIDACAO"
)

// para onde foi o dinheiro, na ordem de abatimento da RN-14.
type Distribuicao struct {
	Multa       decimal.Decimal `json:"multa"`
	JurosMora   decimal.Decimal `json:"jurosMora"`
	Juros       decimal.Decimal `json:"juros"`
	Amortizacao decimal.Decimal `json:"amortizacao"`
}

func (d Distribuicao) Total() decimal.Decimal {
	return d.Multa.Add(d.JurosMora).Add(d.Juros).Add(d.Amortizacao)
}

// lançamento já gravado de uma parcela, com a data do evento. é a base do cálculo dos encargos (RN-13).
type LancamentoParcela struct {
	NumeroParcela int
	Data          string
	Distribuicao
	Desconto decimal.Decimal
}

// retrato da operação usado pelo pagamento e pela liquidação.
type Carteira struct {
	Operacao           Operacao
	Parcelas           []Parcela
	Lancamentos        []LancamentoParcela
	UltimaMovimentacao string
}

// NumeroParcela zero é o lançamento da operação, sem parcela, das parcelas a vencer na liquidação (RN-18).
type NovoLancamento struct {
	NumeroParcela int
	Distribuicao
	Desconto decimal.Decimal
}

// tudo o que um pagamento ou uma liquidação grava, numa transação só (ADR-012).
type NovoEvento struct {
	Tipo                 string
	Data                 string
	Valor                decimal.Decimal
	IdentificadorExterno string
	Origem               string
	// vai para evento.detalhe; é o que o reenvio devolve.
	Detalhe        any
	Lancamentos    []NovoLancamento
	StatusParcelas map[int]string
	SaldoDevedor   decimal.Decimal
	Liquidar       bool
	// RN-19: o pagamento da última parcela grava também um evento LIQUIDACAO sem valor.
	EventoEncerramento bool
}

type EventoGravado struct {
	ID            int64
	Tipo          string
	NumeroParcela int
	RegistradoEm  string
	Detalhe       []byte
}

// detalhe gravado do evento PAGAMENTO. id e registradoEm vêm das colunas do evento.
type Pagamento struct {
	ID                     int64             `json:"-"`
	DataPagamento          string            `json:"dataPagamento"`
	Valor                  decimal.Decimal   `json:"valor"`
	IdentificadorPagamento string            `json:"identificadorPagamento,omitempty"`
	Origem                 string            `json:"origem,omitempty"`
	Distribuicao           Distribuicao      `json:"distribuicao"`
	Parcela                ParcelaAposPagar  `json:"parcela"`
	Operacao               OperacaoAposMover `json:"operacao"`
	RegistradoEm           string            `json:"-"`
}

type ParcelaAposPagar struct {
	Numero           int             `json:"numero"`
	Status           string          `json:"status"`
	ValorEmAberto    decimal.Decimal `json:"valorEmAberto"`
	EncargosEmAberto decimal.Decimal `json:"encargosEmAberto"`
}

type OperacaoAposMover struct {
	NumeroOperacao string          `json:"numeroOperacao"`
	Status         string          `json:"status"`
	SaldoDevedor   decimal.Decimal `json:"saldoDevedor"`
}

// valor para quitar numa data (RN-18). a parcela que vence no dia entra em AVencer.
type ValorLiquidacao struct {
	NumeroOperacao  string
	DataReferencia  string
	ValorParaQuitar decimal.Decimal
	SaldoDevedor    decimal.Decimal
	Vencidas        []ParcelaVencidaLiquidacao
	TotalVencidas   decimal.Decimal
	AVencer         []ParcelaAVencerLiquidacao
	ValorNominal    decimal.Decimal
	ValorPresente   decimal.Decimal
	Desconto        decimal.Decimal
	Amortizacao     decimal.Decimal
	Juros           decimal.Decimal
}

type ParcelaVencidaLiquidacao struct {
	Numero        int
	Vencimento    string
	DiasEmAtraso  int
	Distribuicao  Distribuicao
	ValorEmAberto decimal.Decimal
}

type ParcelaAVencerLiquidacao struct {
	Numero        int
	Vencimento    string
	ValorEmAberto decimal.Decimal
	ValorPresente decimal.Decimal
	Desconto      decimal.Decimal
}

// detalhe gravado do evento LIQUIDACAO.
type Liquidacao struct {
	ID                     int64               `json:"-"`
	DataPagamento          string              `json:"dataPagamento"`
	Valor                  decimal.Decimal     `json:"valor"`
	IdentificadorPagamento string              `json:"identificadorPagamento,omitempty"`
	Origem                 string              `json:"origem,omitempty"`
	Vencidas               []LiquidacaoVencida `json:"vencidas"`
	AVencer                LiquidacaoAVencer   `json:"aVencer"`
	Operacao               OperacaoAposMover   `json:"operacao"`
	RegistradoEm           string              `json:"-"`
}

type LiquidacaoVencida struct {
	Numero       int          `json:"numero"`
	Distribuicao Distribuicao `json:"distribuicao"`
}

type LiquidacaoAVencer struct {
	Parcelas     []int           `json:"parcelas"`
	ValorNominal decimal.Decimal `json:"valorNominal"`
	Desconto     decimal.Decimal `json:"desconto"`
	Amortizacao  decimal.Decimal `json:"amortizacao"`
	Juros        decimal.Decimal `json:"juros"`
	Valor        decimal.Decimal `json:"valor"`
}

// corpo do pagamento e da liquidação. ponteiros e NullDecimal separam o campo ausente do campo vazio.
type PedidoMovimentacao struct {
	Valor                  decimal.NullDecimal
	DataPagamento          *string
	IdentificadorPagamento *string
	Origem                 *string
}
