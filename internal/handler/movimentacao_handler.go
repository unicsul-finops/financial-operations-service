package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/shopspring/decimal"
	"github.com/unicsul-finops/financial-operations-service/internal/domain"
)

// corpo do pagamento de parcela e da liquidação. ponteiro separa o campo ausente do campo vazio.
type MovimentacaoRequest struct {
	Valor                  decimal.NullDecimal `json:"valor"`
	DataPagamento          *string             `json:"dataPagamento"`
	IdentificadorPagamento *string             `json:"identificadorPagamento"`
	Origem                 *string             `json:"origem"`
}

// id e registradoEm vêm do evento; o resto é o detalhe gravado, o mesmo que o reenvio devolve.
type PagamentoResponse struct {
	ID int64 `json:"id"`
	domain.Pagamento
	RegistradoEm string `json:"registradoEm"`
}

type LiquidacaoResponse struct {
	ID int64 `json:"id"`
	domain.Liquidacao
	RegistradoEm string `json:"registradoEm"`
}

type ValorLiquidacaoResponse struct {
	NumeroOperacao  string           `json:"numeroOperacao"`
	DataReferencia  string           `json:"dataReferencia"`
	ValorParaQuitar decimal.Decimal  `json:"valorParaQuitar"`
	SaldoDevedor    decimal.Decimal  `json:"saldoDevedor"`
	Vencidas        VencidasResponse `json:"vencidas"`
	AVencer         AVencerResponse  `json:"aVencer"`
}

type VencidasResponse struct {
	Parcelas []ParcelaVencidaResponse `json:"parcelas"`
	Total    decimal.Decimal          `json:"total"`
}

type ParcelaVencidaResponse struct {
	Numero        int             `json:"numero"`
	Vencimento    string          `json:"vencimento"`
	DiasEmAtraso  int             `json:"diasEmAtraso"`
	ValorEmAberto decimal.Decimal `json:"valorEmAberto"`
	Multa         decimal.Decimal `json:"multa"`
	JurosMora     decimal.Decimal `json:"jurosMora"`
	Total         decimal.Decimal `json:"total"`
}

type AVencerResponse struct {
	Parcelas      []ParcelaAVencerResponse `json:"parcelas"`
	ValorNominal  decimal.Decimal          `json:"valorNominal"`
	ValorPresente decimal.Decimal          `json:"valorPresente"`
	Desconto      decimal.Decimal          `json:"desconto"`
	Amortizacao   decimal.Decimal          `json:"amortizacao"`
	Juros         decimal.Decimal          `json:"juros"`
}

type ParcelaAVencerResponse struct {
	Numero        int             `json:"numero"`
	Vencimento    string          `json:"vencimento"`
	ValorEmAberto decimal.Decimal `json:"valorEmAberto"`
	ValorPresente decimal.Decimal `json:"valorPresente"`
	Desconto      decimal.Decimal `json:"desconto"`
}

// POST /operacoes/:identificador/parcelas/:numero/pagamentos - 201 quando registra, 200 no reenvio.
func (h *OperacaoHandler) PagarParcela(c *gin.Context) {
	var body MovimentacaoRequest
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"erro": "corpo da requisição inválido"})
		return
	}
	pagamento, novo, err := h.service.PagarParcela(c.Param("identificador"), c.Param("numero"), paraPedido(body))
	if err != nil {
		responderErro(c, err)
		return
	}
	resposta := PagamentoResponse{ID: pagamento.ID, Pagamento: pagamento, RegistradoEm: pagamento.RegistradoEm}
	if !novo {
		c.JSON(http.StatusOK, resposta)
		return
	}
	c.Header("Location", "/operacoes/"+pagamento.Operacao.NumeroOperacao+"/historico")
	c.JSON(http.StatusCreated, resposta)
}

// GET /operacoes/:identificador/liquidacao - valor para quitar na dataReferencia.
func (h *OperacaoHandler) ConsultarLiquidacao(c *gin.Context) {
	v, err := h.service.ConsultarLiquidacao(c.Param("identificador"), dataReferenciaQuery(c))
	if err != nil {
		responderErro(c, err)
		return
	}
	resposta := ValorLiquidacaoResponse{
		NumeroOperacao:  v.NumeroOperacao,
		DataReferencia:  v.DataReferencia,
		ValorParaQuitar: v.ValorParaQuitar,
		SaldoDevedor:    v.SaldoDevedor,
		Vencidas:        VencidasResponse{Parcelas: []ParcelaVencidaResponse{}, Total: v.TotalVencidas},
		AVencer: AVencerResponse{Parcelas: []ParcelaAVencerResponse{}, ValorNominal: v.ValorNominal, ValorPresente: v.ValorPresente,
			Desconto: v.Desconto, Amortizacao: v.Amortizacao, Juros: v.Juros},
	}
	for _, p := range v.Vencidas {
		resposta.Vencidas.Parcelas = append(resposta.Vencidas.Parcelas, ParcelaVencidaResponse{
			Numero: p.Numero, Vencimento: p.Vencimento, DiasEmAtraso: p.DiasEmAtraso, ValorEmAberto: p.ValorEmAberto,
			Multa: p.Distribuicao.Multa, JurosMora: p.Distribuicao.JurosMora, Total: p.Distribuicao.Total(),
		})
	}
	for _, p := range v.AVencer {
		resposta.AVencer.Parcelas = append(resposta.AVencer.Parcelas, ParcelaAVencerResponse(p))
	}
	c.JSON(http.StatusOK, resposta)
}

// POST /operacoes/:identificador/liquidacao - 201 quando liquida, 200 no reenvio.
func (h *OperacaoHandler) Liquidar(c *gin.Context) {
	var body MovimentacaoRequest
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"erro": "corpo da requisição inválido"})
		return
	}
	liquidacao, novo, err := h.service.Liquidar(c.Param("identificador"), paraPedido(body))
	if err != nil {
		responderErro(c, err)
		return
	}
	resposta := LiquidacaoResponse{ID: liquidacao.ID, Liquidacao: liquidacao, RegistradoEm: liquidacao.RegistradoEm}
	if !novo {
		c.JSON(http.StatusOK, resposta)
		return
	}
	c.Header("Location", "/operacoes/"+liquidacao.Operacao.NumeroOperacao+"/historico")
	c.JSON(http.StatusCreated, resposta)
}

func paraPedido(body MovimentacaoRequest) domain.PedidoMovimentacao {
	return domain.PedidoMovimentacao{
		Valor:                  body.Valor,
		DataPagamento:          body.DataPagamento,
		IdentificadorPagamento: body.IdentificadorPagamento,
		Origem:                 body.Origem,
	}
}

// GET /operacoes/:identificador/resumo - totais da operação na dataReferencia (RN-20).
func (h *OperacaoHandler) ConsultarResumo(c *gin.Context) {
	resumo, err := h.service.ConsultarResumo(c.Param("identificador"), dataReferenciaQuery(c))
	if err != nil {
		responderErro(c, err)
		return
	}
	c.JSON(http.StatusOK, resumo)
}

// GET /operacoes/:identificador/historico - eventos gravados e entradas calculadas, em ordem cronológica (RN-21).
func (h *OperacaoHandler) ConsultarHistorico(c *gin.Context) {
	itens, err := h.service.ConsultarHistorico(c.Param("identificador"), dataReferenciaQuery(c))
	if err != nil {
		responderErro(c, err)
		return
	}
	c.JSON(http.StatusOK, itens)
}
