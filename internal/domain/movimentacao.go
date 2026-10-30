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
	// lançamentos da operação, sem parcela: as parcelas a vencer na liquidação (RN-18). só entram no resumo.
	LancamentoOperacao LancamentoParcela
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
	// RN-17: parcelas que continuam em aberto com a repartição entre juros e amortização refeita.
	ParcelasRecalculadas []ParcelaRecalculada
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

const EventoAmortizacao = "AMORTIZACAO_ANTECIPADA"

// quais parcelas a amortização quita (RN-17): as últimas, reduzindo o prazo (o padrão), ou as próximas a vencer.
const (
	ModoUltimas  = "ULTIMAS"
	ModoProximas = "PROXIMAS"
)

// uma parcela antecipada. valorPresente = amortizacao + juros e desconto = valorParcela − valorPresente (RN-16 e RN-17).
type ParcelaAmortizada struct {
	Numero        int             `json:"numero"`
	Vencimento    string          `json:"vencimento"`
	ValorParcela  decimal.Decimal `json:"valorParcela"`
	ValorPresente decimal.Decimal `json:"valorPresente"`
	Desconto      decimal.Decimal `json:"desconto"`
	Amortizacao   decimal.Decimal `json:"amortizacao"`
	Juros         decimal.Decimal `json:"juros"`
}

type TotaisAmortizacao struct {
	ValorParcelas decimal.Decimal `json:"valorParcelas"`
	Desconto      decimal.Decimal `json:"desconto"`
	ValorPresente decimal.Decimal `json:"valorPresente"`
	Amortizacao   decimal.Decimal `json:"amortizacao"`
	Juros         decimal.Decimal `json:"juros"`
	ValorAPagar   decimal.Decimal `json:"valorAPagar"`
}

// repartição de uma parcela que continua em aberto, antes e depois da antecipação (RN-17).
type ReparticaoParcela struct {
	Juros        decimal.Decimal `json:"juros"`
	Amortizacao  decimal.Decimal `json:"amortizacao"`
	SaldoDevedor decimal.Decimal `json:"saldoDevedor"`
}

type ParcelaRecalculada struct {
	Numero int               `json:"numero"`
	Antes  ReparticaoParcela `json:"antes"`
	Depois ReparticaoParcela `json:"depois"`
}

// resultado da simulação e, gravado no evento, da amortização. a simulação e o registro usam a mesma conta.
type SimulacaoAmortizacao struct {
	NumeroOperacao              string
	Modo                        string
	DataReferencia              string
	QuantidadeParcelas          int
	Parcelas                    []ParcelaAmortizada
	Totais                      TotaisAmortizacao
	SaldoDevedorAntes           decimal.Decimal
	SaldoDevedorApos            decimal.Decimal
	QuantidadeParcelasRestantes int
	Recalculadas                []ParcelaRecalculada
}

// detalhe gravado do evento AMORTIZACAO_ANTECIPADA. id e registradoEm vêm das colunas do evento.
type Amortizacao struct {
	ID                          int64               `json:"-"`
	NumeroOperacao              string              `json:"numeroOperacao"`
	Modo                        string              `json:"modo"`
	IdentificadorPagamento      string              `json:"identificadorPagamento,omitempty"`
	DataPagamento               string              `json:"dataPagamento"`
	QuantidadeParcelas          int                 `json:"quantidadeParcelas"`
	Lancamentos                 []ParcelaAmortizada `json:"lancamentos"`
	Totais                      TotaisAmortizacao   `json:"totais"`
	SaldoDevedorAntes           decimal.Decimal     `json:"saldoDevedorAntes"`
	SaldoDevedorApos            decimal.Decimal     `json:"saldoDevedorApos"`
	QuantidadeParcelasRestantes int                 `json:"quantidadeParcelasRestantes"`
	// só no histórico: como a tabela das parcelas que sobraram ficou depois da antecipação.
	ParcelasRecalculadas []ParcelaRecalculada `json:"parcelasRecalculadas,omitempty"`
}

// corpo do POST de amortização. ponteiro e NullDecimal separam o campo ausente do campo vazio.
type PedidoAmortizacao struct {
	QuantidadeParcelas     decimal.NullDecimal
	Modo                   *string
	DataPagamento          *string
	IdentificadorPagamento *string
}

// resumo financeiro da operação na dataReferencia (RN-20). totalPago = valorAmortizado + jurosPagos + encargosPagos.
type Resumo struct {
	NumeroOperacao      string          `json:"numeroOperacao"`
	DataReferencia      string          `json:"dataReferencia"`
	Status              string          `json:"status"`
	DiasEmAtraso        int             `json:"diasEmAtraso"`
	TotalContratado     decimal.Decimal `json:"totalContratado"`
	TotalPrevisto       decimal.Decimal `json:"totalPrevisto"`
	TotalPago           decimal.Decimal `json:"totalPago"`
	ValorAmortizado     decimal.Decimal `json:"valorAmortizado"`
	JurosPagos          decimal.Decimal `json:"jurosPagos"`
	EncargosPagos       decimal.Decimal `json:"encargosPagos"`
	DescontoAntecipacao decimal.Decimal `json:"descontoAntecipacao"`
	ValorVencido        decimal.Decimal `json:"valorVencido"`
	EncargosEmAberto    decimal.Decimal `json:"encargosEmAberto"`
	SaldoDevedor        decimal.Decimal `json:"saldoDevedor"`
	ValorParaQuitar     decimal.Decimal `json:"valorParaQuitar"`
}

const (
	EventoOperacaoRegistrada = "OPERACAO_REGISTRADA"

	// itens calculados do histórico (RN-21): não são gravados, saem da situação na dataReferencia.
	ItemParcelaVencida       = "PARCELA_VENCIDA"
	ItemOperacaoInadimplente = "OPERACAO_INADIMPLENTE"

	OrigemRegistrado = "REGISTRADO"
	OrigemCalculado  = "CALCULADO"
)

// evento gravado como o banco devolve, com os lançamentos dele.
type EventoHistorico struct {
	ID                     int64
	Tipo                   string
	DataEfeito             string
	RegistradoEm           string
	Valor                  decimal.NullDecimal
	IdentificadorPagamento string
	Origem                 string
	Lancamentos            []LancamentoEvento
}

// NumeroParcela zero é o lançamento da operação, sem parcela (RN-18).
type LancamentoEvento struct {
	NumeroParcela int
	Distribuicao
	Desconto decimal.Decimal
}

type Historico struct {
	Carteira Carteira
	Eventos  []EventoHistorico
}

type LancamentoItem struct {
	NumeroParcela *int            `json:"numeroParcela"`
	Multa         decimal.Decimal `json:"multa"`
	JurosMora     decimal.Decimal `json:"jurosMora"`
	Juros         decimal.Decimal `json:"juros"`
	Amortizacao   decimal.Decimal `json:"amortizacao"`
	Desconto      decimal.Decimal `json:"desconto"`
}

// item do histórico: um evento gravado (REGISTRADO) ou uma entrada calculada na dataReferencia (CALCULADO).
// id, registradoEm, valor, identificadorPagamento e origemPagamento só existem nos itens gravados que os têm.
type ItemHistorico struct {
	ID                     int64            `json:"id,omitempty"`
	Tipo                   string           `json:"tipo"`
	Origem                 string           `json:"origem"`
	DataEfeito             string           `json:"dataEfeito"`
	RegistradoEm           string           `json:"registradoEm,omitempty"`
	Valor                  *decimal.Decimal `json:"valor,omitempty"`
	IdentificadorPagamento string           `json:"identificadorPagamento,omitempty"`
	OrigemPagamento        string           `json:"origemPagamento,omitempty"`
	NumeroParcela          *int             `json:"numeroParcela,omitempty"`
	Lancamentos            []LancamentoItem `json:"lancamentos"`
	Descricao              string           `json:"descricao,omitempty"`
}
