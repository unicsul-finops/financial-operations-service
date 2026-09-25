package handler

import (
	"database/sql"
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/shopspring/decimal"
	"github.com/unicsul-finops/financial-operations-service/internal/domain"
	"github.com/unicsul-finops/financial-operations-service/internal/service"
)

// payload combinado com o grupo de decisão. é aninhado, diferente do domínio que é flat.
type CriarOperacaoRequest struct {
	ClienteID            string                      `json:"clienteId"`
	TipoPessoa           string                      `json:"tipoPessoa"`
	Decisao              string                      `json:"decisao"`
	DataAprovacao        string                      `json:"dataAprovacao"`
	CondicoesFinanceiras CondicoesFinanceirasRequest `json:"condicoesFinanceiras"`
	ContextoScore        *ContextoScoreRequest       `json:"contextoScore"`
}

type CondicoesFinanceirasRequest struct {
	ValorAprovado      decimal.Decimal     `json:"valorAprovado"`
	PrazoMeses         int                 `json:"prazoMeses"`
	QuantidadeParcelas int                 `json:"quantidadeParcelas"`
	TaxaJuros          decimal.NullDecimal `json:"taxaJuros"`
	SistemaAmortizacao string              `json:"sistemaAmortizacao"`
	PrimeiroVencimento string              `json:"primeiroVencimento"`
}

type ContextoScoreRequest struct {
	ScoreFinal           *int32              `json:"scoreFinal"`
	FaixaRisco           string              `json:"faixaRisco"`
	ProbabilidadeDefault decimal.NullDecimal `json:"probabilidadeDefault"`
	Modelo               string              `json:"modelo"`
	Motivo               string              `json:"motivo"`
}

type OperacaoResponse struct {
	ID                   string                       `json:"id"`
	NumeroOperacao       string                       `json:"numeroOperacao"`
	ClienteID            string                       `json:"clienteId"`
	TipoPessoa           string                       `json:"tipoPessoa"`
	Status               string                       `json:"status"`
	DataAprovacao        string                       `json:"dataAprovacao"`
	SaldoDevedor         decimal.Decimal              `json:"saldoDevedor"`
	CondicoesFinanceiras CondicoesFinanceirasResponse `json:"condicoesFinanceiras"`
	ContextoScore        *ContextoScoreResponse       `json:"contextoScore,omitempty"`
	CriadoEm             string                       `json:"criadoEm"`
	Parcelas             []ParcelaResponse            `json:"parcelas,omitempty"`
}

type CondicoesFinanceirasResponse struct {
	ValorAprovado      decimal.Decimal `json:"valorAprovado"`
	PrazoMeses         int             `json:"prazoMeses"`
	QuantidadeParcelas int             `json:"quantidadeParcelas"`
	TaxaJuros          decimal.Decimal `json:"taxaJuros"`
	SistemaAmortizacao string          `json:"sistemaAmortizacao"`
	PrimeiroVencimento string          `json:"primeiroVencimento"`
}

type ContextoScoreResponse struct {
	ScoreFinal           *int32           `json:"scoreFinal,omitempty"`
	FaixaRisco           string           `json:"faixaRisco,omitempty"`
	ProbabilidadeDefault *decimal.Decimal `json:"probabilidadeDefault,omitempty"`
	Modelo               string           `json:"modelo,omitempty"`
	Motivo               string           `json:"motivo,omitempty"`
}

type ParcelaResponse struct {
	Numero       int             `json:"numero"`
	Vencimento   string          `json:"vencimento"`
	ValorParcela decimal.Decimal `json:"valorParcela"`
	Amortizacao  decimal.Decimal `json:"amortizacao"`
	Juros        decimal.Decimal `json:"juros"`
	SaldoDevedor decimal.Decimal `json:"saldoDevedor"`
	Status       string          `json:"status"`
}

type OperacaoHandler struct {
	service *service.OperacaoService
}

func NovoOperacaoHandler(service *service.OperacaoService) *OperacaoHandler {
	return &OperacaoHandler{service: service}
}

// POST /operacoes - recebe a operação aprovada pela decisão e gera o cronograma.
func (h *OperacaoHandler) Criar(c *gin.Context) {
	var body CriarOperacaoRequest

	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"erro": "corpo da requisição inválido"})
		return
	}

	operacao, parcelas, err := h.service.CriarOperacao(paraNovaOperacao(body))
	if err != nil {
		responderErro(c, err)
		return
	}

	resposta := paraOperacaoResponse(operacao)
	resposta.Parcelas = paraParcelasResponse(parcelas)

	c.JSON(http.StatusCreated, resposta)
}

// GET /operacoes - lista as operações, sem as parcelas pra não pesar a resposta.
func (h *OperacaoHandler) Listar(c *gin.Context) {
	operacoes, err := h.service.ListarOperacoes()
	if err != nil {
		responderErro(c, err)
		return
	}

	resposta := make([]OperacaoResponse, 0, len(operacoes))
	for _, operacao := range operacoes {
		resposta = append(resposta, paraOperacaoResponse(operacao))
	}

	c.JSON(http.StatusOK, resposta)
}

// GET /operacoes/:id - situação atual da operação. o :id aceita o uuid ou o numeroOperacao.
func (h *OperacaoHandler) Buscar(c *gin.Context) {
	operacao, err := h.service.BuscarOperacao(c.Param("id"))
	if err != nil {
		responderErro(c, err)
		return
	}

	c.JSON(http.StatusOK, paraOperacaoResponse(operacao))
}

// GET /operacoes/:id/parcelas - cronograma com a situação de cada parcela. o :id aceita o uuid ou o numeroOperacao.
func (h *OperacaoHandler) ListarParcelas(c *gin.Context) {
	parcelas, err := h.service.ListarParcelas(c.Param("id"))
	if err != nil {
		responderErro(c, err)
		return
	}

	c.JSON(http.StatusOK, paraParcelasResponse(parcelas))
}

func responderErro(c *gin.Context, err error) {
	if errors.Is(err, domain.ErrOperacaoNaoEncontrada) {
		c.JSON(http.StatusNotFound, gin.H{"erro": err.Error()})
		return
	}

	c.JSON(http.StatusBadRequest, gin.H{"erro": err.Error()})
}

func paraNovaOperacao(body CriarOperacaoRequest) domain.NovaOperacao {
	nova := domain.NovaOperacao{
		ClienteID:          body.ClienteID,
		TipoPessoa:         body.TipoPessoa,
		Decisao:            body.Decisao,
		DataAprovacao:      body.DataAprovacao,
		ValorAprovado:      body.CondicoesFinanceiras.ValorAprovado,
		PrazoMeses:         body.CondicoesFinanceiras.PrazoMeses,
		QuantidadeParcelas: body.CondicoesFinanceiras.QuantidadeParcelas,
		TaxaJuros:          body.CondicoesFinanceiras.TaxaJuros,
		SistemaAmortizacao: body.CondicoesFinanceiras.SistemaAmortizacao,
		PrimeiroVencimento: body.CondicoesFinanceiras.PrimeiroVencimento,
	}

	if body.ContextoScore != nil {
		contexto := body.ContextoScore
		nova.ContextoScore = domain.ContextoScore{
			FaixaRisco:           sql.NullString{String: contexto.FaixaRisco},
			ProbabilidadeDefault: contexto.ProbabilidadeDefault,
			Modelo:               sql.NullString{String: contexto.Modelo},
			Motivo:               sql.NullString{String: contexto.Motivo},
		}
		if contexto.ScoreFinal != nil {
			nova.ContextoScore.ScoreFinal = sql.NullInt32{Int32: *contexto.ScoreFinal, Valid: true}
		}
	}

	return nova
}

func paraOperacaoResponse(operacao domain.Operacao) OperacaoResponse {
	return OperacaoResponse{
		ID:             operacao.ID,
		NumeroOperacao: operacao.NumeroOperacao,
		ClienteID:      operacao.ClienteID,
		TipoPessoa:     operacao.TipoPessoa,
		Status:         operacao.Status,
		DataAprovacao:  operacao.DataAprovacao,
		SaldoDevedor:   operacao.SaldoDevedor,
		CondicoesFinanceiras: CondicoesFinanceirasResponse{
			ValorAprovado:      operacao.ValorAprovado,
			PrazoMeses:         operacao.PrazoMeses,
			QuantidadeParcelas: operacao.QuantidadeParcelas,
			TaxaJuros:          operacao.TaxaJuros,
			SistemaAmortizacao: operacao.SistemaAmortizacao,
			PrimeiroVencimento: operacao.PrimeiroVencimento,
		},
		ContextoScore: paraContextoScoreResponse(operacao.ContextoScore),
		CriadoEm:      operacao.CriadoEm,
	}
}

// devolve nil quando nada do score foi gravado, pra o campo sumir da resposta.
func paraContextoScoreResponse(contexto domain.ContextoScore) *ContextoScoreResponse {
	if !contexto.ScoreFinal.Valid && !contexto.FaixaRisco.Valid && !contexto.ProbabilidadeDefault.Valid &&
		!contexto.Modelo.Valid && !contexto.Motivo.Valid {
		return nil
	}

	resposta := &ContextoScoreResponse{
		FaixaRisco: contexto.FaixaRisco.String,
		Modelo:     contexto.Modelo.String,
		Motivo:     contexto.Motivo.String,
	}
	if contexto.ScoreFinal.Valid {
		resposta.ScoreFinal = &contexto.ScoreFinal.Int32
	}
	if contexto.ProbabilidadeDefault.Valid {
		resposta.ProbabilidadeDefault = &contexto.ProbabilidadeDefault.Decimal
	}

	return resposta
}

func paraParcelasResponse(parcelas []domain.Parcela) []ParcelaResponse {
	resposta := make([]ParcelaResponse, 0, len(parcelas))

	for _, parcela := range parcelas {
		resposta = append(resposta, ParcelaResponse{
			Numero:       parcela.Numero,
			Vencimento:   parcela.Vencimento,
			ValorParcela: parcela.ValorParcela,
			Amortizacao:  parcela.Amortizacao,
			Juros:        parcela.Juros,
			SaldoDevedor: parcela.SaldoDevedor,
			Status:       parcela.Status,
		})
	}

	return resposta
}
