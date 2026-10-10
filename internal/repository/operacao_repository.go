package repository

import (
	"database/sql"
	"errors"

	"github.com/jackc/pgx/v5/pgconn"

	"github.com/unicsul-finops/financial-operations-service/internal/domain"
)

// colunas lidas da operação, repetidas em todo SELECT/RETURNING pra manter a mesma ordem do scan.
const colunasOperacao = `
	id,
	numero_operacao,
	cliente_id,
	tipo_pessoa,
	status,
	to_char(definido_em AT TIME ZONE 'UTC', 'YYYY-MM-DD"T"HH24:MI:SS"Z"'),
	to_char(data_liberacao, 'YYYY-MM-DD'),
	produto_codigo,
	produto_nome,
	modalidade_bcb,
	valor_aprovado,
	prazo_meses,
	quantidade_parcelas,
	taxa_juros,
	taxa_juros_anual,
	faixa_risco,
	teto_aplicado,
	sistema_amortizacao,
	to_char(primeiro_vencimento, 'YYYY-MM-DD'),
	saldo_devedor,
	iof_diario,
	iof_adicional,
	iof_total,
	tarifa_cadastro,
	encargos_total,
	valor_liberado,
	cet_mensal,
	cet_anual,
	to_char(criado_em AT TIME ZONE 'UTC', 'YYYY-MM-DD"T"HH24:MI:SS"Z"')
`

const colunasParcela = `
	id,
	operacao_id,
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
		&operacao.DefinidoEm,
		&operacao.DataLiberacao,
		&operacao.ProdutoCodigo,
		&operacao.ProdutoNome,
		&operacao.ModalidadeBcb,
		&operacao.ValorAprovado,
		&operacao.PrazoMeses,
		&operacao.QuantidadeParcelas,
		&operacao.TaxaJuros,
		&operacao.TaxaJurosAnual,
		&operacao.FaixaRisco,
		&operacao.TetoAplicado,
		&operacao.SistemaAmortizacao,
		&operacao.PrimeiroVencimento,
		&operacao.SaldoDevedor,
		&operacao.IOFDiario,
		&operacao.IOFAdicional,
		&operacao.IOFTotal,
		&operacao.TarifaCadastro,
		&operacao.EncargosTotal,
		&operacao.ValorLiberado,
		&operacao.CETMensal,
		&operacao.CETAnual,
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

// grava a operação e o calendário inteiro juntos, ou nada. nunca pode existir operação sem parcelas.
func (r *OperacaoRepository) CriarComParcelas(nova domain.NovaOperacao) (domain.Operacao, error) {
	tx, err := r.db.Begin()
	if err != nil {
		return domain.Operacao{}, err
	}

	defer tx.Rollback()

	// o saldo devedor começa igual ao valor aprovado, ninguém pagou nada ainda.
	row := tx.QueryRow(`
		INSERT INTO operacao (
			numero_operacao,
			cliente_id,
			tipo_pessoa,
			status,
			definido_em,
			data_liberacao,
			produto_codigo,
			produto_nome,
			modalidade_bcb,
			valor_aprovado,
			prazo_meses,
			quantidade_parcelas,
			taxa_juros,
			taxa_juros_anual,
			faixa_risco,
			teto_aplicado,
			sistema_amortizacao,
			primeiro_vencimento,
			saldo_devedor,
			iof_diario,
			iof_adicional,
			iof_total,
			tarifa_cadastro,
			encargos_total,
			valor_liberado,
			cet_mensal,
			cet_anual
		)
		VALUES (
			$1,
			$2,
			$3,
			$4,
			$5,
			$6::date,
			$7,
			$8,
			$9,
			$10,
			$11,
			$12,
			$13,
			$14,
			$15,
			$16,
			$17,
			$18::date,
			$10,
			$19,
			$20,
			$21,
			$22,
			$23,
			$24,
			$25,
			$26
		)
		RETURNING `+colunasOperacao,
		nova.Identificador,
		nova.ClienteID,
		nova.TipoPessoa,
		domain.StatusOperacaoAtiva,
		nova.DefinidoEmUTC,
		nova.DataLiberacao,
		nova.ProdutoCodigo,
		nova.ProdutoNome,
		nova.ModalidadeBcb,
		nova.ValorAprovado,
		nova.PrazoMeses,
		nova.QuantidadeParcelas,
		nova.TaxaJuros,
		nova.TaxaJurosAnual,
		nova.FaixaRisco,
		nova.TetoAplicado,
		nova.SistemaAmortizacao,
		nova.PrimeiroVencimento,
		nova.IOFDiario,
		nova.IOFAdicional,
		nova.IOFTotal,
		nova.TarifaCadastro,
		nova.EncargosTotal,
		nova.ValorLiberado,
		nova.CETMensal,
		nova.CETAnual,
	)

	operacao, err := scanOperacao(row)
	if numeroJaRegistrado(err) {
		return domain.Operacao{}, domain.ErrOperacaoJaRegistrada
	}
	if err != nil {
		return domain.Operacao{}, err
	}

	for _, novaParcela := range nova.Parcelas {
		_, err := tx.Exec(`
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
				$1,
				$2,
				$3::date,
				$4,
				$5,
				$6,
				$7,
				$8
			)`,
			operacao.ID,
			novaParcela.Numero,
			novaParcela.Vencimento,
			novaParcela.ValorParcela,
			novaParcela.Amortizacao,
			novaParcela.Juros,
			novaParcela.SaldoDevedor,
			domain.StatusParcelaPendente,
		)
		if err != nil {
			return domain.Operacao{}, err
		}
	}

	if err := tx.Commit(); err != nil {
		return domain.Operacao{}, err
	}

	return operacao, nil
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

func (r *OperacaoRepository) Buscar(id int64) (domain.Operacao, error) {
	row := r.db.QueryRow(`
		SELECT `+colunasOperacao+`
		FROM operacao
		WHERE id = $1
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

// violação da UNIQUE do numero_operacao: o mesmo identificador foi gravado por outro envio ao mesmo tempo.
func numeroJaRegistrado(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505" && pgErr.ConstraintName == "operacao_numero_operacao_key"
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

func (r *OperacaoRepository) ListarParcelas(operacaoID int64) ([]domain.Parcela, error) {
	rows, err := r.db.Query(`
		SELECT `+colunasParcela+`
		FROM parcela
		WHERE operacao_id = $1
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
