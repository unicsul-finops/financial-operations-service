package repository

import (
	"context"
	"database/sql"

	"github.com/unicsul-finops/financial-operations-service/internal/domain"
)

// o mesmo snapshot evita misturar parcelas anteriores a um pagamento com sua data nova.
func (r *OperacaoRepository) ConsultarSituacao(id int64, numero string) (domain.Operacao, []domain.Parcela, string, error) {
	tx, err := r.db.BeginTx(context.Background(), &sql.TxOptions{Isolation: sql.LevelRepeatableRead, ReadOnly: true})
	if err != nil {
		return domain.Operacao{}, nil, "", err
	}
	defer tx.Rollback()

	filtro, argumento := "id = $1", any(id)
	if numero != "" {
		filtro, argumento = "numero_operacao = $1", numero
	}
	operacao, err := scanOperacaoEncontrada(tx.QueryRow("SELECT "+colunasOperacao+" FROM operacao WHERE "+filtro, argumento))
	if err != nil {
		return domain.Operacao{}, nil, "", err
	}

	var ultimaData string
	err = tx.QueryRow(`SELECT COALESCE(to_char(MAX(data_efeito), 'YYYY-MM-DD'), '')
  FROM evento WHERE operacao_id = $1
  AND tipo IN ('PAGAMENTO', 'AMORTIZACAO_ANTECIPADA', 'LIQUIDACAO')`, operacao.ID).Scan(&ultimaData)
	if err != nil {
		return domain.Operacao{}, nil, "", err
	}

	rows, err := tx.Query(`SELECT `+colunasParcela+`,
  COALESCE(pagos.juros_pagos, 0), COALESCE(pagos.amortizacao_paga, 0), COALESCE(pagos.desconto_concedido, 0)
  FROM parcela
  LEFT JOIN LATERAL (
   SELECT SUM(l.juros) AS juros_pagos, SUM(l.amortizacao) AS amortizacao_paga, SUM(l.desconto) AS desconto_concedido
   FROM lancamento l WHERE l.operacao_id = parcela.operacao_id AND l.numero_parcela = parcela.numero
  ) pagos ON true
  WHERE operacao_id = $1 ORDER BY numero`, operacao.ID)
	if err != nil {
		return domain.Operacao{}, nil, "", err
	}
	defer rows.Close()
	parcelas := []domain.Parcela{}
	for rows.Next() {
		var p domain.Parcela
		err := rows.Scan(&p.ID, &p.OperacaoID, &p.Numero, &p.Vencimento, &p.ValorParcela,
			&p.Amortizacao, &p.Juros, &p.SaldoDevedor, &p.Status, &p.JurosPagos, &p.AmortizacaoPaga, &p.DescontoConcedido)
		if err != nil {
			return domain.Operacao{}, nil, "", err
		}
		parcelas = append(parcelas, p)
	}
	if err := rows.Err(); err != nil {
		return domain.Operacao{}, nil, "", err
	}
	if err := rows.Close(); err != nil {
		return domain.Operacao{}, nil, "", err
	}
	if err := anexarLancamentos(tx, operacao.ID, parcelas); err != nil {
		return domain.Operacao{}, nil, "", err
	}
	if err := tx.Commit(); err != nil {
		return domain.Operacao{}, nil, "", err
	}
	return operacao, parcelas, ultimaData, nil
}

// só os lançamentos por parcela entram nos encargos: o da operação, na liquidação, não tem parcela.
func anexarLancamentos(tx *sql.Tx, operacaoID int64, parcelas []domain.Parcela) error {
	rows, err := tx.Query(`SELECT l.numero_parcela, to_char(e.data_efeito, 'YYYY-MM-DD'),
  l.multa, l.juros_mora, l.juros, l.amortizacao, l.desconto
  FROM lancamento l JOIN evento e ON e.id = l.evento_id
  WHERE l.operacao_id = $1 AND l.numero_parcela IS NOT NULL
  ORDER BY e.data_efeito, l.id`, operacaoID)
	if err != nil {
		return err
	}
	defer rows.Close()
	indice := map[int]int{}
	for i, p := range parcelas {
		indice[p.Numero] = i
	}
	for rows.Next() {
		var l domain.LancamentoParcela
		if err := rows.Scan(&l.NumeroParcela, &l.Data, &l.Multa, &l.JurosMora, &l.Juros, &l.Amortizacao, &l.Desconto); err != nil {
			return err
		}
		if i, ok := indice[l.NumeroParcela]; ok {
			parcelas[i].Lancamentos = append(parcelas[i].Lancamentos, l)
		}
	}
	return rows.Err()
}
