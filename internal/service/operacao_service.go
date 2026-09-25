package service

import (
	"database/sql"
	"errors"
	"regexp"
	"strings"
	"time"

	"github.com/shopspring/decimal"
	"github.com/unicsul-finops/financial-operations-service/internal/domain"
	"github.com/unicsul-finops/financial-operations-service/internal/repository"
)

// limite só pra barrar payload absurdo, 35 anos é o prazo de um financiamento imobiliário longo.
const maximoParcelas = 420

var formatoUUID = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`)

// 6 dígitos ou mais, porque a migration 0002 deixa o número crescer depois do 999999.
var formatoNumeroOperacao = regexp.MustCompile(`^OP-\d{4}-\d{6,}$`)

type OperacaoService struct {
	repo *repository.OperacaoRepository
}

func NovoOperacaoService(repo *repository.OperacaoRepository) *OperacaoService {
	return &OperacaoService{repo: repo}
}

func (s *OperacaoService) CriarOperacao(nova domain.NovaOperacao) (domain.Operacao, []domain.Parcela, error) {
	if err := validarNovaOperacao(&nova); err != nil {
		return domain.Operacao{}, nil, err
	}

	// ja foi validado acima, aqui o parse não falha.
	primeiroVencimento, _ := time.Parse("2006-01-02", nova.PrimeiroVencimento)

	parcelas, err := GerarCronograma(nova.ValorAprovado, nova.TaxaJuros.Decimal, nova.QuantidadeParcelas, primeiroVencimento, nova.SistemaAmortizacao)
	if err != nil {
		return domain.Operacao{}, nil, err
	}

	return s.repo.CriarComParcelas(nova, parcelas)
}

func (s *OperacaoService) ListarOperacoes() ([]domain.Operacao, error) {
	return s.repo.Listar()
}

// aceita o id (uuid) ou o numeroOperacao, pra quem só tem o número da tela conseguir consultar.
func (s *OperacaoService) BuscarOperacao(identificador string) (domain.Operacao, error) {
	valor, porNumero, err := identificarOperacao(identificador)
	if err != nil {
		return domain.Operacao{}, err
	}

	if porNumero {
		return s.repo.BuscarPorNumero(valor)
	}

	return s.repo.Buscar(valor)
}

func (s *OperacaoService) ListarParcelas(identificador string) ([]domain.Parcela, error) {
	// além de achar o id quando vier o número, garante 404 pra operação inexistente em vez de lista vazia.
	operacao, err := s.BuscarOperacao(identificador)
	if err != nil {
		return nil, err
	}

	return s.repo.ListarParcelas(operacao.ID)
}

// descobre pelo formato se veio o uuid ou o numeroOperacao. devolve true quando é o número.
func identificarOperacao(identificador string) (string, bool, error) {
	identificador = strings.TrimSpace(identificador)

	if formatoUUID.MatchString(identificador) {
		return identificador, false, nil
	}

	// o número é gravado em maiúsculo, então op-2026-000001 também acha.
	numero := strings.ToUpper(identificador)
	if formatoNumeroOperacao.MatchString(numero) {
		return numero, true, nil
	}

	// o postgres rejeitaria de qualquer jeito, mas com erro de banco em vez de mensagem clara.
	return "", false, errors.New("identificador da operação inválido, use o id ou o numeroOperacao (ex.: OP-2026-000001)")
}

func validarNovaOperacao(nova *domain.NovaOperacao) error {
	nova.ClienteID = strings.TrimSpace(nova.ClienteID)
	nova.TipoPessoa = strings.ToUpper(strings.TrimSpace(nova.TipoPessoa))
	nova.Decisao = strings.ToUpper(strings.TrimSpace(nova.Decisao))
	nova.DataAprovacao = strings.TrimSpace(nova.DataAprovacao)
	nova.SistemaAmortizacao = strings.ToUpper(strings.TrimSpace(nova.SistemaAmortizacao))
	nova.PrimeiroVencimento = strings.TrimSpace(nova.PrimeiroVencimento)

	if nova.ClienteID == "" {
		return errors.New("clienteId não pode ser vazio")
	}
	// o formato do cpf/cnpj já foi validado pelo score, aqui só protege o tamanho da coluna.
	if len(nova.ClienteID) > 20 {
		return errors.New("clienteId deve ter no máximo 20 caracteres")
	}
	if nova.TipoPessoa != "PF" && nova.TipoPessoa != "PJ" {
		return errors.New("tipoPessoa deve ser PF ou PJ")
	}
	// este serviço é a ponta final da esteira, operação reprovada ou em análise não deveria chegar aqui.
	if nova.Decisao != "APROVADO" {
		return errors.New("apenas operações com decisao APROVADO podem ser registradas")
	}

	dataAprovacao, err := time.Parse("2006-01-02", nova.DataAprovacao)
	if err != nil {
		return errors.New("dataAprovacao deve estar no formato YYYY-MM-DD")
	}

	primeiroVencimento, err := time.Parse("2006-01-02", nova.PrimeiroVencimento)
	if err != nil {
		return errors.New("primeiroVencimento deve estar no formato YYYY-MM-DD")
	}
	if !primeiroVencimento.After(dataAprovacao) {
		return errors.New("primeiroVencimento deve ser posterior à dataAprovacao")
	}

	if !nova.ValorAprovado.IsPositive() {
		return errors.New("valorAprovado deve ser maior que zero")
	}
	// fração de centavo não existe em dinheiro, melhor recusar do que arredondar sem avisar.
	if !nova.ValorAprovado.Equal(nova.ValorAprovado.Round(2)) {
		return errors.New("valorAprovado deve ter no máximo 2 casas decimais")
	}
	if nova.QuantidadeParcelas <= 0 {
		return errors.New("quantidadeParcelas deve ser maior que zero")
	}
	if nova.QuantidadeParcelas > maximoParcelas {
		return errors.New("quantidadeParcelas deve ser no máximo 420")
	}
	// o cronograma é mensal, então prazo diferente da quantidade de parcelas é contradição no payload.
	if nova.PrazoMeses != nova.QuantidadeParcelas {
		return errors.New("prazoMeses deve ser igual a quantidadeParcelas")
	}

	if !nova.TaxaJuros.Valid {
		return errors.New("taxaJuros é obrigatória")
	}
	if nova.TaxaJuros.Decimal.IsNegative() {
		return errors.New("taxaJuros deve ser maior ou igual a zero")
	}
	// a taxa vem em decimal (0.0199 = 1,99% a.m.), 1.99 quase certo é alguém mandando em porcentagem.
	if nova.TaxaJuros.Decimal.GreaterThanOrEqual(decimal.NewFromInt(1)) {
		return errors.New("taxaJuros deve ser a taxa mensal em decimal, ex.: 0.0199 para 1,99% ao mês")
	}
	if !nova.TaxaJuros.Decimal.Equal(nova.TaxaJuros.Decimal.Round(6)) {
		return errors.New("taxaJuros deve ter no máximo 6 casas decimais")
	}

	if nova.SistemaAmortizacao != domain.SistemaPrice && nova.SistemaAmortizacao != domain.SistemaSAC {
		return errors.New("sistemaAmortizacao deve ser PRICE ou SAC")
	}

	return validarContextoScore(&nova.ContextoScore)
}

// o contexto do score é opcional e só fica guardado, então valida apenas o que quebraria o banco.
func validarContextoScore(contexto *domain.ContextoScore) error {
	contexto.FaixaRisco = normalizarTexto(contexto.FaixaRisco.String)
	contexto.Modelo = normalizarTexto(contexto.Modelo.String)
	contexto.Motivo = normalizarTexto(contexto.Motivo.String)

	if contexto.ScoreFinal.Valid && contexto.ScoreFinal.Int32 < 0 {
		return errors.New("contextoScore.scoreFinal deve ser maior ou igual a zero")
	}
	if contexto.ProbabilidadeDefault.Valid {
		probabilidade := contexto.ProbabilidadeDefault.Decimal
		if probabilidade.IsNegative() || probabilidade.GreaterThan(decimal.NewFromInt(1)) {
			return errors.New("contextoScore.probabilidadeDefault deve estar entre 0 e 1")
		}
	}
	if len(contexto.FaixaRisco.String) > 30 {
		return errors.New("contextoScore.faixaRisco deve ter no máximo 30 caracteres")
	}
	if len(contexto.Modelo.String) > 50 {
		return errors.New("contextoScore.modelo deve ter no máximo 50 caracteres")
	}

	return nil
}

// texto em branco vira NULL no banco, em vez de gravar string vazia.
func normalizarTexto(texto string) sql.NullString {
	texto = strings.TrimSpace(texto)
	return sql.NullString{String: texto, Valid: texto != ""}
}
