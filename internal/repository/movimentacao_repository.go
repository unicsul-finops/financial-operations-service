package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"

	"github.com/unicsul-finops/financial-operations-service/internal/domain"
)

func (r *OperacaoRepository) ConsultarCarteira(id int64, numero string) (domain.Carteira, error) {
	tx, err := r.db.BeginTx(context.Background(), &sql.TxOptions{Isolation: sql.LevelRepeatableRead, ReadOnly: true})
	if err != nil {
		return domain.Carteira{}, err
	}
	defer tx.Rollback()

	carteira, err := carregarCarteira(tx, id, numero, false)
	if err != nil {
		return domain.Carteira{}, err
	}
	return carteira, tx.Commit()
}

// evento, lançamentos, status das parcelas e saldo numa transação só (ADR-012).
// o FOR UPDATE na operação faz dois pagamentos simultâneos rodarem um depois do outro.
func (r *OperacaoRepository) Movimentar(id int64, numero, identificadorExterno string, decidir func(domain.Carteira) (domain.NovoEvento, error)) (domain.EventoGravado, bool, error) {
	tx, err := r.db.Begin()
	if err != nil {
		return domain.EventoGravado{}, false, err
	}
	defer tx.Rollback()

	carteira, err := carregarCarteira(tx, id, numero, true)
	if err != nil {
		return domain.EventoGravado{}, false, err
	}
	operacaoID := carteira.Operacao.ID

	// reenvio: devolve o que já foi gravado, antes de qualquer regra.
	if identificadorExterno != "" {
		var gravado domain.EventoGravado
		err := tx.QueryRow(`SELECT e.id, e.tipo,
  COALESCE((SELECT MIN(l.numero_parcela) FROM lancamento l WHERE l.evento_id = e.id), 0),
  to_char(e.registrado_em AT TIME ZONE 'UTC', 'YYYY-MM-DD"T"HH24:MI:SS"Z"'), e.detalhe
  FROM evento e WHERE e.operacao_id = $1 AND e.identificador_externo = $2`, operacaoID, identificadorExterno).
			Scan(&gravado.ID, &gravado.Tipo, &gravado.NumeroParcela, &gravado.RegistradoEm, &gravado.Detalhe)
		if err == nil {
			return gravado, false, nil
		}
		if !errors.Is(err, sql.ErrNoRows) {
			return domain.EventoGravado{}, false, err
		}
	}

	novo, err := decidir(carteira)
	if err != nil {
		return domain.EventoGravado{}, false, err
	}
	detalhe, err := json.Marshal(novo.Detalhe)
	if err != nil {
		return domain.EventoGravado{}, false, err
	}

	gravado := domain.EventoGravado{Tipo: novo.Tipo, Detalhe: detalhe}
	err = tx.QueryRow(`INSERT INTO evento (operacao_id, tipo, data_efeito, valor, identificador_externo, origem, detalhe)
  VALUES ($1, $2, $3::date, $4, NULLIF($5, ''), NULLIF($6, ''), $7::jsonb)
  RETURNING id, to_char(registrado_em AT TIME ZONE 'UTC', 'YYYY-MM-DD"T"HH24:MI:SS"Z"')`,
		operacaoID, novo.Tipo, novo.Data, novo.Valor, novo.IdentificadorExterno, novo.Origem, string(detalhe)).
		Scan(&gravado.ID, &gravado.RegistradoEm)
	if err != nil {
		return domain.EventoGravado{}, false, err
	}

	for _, l := range novo.Lancamentos {
		_, err := tx.Exec(`INSERT INTO lancamento (evento_id, operacao_id, numero_parcela, multa, juros_mora, juros, amortizacao, desconto, valor)
  VALUES ($1, $2, NULLIF($3, 0), $4, $5, $6, $7, $8, $9)`,
			gravado.ID, operacaoID, l.NumeroParcela, l.Multa, l.JurosMora, l.Juros, l.Amortizacao, l.Desconto, l.Total())
		if err != nil {
			return domain.EventoGravado{}, false, err
		}
	}
	for numeroParcela, status := range novo.StatusParcelas {
		if _, err := tx.Exec(`UPDATE parcela SET status = $1 WHERE operacao_id = $2 AND numero = $3`, status, operacaoID, numeroParcela); err != nil {
			return domain.EventoGravado{}, false, err
		}
	}

	for _, p := range novo.ParcelasRecalculadas {
		if _, err := tx.Exec(`UPDATE parcela SET juros = $1, amortizacao = $2, saldo_devedor = $3 WHERE operacao_id = $4 AND numero = $5`,
			p.Depois.Juros, p.Depois.Amortizacao, p.Depois.SaldoDevedor, operacaoID, p.Numero); err != nil {
			return domain.EventoGravado{}, false, err
		}
	}

	status := carteira.Operacao.Status
	if novo.Liquidar {
		status = domain.StatusOperacaoLiquidada
	}
	if _, err := tx.Exec(`UPDATE operacao SET saldo_devedor = $1, status = $2 WHERE id = $3`, novo.SaldoDevedor, status, operacaoID); err != nil {
		return domain.EventoGravado{}, false, err
	}
	// RN-19: só marca o encerramento no histórico; o dinheiro está no evento PAGAMENTO.
	if novo.EventoEncerramento {
		if _, err := tx.Exec(`INSERT INTO evento (operacao_id, tipo, data_efeito) VALUES ($1, $2, $3::date)`, operacaoID, domain.EventoLiquidacao, novo.Data); err != nil {
			return domain.EventoGravado{}, false, err
		}
	}

	if err := tx.Commit(); err != nil {
		return domain.EventoGravado{}, false, err
	}
	return gravado, true, nil
}

func carregarCarteira(tx *sql.Tx, id int64, numero string, travar bool) (domain.Carteira, error) {
	filtro, argumento := "id = $1", any(id)
	if numero != "" {
		filtro, argumento = "numero_operacao = $1", numero
	}
	trava := ""
	if travar {
		trava = " FOR UPDATE"
	}
	operacao, err := scanOperacaoEncontrada(tx.QueryRow("SELECT "+colunasOperacao+" FROM operacao WHERE "+filtro+trava, argumento))
	if err != nil {
		return domain.Carteira{}, err
	}
	carteira := domain.Carteira{Operacao: operacao, Parcelas: []domain.Parcela{}, Lancamentos: []domain.LancamentoParcela{}}

	err = tx.QueryRow(`SELECT COALESCE(to_char(MAX(data_efeito), 'YYYY-MM-DD'), '')
  FROM evento WHERE operacao_id = $1
  AND tipo IN ('PAGAMENTO', 'AMORTIZACAO_ANTECIPADA', 'LIQUIDACAO')`, operacao.ID).Scan(&carteira.UltimaMovimentacao)
	if err != nil {
		return domain.Carteira{}, err
	}

	err = tx.QueryRow(`SELECT COALESCE(SUM(multa), 0), COALESCE(SUM(juros_mora), 0), COALESCE(SUM(juros), 0),
  COALESCE(SUM(amortizacao), 0), COALESCE(SUM(desconto), 0)
  FROM lancamento WHERE operacao_id = $1 AND numero_parcela IS NULL`, operacao.ID).
		Scan(&carteira.LancamentoOperacao.Multa, &carteira.LancamentoOperacao.JurosMora, &carteira.LancamentoOperacao.Juros,
			&carteira.LancamentoOperacao.Amortizacao, &carteira.LancamentoOperacao.Desconto)
	if err != nil {
		return domain.Carteira{}, err
	}

	rows, err := tx.Query(`SELECT `+colunasParcela+` FROM parcela WHERE operacao_id = $1 ORDER BY numero`, operacao.ID)
	if err != nil {
		return domain.Carteira{}, err
	}
	for rows.Next() {
		parcela, err := scanParcela(rows)
		if err != nil {
			rows.Close()
			return domain.Carteira{}, err
		}
		carteira.Parcelas = append(carteira.Parcelas, parcela)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return domain.Carteira{}, err
	}

	// só os lançamentos por parcela: o da operação, na liquidação, não entra em encargo nenhum.
	rows, err = tx.Query(`SELECT l.numero_parcela, to_char(e.data_efeito, 'YYYY-MM-DD'),
  l.multa, l.juros_mora, l.juros, l.amortizacao, l.desconto
  FROM lancamento l JOIN evento e ON e.id = l.evento_id
  WHERE l.operacao_id = $1 AND l.numero_parcela IS NOT NULL
  ORDER BY e.data_efeito, l.id`, operacao.ID)
	if err != nil {
		return domain.Carteira{}, err
	}
	defer rows.Close()
	for rows.Next() {
		var l domain.LancamentoParcela
		if err := rows.Scan(&l.NumeroParcela, &l.Data, &l.Multa, &l.JurosMora, &l.Juros, &l.Amortizacao, &l.Desconto); err != nil {
			return domain.Carteira{}, err
		}
		carteira.Lancamentos = append(carteira.Lancamentos, l)
	}
	return carteira, rows.Err()
}

// eventos e lançamentos da operação no mesmo retrato da carteira, pra situação calculada não destoar do que foi gravado.
func (r *OperacaoRepository) ConsultarHistorico(id int64, numero string) (domain.Historico, error) {
	tx, err := r.db.BeginTx(context.Background(), &sql.TxOptions{Isolation: sql.LevelRepeatableRead, ReadOnly: true})
	if err != nil {
		return domain.Historico{}, err
	}
	defer tx.Rollback()

	carteira, err := carregarCarteira(tx, id, numero, false)
	if err != nil {
		return domain.Historico{}, err
	}
	historico := domain.Historico{Carteira: carteira, Eventos: []domain.EventoHistorico{}}
	operacaoID := carteira.Operacao.ID

	rows, err := tx.Query(`SELECT e.id, e.tipo, to_char(e.data_efeito, 'YYYY-MM-DD'),
  to_char(e.registrado_em AT TIME ZONE 'UTC', 'YYYY-MM-DD"T"HH24:MI:SS"Z"'), e.valor,
  COALESCE(e.identificador_externo, ''), COALESCE(e.origem, '')
  FROM evento e WHERE e.operacao_id = $1 ORDER BY e.data_efeito, e.id`, operacaoID)
	if err != nil {
		return domain.Historico{}, err
	}
	indice := map[int64]int{}
	for rows.Next() {
		var e domain.EventoHistorico
		if err := rows.Scan(&e.ID, &e.Tipo, &e.DataEfeito, &e.RegistradoEm, &e.Valor, &e.IdentificadorPagamento, &e.Origem); err != nil {
			rows.Close()
			return domain.Historico{}, err
		}
		e.Lancamentos = []domain.LancamentoEvento{}
		indice[e.ID] = len(historico.Eventos)
		historico.Eventos = append(historico.Eventos, e)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return domain.Historico{}, err
	}

	rows, err = tx.Query(`SELECT evento_id, COALESCE(numero_parcela, 0), multa, juros_mora, juros, amortizacao, desconto
  FROM lancamento WHERE operacao_id = $1 ORDER BY id`, operacaoID)
	if err != nil {
		return domain.Historico{}, err
	}
	defer rows.Close()
	for rows.Next() {
		var eventoID int64
		var l domain.LancamentoEvento
		if err := rows.Scan(&eventoID, &l.NumeroParcela, &l.Multa, &l.JurosMora, &l.Juros, &l.Amortizacao, &l.Desconto); err != nil {
			return domain.Historico{}, err
		}
		i := indice[eventoID]
		historico.Eventos[i].Lancamentos = append(historico.Eventos[i].Lancamentos, l)
	}
	if err := rows.Err(); err != nil {
		return domain.Historico{}, err
	}
	return historico, tx.Commit()
}
