package service

import (
	"errors"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/unicsul-finops/financial-operations-service/internal/domain"
)

// formato do identificador do serviço de juros. 4 dígitos ou mais, pra numeração poder crescer.
var formatoNumeroOperacao = regexp.MustCompile(`^OP-\d{4}-\d{4,}$`)

type RepositorioOperacoes interface {
	BuscarPorNumero(string) (domain.Operacao, error)
	CriarComParcelas(domain.NovaOperacao) (domain.Operacao, error)
	Listar() ([]domain.Operacao, error)
	ConsultarSituacao(int64, string) (domain.Operacao, []domain.Parcela, string, error)
}

type OperacaoService struct {
	repo          RepositorioOperacoes
	movimentacoes RepositorioMovimentacoes
	agora         func() time.Time
}

func NovoOperacaoService(repo RepositorioOperacoes) *OperacaoService {
	s := &OperacaoService{repo: repo, agora: time.Now}
	// o repositório de verdade também grava pagamentos; os dublês dos testes de consulta não precisam.
	s.movimentacoes, _ = repo.(RepositorioMovimentacoes)
	return s
}

// devolve true quando a operação foi criada agora e false quando o identificador já existia (reenvio do juros).
func (s *OperacaoService) CriarOperacao(nova domain.NovaOperacao) (domain.Operacao, bool, error) {
	if err := validarNovaOperacao(&nova); err != nil {
		return domain.Operacao{}, false, err
	}

	// reenvio do mesmo identificador devolve o que já foi gravado, em vez de duplicar ou dar erro.
	operacao, err := s.repo.BuscarPorNumero(nova.Identificador)
	if err == nil {
		return operacao, false, nil
	}
	if !errors.Is(err, domain.ErrOperacaoNaoEncontrada) {
		return domain.Operacao{}, false, err
	}

	operacao, err = s.repo.CriarComParcelas(nova)

	// outro envio do mesmo identificador gravou entre a busca e o insert.
	if errors.Is(err, domain.ErrOperacaoJaRegistrada) {
		operacao, err = s.repo.BuscarPorNumero(nova.Identificador)
		return operacao, false, err
	}
	if err != nil {
		return domain.Operacao{}, false, err
	}

	return operacao, true, nil
}

func (s *OperacaoService) ListarOperacoes() ([]domain.Operacao, error) {
	return s.repo.Listar()
}

// operação e parcelas usam a mesma regra e o mesmo retrato dos dados financeiros.
func (s *OperacaoService) BuscarOperacao(identificador string, referencia *string) (domain.Operacao, error) {
	operacao, _, err := s.consultarSituacao(identificador, referencia)
	return operacao, err
}

func (s *OperacaoService) ListarParcelas(identificador string, referencia *string) ([]domain.Parcela, error) {
	_, parcelas, err := s.consultarSituacao(identificador, referencia)
	return parcelas, err
}

func (s *OperacaoService) consultarSituacao(identificador string, referencia *string) (domain.Operacao, []domain.Parcela, error) {
	id, numero, err := identificarOperacao(identificador)
	if err != nil {
		return domain.Operacao{}, nil, err
	}
	data, err := interpretarDataReferencia(referencia, s.agora())
	if err != nil {
		return domain.Operacao{}, nil, err
	}
	operacao, parcelas, ultimaData, err := s.repo.ConsultarSituacao(id, numero)
	if err != nil {
		return domain.Operacao{}, nil, err
	}
	if ultimaData != "" && data.Format(time.DateOnly) < ultimaData {
		return domain.Operacao{}, nil, errors.New("dataReferencia não pode ser anterior à data da última movimentação financeira da operação")
	}
	calculadas := make([]domain.Parcela, 0, len(parcelas))
	for _, parcela := range parcelas {
		calculada, err := calcularSituacaoParcela(parcela, data)
		if err != nil {
			return domain.Operacao{}, nil, err
		}
		calculadas = append(calculadas, calculada)
		if operacao.Status == domain.StatusOperacaoAtiva && calculada.Status == domain.StatusParcelaVencida {
			operacao.Status = domain.StatusOperacaoEmAtraso
		}
	}
	return operacao, calculadas, nil
}

// descobre pelo formato se veio o id ou o numeroOperacao. quando é o número, devolve ele e o id fica zero.
func identificarOperacao(identificador string) (int64, string, error) {
	identificador = strings.TrimSpace(identificador)

	// o id começa em 1, então zero ou número grande demais pro bigint não é id de ninguém.
	if somenteDigitos.MatchString(identificador) {
		id, err := strconv.ParseInt(identificador, 10, 64)
		if err == nil && id > 0 {
			return id, "", nil
		}
	}

	// o número é gravado em maiúsculo, então op-2026-0001 também acha.
	numero := strings.ToUpper(identificador)
	if formatoNumeroOperacao.MatchString(numero) {
		return 0, numero, nil
	}

	// o postgres rejeitaria de qualquer jeito, mas com erro de banco em vez de mensagem clara.
	return 0, "", errors.New("identificador da operação inválido, use o id ou o numeroOperacao (ex.: 1 ou OP-2026-0001)")
}
