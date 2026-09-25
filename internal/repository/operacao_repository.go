package repository

import (
	"database/sql"

	"github.com/unicsul-finops/financial-operations-service/internal/domain"
)

// colunas lidas da operação, repetidas em todo SELECT/RETURNING pra manter a mesma ordem do scan.
const colunasOperacao = `
	id::text,
	numero_operacao,
	cliente_id,
	tipo_pessoa,
	status,
	to_char(data_aprovacao, 'YYYY-MM-DD'),
	valor_aprovado,
	prazo_meses,
	quantidade_parcelas,
	taxa_juros,
	sistema_amortizacao,
	to_char(primeiro_vencimento, 'YYYY-MM-DD'),
	saldo_devedor,
	score_final,
	faixa_risco,
	probabilidade_default,
	modelo_score,
	motivo_score,
	to_char(criado_em AT TIME ZONE 'UTC', 'YYYY-MM-DD"T"HH24:MI:SS"Z"')
`

const colunasParcela = `
	id::text,
	operacao_id::text,
	numero,
	to_char(vencimento, 'YYYY-MM-DD'),
	valor_parcela,
	amortizacao,
	juros,
	saldo_devedor,
	status
`

type OperacaoRepository struct {
	db *sql.DB
}

func NovoOperacaoRepository(db *sql.DB) *OperacaoRepository {
	return &OperacaoRepository{db: db}
}

// serve tanto pra *sql.Row quanto pra *sql.Rows.
type scanner interface {
	Scan(dest ...any) error
}

func scanOperacao(linha scanner) (domain.Operacao, error) {
	operacao := domain.Operacao{}

	err := linha.Scan(
		&operacao.ID,
		&operacao.NumeroOperacao,
		&operacao.ClienteID,
		&operacao.TipoPessoa,
		&operacao.Status,
		&operacao.DataAprovacao,
		&operacao.ValorAprovado,
		&operacao.PrazoMeses,
		&operacao.QuantidadeParcelas,
		&operacao.TaxaJuros,
		&operacao.SistemaAmortizacao,
		&operacao.PrimeiroVencimento,
		&operacao.SaldoDevedor,
		&operacao.ContextoScore.ScoreFinal,
		&operacao.ContextoScore.FaixaRisco,
		&operacao.ContextoScore.ProbabilidadeDefault,
		&operacao.ContextoScore.Modelo,
		&operacao.ContextoScore.Motivo,
		&operacao.CriadoEm,
	)

	return operacao, err
}

func scanParcela(linha scanner) (domain.Parcela, error) {
	parcela := domain.Parcela{}

	err := linha.Scan(
		&parcela.ID,
		&parcela.OperacaoID,
		&parcela.Numero,
		&parcela.Vencimento,
		&parcela.ValorParcela,
		&parcela.Amortizacao,
		&parcela.Juros,
		&parcela.SaldoDevedor,
		&parcela.Status,
	)

	return parcela, err
}

// grava a operação e o cronograma inteiro juntos, ou nada. nunca pode existir operação sem parcelas.
func (r *OperacaoRepository) CriarComParcelas(nova domain.NovaOperacao, novas []domain.NovaParcela) (domain.Operacao, []domain.Parcela, error) {
	tx, err := r.db.Begin()
	if err != nil {
		return domain.Operacao{}, nil, err
	}

	defer tx.Rollback()

	// o saldo devedor começa igual ao valor aprovado, ninguém pagou nada ainda.
	// numero_operacao fica de fora porque vem do default da sequence.
	row := tx.QueryRow(`
		INSERT INTO operacao (
			cliente_id,
			tipo_pessoa,
			status,
			data_aprovacao,
			valor_aprovado,
			prazo_meses,
			quantidade_parcelas,
			taxa_juros,
			sistema_amortizacao,
			primeiro_vencimento,
			saldo_devedor,
			score_final,
			faixa_risco,
			probabilidade_default,
			modelo_score,
			motivo_score
		)
		VALUES (
			$1,
			$2,
			$3,
			$4::date,
			$5,
			$6,
			$7,
			$8,
			$9,
			$10::date,
			$5,
			$11,
			$12,
			$13,
			$14,
			$15
		)
		RETURNING `+colunasOperacao,
		nova.ClienteID,
		nova.TipoPessoa,
		domain.StatusOperacaoAtiva,
		nova.DataAprovacao,
		nova.ValorAprovado,
		nova.PrazoMeses,
		nova.QuantidadeParcelas,
		nova.TaxaJuros,
		nova.SistemaAmortizacao,
		nova.PrimeiroVencimento,
		nova.ContextoScore.ScoreFinal,
		nova.ContextoScore.FaixaRisco,
		nova.ContextoScore.ProbabilidadeDefault,
		nova.ContextoScore.Modelo,
		nova.ContextoScore.Motivo,
	)

	operacao, err := scanOperacao(row)
	if err != nil {
		return domain.Operacao{}, nil, err
	}

	parcelas := make([]domain.Parcela, 0, len(novas))

	for _, novaParcela := range novas {
		row := tx.QueryRow(`
			INSERT INTO parcela (
				operacao_id,
				numero,
				vencimento,
				valor_parcela,
				amortizacao,
				juros,
				saldo_devedor,
				status
			)
			VALUES (
				$1::uuid,
				$2,
				$3::date,
				$4,
				$5,
				$6,
				$7,
				$8
			)
			RETURNING `+colunasParcela,
			operacao.ID,
			novaParcela.Numero,
			novaParcela.Vencimento,
			novaParcela.ValorParcela,
			novaParcela.Amortizacao,
			novaParcela.Juros,
			novaParcela.SaldoDevedor,
			domain.StatusParcelaPendente,
		)

		parcela, err := scanParcela(row)
		if err != nil {
			return domain.Operacao{}, nil, err
		}

		parcelas = append(parcelas, parcela)
	}

	if err := tx.Commit(); err != nil {
		return domain.Operacao{}, nil, err
	}

	return operacao, parcelas, nil
}

func (r *OperacaoRepository) Listar() ([]domain.Operacao, error) {
	rows, err := r.db.Query(`
		SELECT ` + colunasOperacao + `
		FROM operacao
		ORDER BY criado_em DESC
	`)
	if err != nil {
		return nil, err
	}

	defer rows.Close()

	operacoes := []domain.Operacao{}

	for rows.Next() {
		operacao, err := scanOperacao(rows)
		if err != nil {
			return nil, err
		}

		operacoes = append(operacoes, operacao)
	}

	return operacoes, rows.Err()
}

func (r *OperacaoRepository) Buscar(id string) (domain.Operacao, error) {
	row := r.db.QueryRow(`
		SELECT `+colunasOperacao+`
		FROM operacao
		WHERE id = $1::uuid
	`, id)

	return scanOperacaoEncontrada(row)
}

// numero_operacao é UNIQUE, então o índice da constraint já deixa essa busca rápida.
func (r *OperacaoRepository) BuscarPorNumero(numero string) (domain.Operacao, error) {
	row := r.db.QueryRow(`
		SELECT `+colunasOperacao+`
		FROM operacao
		WHERE numero_operacao = $1
	`, numero)

	return scanOperacaoEncontrada(row)
}

// traduz o "nenhuma linha" do banco pro erro que o handler transforma em 404.
func scanOperacaoEncontrada(row *sql.Row) (domain.Operacao, error) {
	operacao, err := scanOperacao(row)

	if err == sql.ErrNoRows {
		return domain.Operacao{}, domain.ErrOperacaoNaoEncontrada
	}

	if err != nil {
		return domain.Operacao{}, err
	}

	return operacao, nil
}

func (r *OperacaoRepository) ListarParcelas(operacaoID string) ([]domain.Parcela, error) {
	rows, err := r.db.Query(`
		SELECT `+colunasParcela+`
		FROM parcela
		WHERE operacao_id = $1::uuid
		ORDER BY numero
	`, operacaoID)
	if err != nil {
		return nil, err
	}

	defer rows.Close()

	parcelas := []domain.Parcela{}

	for rows.Next() {
		parcela, err := scanParcela(rows)
		if err != nil {
			return nil, err
		}

		parcelas = append(parcelas, parcela)
	}

	return parcelas, rows.Err()
}
