package service

import (
	"errors"
	"regexp"
	"strconv"
	"strings"

	"github.com/unicsul-finops/financial-operations-service/internal/domain"
	"github.com/unicsul-finops/financial-operations-service/internal/repository"
)

// formato do identificador do serviço de juros. 4 dígitos ou mais, pra numeração poder crescer.
var formatoNumeroOperacao = regexp.MustCompile(`^OP-\d{4}-\d{4,}$`)

type OperacaoService struct {
	repo *repository.OperacaoRepository
}

func NovoOperacaoService(repo *repository.OperacaoRepository) *OperacaoService {
	return &OperacaoService{repo: repo}
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

// aceita o numeroOperacao, que é como a esteira conhece a operação, ou o id interno.
func (s *OperacaoService) BuscarOperacao(identificador string) (domain.Operacao, error) {
	id, numero, err := identificarOperacao(identificador)
	if err != nil {
		return domain.Operacao{}, err
	}

	if numero != "" {
		return s.repo.BuscarPorNumero(numero)
	}

	return s.repo.Buscar(id)
}

func (s *OperacaoService) ListarParcelas(identificador string) ([]domain.Parcela, error) {
	// além de achar o id quando vier o número, garante 404 pra operação inexistente em vez de lista vazia.
	operacao, err := s.BuscarOperacao(identificador)
	if err != nil {
		return nil, err
	}

	return s.repo.ListarParcelas(operacao.ID)
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
