package handler

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/shopspring/decimal"
	"github.com/unicsul-finops/financial-operations-service/internal/domain"
	"github.com/unicsul-finops/financial-operations-service/internal/service"
)

// payload enviado pelo serviço de juros, com a simulação que o cliente escolheu. é aninhado, diferente do domínio que é flat.
// o demonstrativo e os campos que não são gravados nem são declarados aqui, o bind ignora.
type CriarOperacaoRequest struct {
	Operacao      OperacaoJurosRequest `json:"operacao"`
	Cliente       ClienteRequest       `json:"cliente"`
	Produto       ProdutoRequest       `json:"produto"`
	Condicoes     CondicoesRequest     `json:"condicoes"`
	Taxa          TaxaRequest          `json:"taxa"`
	Sistema       string               `json:"sistema"`
	Resumo        ResumoRequest        `json:"resumo"`
	Encargos      EncargosRequest      `json:"encargos"`
	ValorLiberado decimal.Decimal      `json:"valorLiberado"`
	CET           CETRequest           `json:"cet"`
	Calendario    []ParcelaRequest     `json:"calendario"`
}

type OperacaoJurosRequest struct {
	Identificador string `json:"identificador"`
	Estado        string `json:"estado"`
	DefinidoEm    string `json:"definidoEm"`
}

type ClienteRequest struct {
	TipoPessoa string `json:"tipoPessoa"`
	Documento  string `json:"documento"`
}

type ProdutoRequest struct {
	Codigo        string `json:"codigo"`
	Nome          string `json:"nome"`
	ModalidadeBcb string `json:"modalidadeBcb"`
}

type CondicoesRequest struct {
	Valor         decimal.Decimal `json:"valor"`
	PrazoMeses    int             `json:"prazoMeses"`
	DataLiberacao string          `json:"dataLiberacao"`
}

type TaxaRequest struct {
	FaixaRisco   string              `json:"faixaRisco"`
	TaxaFinalMes decimal.NullDecimal `json:"taxaFinalMes"`
	TaxaFinalAno decimal.NullDecimal `json:"taxaFinalAno"`
	TetoAplicado bool                `json:"tetoAplicado"`
}

type ResumoRequest struct {
	TotalJuros decimal.Decimal `json:"totalJuros"`
	TotalPago  decimal.Decimal `json:"totalPago"`
}

type EncargosRequest struct {
	IOF            IOFRequest      `json:"iof"`
	TarifaCadastro decimal.Decimal `json:"tarifaCadastro"`
	Total          decimal.Decimal `json:"total"`
}

type IOFRequest struct {
	Diario    decimal.Decimal `json:"diario"`
	Adicional decimal.Decimal `json:"adicional"`
	Total     decimal.Decimal `json:"total"`
}

type CETRequest struct {
	AnualPercentual  decimal.NullDecimal `json:"anualPercentual"`
	MensalPercentual decimal.NullDecimal `json:"mensalPercentual"`
}

type ParcelaRequest struct {
	Numero       int             `json:"numero"`
	Amortizacao  decimal.Decimal `json:"amortizacao"`
	Juros        decimal.Decimal `json:"juros"`
	Valor        decimal.Decimal `json:"valor"`
	SaldoDevedor decimal.Decimal `json:"saldoDevedor"`
	Vencimento   string          `json:"vencimento"`
}

// resposta do POST. o juros acabou de mandar todos os dados, então só confirma o registro e diz como consultar.
type OperacaoCriadaResponse struct {
	ID             int64  `json:"id"`
	NumeroOperacao string `json:"numeroOperacao"`
	Status         string `json:"status"`
	CriadoEm       string `json:"criadoEm"`
}

// todas as taxas da resposta saem em fração (0.0185 = 1,85%).
type OperacaoResponse struct {
	ID                   int64                        `json:"id"`
	NumeroOperacao       string                       `json:"numeroOperacao"`
	ClienteID            string                       `json:"clienteId"`
	TipoPessoa           string                       `json:"tipoPessoa"`
	Status               string                       `json:"status"`
	DefinidoEm           string                       `json:"definidoEm"`
	DataLiberacao        string                       `json:"dataLiberacao"`
	SaldoDevedor         decimal.Decimal              `json:"saldoDevedor"`
	Produto              ProdutoResponse              `json:"produto"`
	CondicoesFinanceiras CondicoesFinanceirasResponse `json:"condicoesFinanceiras"`
	Encargos             EncargosResponse             `json:"encargos"`
	ValorLiberado        decimal.Decimal              `json:"valorLiberado"`
	CET                  CETResponse                  `json:"cet"`
	CriadoEm             string                       `json:"criadoEm"`
}

type ProdutoResponse struct {
	Codigo        string `json:"codigo"`
	Nome          string `json:"nome"`
	ModalidadeBcb string `json:"modalidadeBcb"`
}

type CondicoesFinanceirasResponse struct {
	ValorAprovado      decimal.Decimal `json:"valorAprovado"`
	PrazoMeses         int             `json:"prazoMeses"`
	QuantidadeParcelas int             `json:"quantidadeParcelas"`
	TaxaJuros          decimal.Decimal `json:"taxaJuros"`
	TaxaJurosAnual     decimal.Decimal `json:"taxaJurosAnual"`
	FaixaRisco         string          `json:"faixaRisco"`
	TetoAplicado       bool            `json:"tetoAplicado"`
	SistemaAmortizacao string          `json:"sistemaAmortizacao"`
	PrimeiroVencimento string          `json:"primeiroVencimento"`
}

type EncargosResponse struct {
	IOF            IOFResponse     `json:"iof"`
	TarifaCadastro decimal.Decimal `json:"tarifaCadastro"`
	Total          decimal.Decimal `json:"total"`
}

type IOFResponse struct {
	Diario    decimal.Decimal `json:"diario"`
	Adicional decimal.Decimal `json:"adicional"`
	Total     decimal.Decimal `json:"total"`
}

type CETResponse struct {
	Mensal decimal.Decimal `json:"mensal"`
	Anual  decimal.Decimal `json:"anual"`
}

type ParcelaResponse struct {
	Numero        int             `json:"numero"`
	Vencimento    string          `json:"vencimento"`
	ValorParcela  decimal.Decimal `json:"valorParcela"`
	Amortizacao   decimal.Decimal `json:"amortizacao"`
	Juros         decimal.Decimal `json:"juros"`
	SaldoDevedor  decimal.Decimal `json:"saldoDevedor"`
	Status        string          `json:"status"`
	ValorEmAberto decimal.Decimal `json:"valorEmAberto"`
	DiasEmAtraso  int             `json:"diasEmAtraso"`
}

type OperacaoHandler struct {
	service *service.OperacaoService
}

func NovoOperacaoHandler(service *service.OperacaoService) *OperacaoHandler {
	return &OperacaoHandler{service: service}
}

// POST /operacoes - recebe a operação definida no serviço de juros, com o calendário pronto.
// 201 quando cria, 200 quando o identificador já existia (reenvio). os dados completos ficam nos GETs.
func (h *OperacaoHandler) Criar(c *gin.Context) {
	var body CriarOperacaoRequest

	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"erro": "corpo da requisição inválido"})
		return
	}

	operacao, criada, err := h.service.CriarOperacao(paraNovaOperacao(body))
	if err != nil {
		responderErro(c, err)
		return
	}

	resposta := OperacaoCriadaResponse{
		ID:             operacao.ID,
		NumeroOperacao: operacao.NumeroOperacao,
		Status:         operacao.Status,
		CriadoEm:       operacao.CriadoEm,
	}

	if !criada {
		c.JSON(http.StatusOK, resposta)
		return
	}

	c.Header("Location", "/operacoes/"+operacao.NumeroOperacao)
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

// GET /operacoes/:identificador - situação atual da operação. aceita o numeroOperacao (o normal) ou o id.
func (h *OperacaoHandler) Buscar(c *gin.Context) {
	operacao, err := h.service.BuscarOperacao(c.Param("identificador"), dataReferenciaQuery(c))
	if err != nil {
		responderErro(c, err)
		return
	}

	c.JSON(http.StatusOK, paraOperacaoResponse(operacao))
}

// GET /operacoes/:identificador/parcelas - cronograma com a situação de cada parcela. aceita o numeroOperacao (o normal) ou o id.
func (h *OperacaoHandler) ListarParcelas(c *gin.Context) {
	parcelas, err := h.service.ListarParcelas(c.Param("identificador"), dataReferenciaQuery(c))
	if err != nil {
		responderErro(c, err)
		return
	}

	c.JSON(http.StatusOK, paraParcelasResponse(parcelas))
}

// ponteiro distingue parâmetro omitido de parâmetro enviado vazio.
func dataReferenciaQuery(c *gin.Context) *string {
	valores, presente := c.Request.URL.Query()["dataReferencia"]
	if !presente {
		return nil
	}
	valor := valores[0]
	return &valor
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
		Identificador:        body.Operacao.Identificador,
		Estado:               body.Operacao.Estado,
		DefinidoEm:           body.Operacao.DefinidoEm,
		ClienteID:            body.Cliente.Documento,
		TipoPessoa:           body.Cliente.TipoPessoa,
		ProdutoCodigo:        body.Produto.Codigo,
		ProdutoNome:          body.Produto.Nome,
		ModalidadeBcb:        body.Produto.ModalidadeBcb,
		ValorAprovado:        body.Condicoes.Valor,
		PrazoMeses:           body.Condicoes.PrazoMeses,
		DataLiberacao:        body.Condicoes.DataLiberacao,
		TaxaMensalPercentual: body.Taxa.TaxaFinalMes,
		TaxaAnualPercentual:  body.Taxa.TaxaFinalAno,
		FaixaRisco:           body.Taxa.FaixaRisco,
		TetoAplicado:         body.Taxa.TetoAplicado,
		SistemaAmortizacao:   body.Sistema,
		TotalJuros:           body.Resumo.TotalJuros,
		TotalPago:            body.Resumo.TotalPago,
		IOFDiario:            body.Encargos.IOF.Diario,
		IOFAdicional:         body.Encargos.IOF.Adicional,
		IOFTotal:             body.Encargos.IOF.Total,
		TarifaCadastro:       body.Encargos.TarifaCadastro,
		EncargosTotal:        body.Encargos.Total,
		ValorLiberado:        body.ValorLiberado,
		CETMensalPercentual:  body.CET.MensalPercentual,
		CETAnualPercentual:   body.CET.AnualPercentual,
		Parcelas:             make([]domain.NovaParcela, 0, len(body.Calendario)),
	}

	for _, parcela := range body.Calendario {
		nova.Parcelas = append(nova.Parcelas, domain.NovaParcela{
			Numero:       parcela.Numero,
			Vencimento:   parcela.Vencimento,
			ValorParcela: parcela.Valor,
			Amortizacao:  parcela.Amortizacao,
			Juros:        parcela.Juros,
			SaldoDevedor: parcela.SaldoDevedor,
		})
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
		DefinidoEm:     operacao.DefinidoEm,
		DataLiberacao:  operacao.DataLiberacao,
		SaldoDevedor:   operacao.SaldoDevedor,
		Produto: ProdutoResponse{
			Codigo:        operacao.ProdutoCodigo,
			Nome:          operacao.ProdutoNome,
			ModalidadeBcb: operacao.ModalidadeBcb,
		},
		CondicoesFinanceiras: CondicoesFinanceirasResponse{
			ValorAprovado:      operacao.ValorAprovado,
			PrazoMeses:         operacao.PrazoMeses,
			QuantidadeParcelas: operacao.QuantidadeParcelas,
			TaxaJuros:          operacao.TaxaJuros,
			TaxaJurosAnual:     operacao.TaxaJurosAnual,
			FaixaRisco:         operacao.FaixaRisco,
			TetoAplicado:       operacao.TetoAplicado,
			SistemaAmortizacao: operacao.SistemaAmortizacao,
			PrimeiroVencimento: operacao.PrimeiroVencimento,
		},
		Encargos: EncargosResponse{
			IOF: IOFResponse{
				Diario:    operacao.IOFDiario,
				Adicional: operacao.IOFAdicional,
				Total:     operacao.IOFTotal,
			},
			TarifaCadastro: operacao.TarifaCadastro,
			Total:          operacao.EncargosTotal,
		},
		ValorLiberado: operacao.ValorLiberado,
		CET: CETResponse{
			Mensal: operacao.CETMensal,
			Anual:  operacao.CETAnual,
		},
		CriadoEm: operacao.CriadoEm,
	}
}

func paraParcelasResponse(parcelas []domain.Parcela) []ParcelaResponse {
	resposta := make([]ParcelaResponse, 0, len(parcelas))

	for _, parcela := range parcelas {
		resposta = append(resposta, ParcelaResponse{
			Numero:        parcela.Numero,
			Vencimento:    parcela.Vencimento,
			ValorParcela:  parcela.ValorParcela,
			Amortizacao:   parcela.Amortizacao,
			Juros:         parcela.Juros,
			SaldoDevedor:  parcela.SaldoDevedor,
			Status:        parcela.Status,
			ValorEmAberto: parcela.ValorEmAberto,
			DiasEmAtraso:  parcela.DiasEmAtraso,
		})
	}

	return resposta
}
