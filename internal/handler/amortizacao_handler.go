package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/shopspring/decimal"
	"github.com/unicsul-finops/financial-operations-service/internal/domain"
)

// corpo do POST de amortização. ponteiro e NullDecimal separam o campo ausente do campo vazio.
type AmortizacaoRequest struct {
	QuantidadeParcelas     decimal.NullDecimal `json:"quantidadeParcelas"`
	Modo                   *string             `json:"modo"`
	DataPagamento          *string             `json:"dataPagamento"`
	IdentificadorPagamento *string             `json:"identificadorPagamento"`
}

type SimulacaoAmortizacaoResponse struct {
	NumeroOperacao              string                     `json:"numeroOperacao"`
	Modo                        string                     `json:"modo"`
	DataReferencia              string                     `json:"dataReferencia"`
	QuantidadeParcelas          int                        `json:"quantidadeParcelas"`
	Parcelas                    []domain.ParcelaAmortizada `json:"parcelas"`
	Totais                      domain.TotaisAmortizacao   `json:"totais"`
	SaldoDevedorAntes           decimal.Decimal            `json:"saldoDevedorAntes"`
	SaldoDevedorApos            decimal.Decimal            `json:"saldoDevedorApos"`
	QuantidadeParcelasRestantes int                        `json:"quantidadeParcelasRestantes"`
}

// eventoId e tipo vêm do evento; o resto é o detalhe gravado, o mesmo que o reenvio devolve.
type AmortizacaoRegistradaResponse struct {
	EventoID int64  `json:"eventoId"`
	Tipo     string `json:"tipo"`
	domain.Amortizacao
}

// GET /operacoes/:identificador/amortizacoes/simulacao - quanto custa antecipar as últimas parcelas, sem gravar nada.
func (h *OperacaoHandler) SimularAmortizacao(c *gin.Context) {
	var quantidade *string
	if valores, presente := c.Request.URL.Query()["quantidadeParcelas"]; presente {
		quantidade = &valores[0]
	}
	var modo *string
	if valores, presente := c.Request.URL.Query()["modo"]; presente {
		modo = &valores[0]
	}
	s, err := h.service.SimularAmortizacao(c.Param("identificador"), quantidade, modo, dataReferenciaQuery(c))
	if err != nil {
		responderErro(c, err)
		return
	}
	c.JSON(http.StatusOK, SimulacaoAmortizacaoResponse{
		NumeroOperacao: s.NumeroOperacao, Modo: s.Modo, DataReferencia: s.DataReferencia, QuantidadeParcelas: s.QuantidadeParcelas,
		Parcelas: s.Parcelas, Totais: s.Totais, SaldoDevedorAntes: s.SaldoDevedorAntes, SaldoDevedorApos: s.SaldoDevedorApos,
		QuantidadeParcelasRestantes: s.QuantidadeParcelasRestantes,
	})
}

// POST /operacoes/:identificador/amortizacoes - 201 quando registra, 200 no reenvio.
func (h *OperacaoHandler) RegistrarAmortizacao(c *gin.Context) {
	var body AmortizacaoRequest
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"erro": "corpo da requisição inválido"})
		return
	}
	amortizacao, novo, err := h.service.RegistrarAmortizacao(c.Param("identificador"), domain.PedidoAmortizacao{
		QuantidadeParcelas:     body.QuantidadeParcelas,
		Modo:                   body.Modo,
		DataPagamento:          body.DataPagamento,
		IdentificadorPagamento: body.IdentificadorPagamento,
	})
	if err != nil {
		responderErro(c, err)
		return
	}
	resposta := AmortizacaoRegistradaResponse{EventoID: amortizacao.ID, Tipo: domain.EventoAmortizacao, Amortizacao: amortizacao}
	if !novo {
		c.JSON(http.StatusOK, resposta)
		return
	}
	c.Header("Location", "/operacoes/"+amortizacao.NumeroOperacao)
	c.JSON(http.StatusCreated, resposta)
}
