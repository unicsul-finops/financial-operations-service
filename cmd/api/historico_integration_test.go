package main

import (
	"encoding/json"
	"strings"
	"testing"
)

// #13: o registro da operação, um pagamento e o atraso calculado, em ordem cronológica (RN-21).
func TestHistoricoIntegracao(t *testing.T) {
	db := bancoDeIntegracao(t)
	chamar, esperar := chamadasDeTeste(t, db)
	esperar(chamar("POST", "/operacoes", string(operacaoUmAnoAntes(t, "OP-2025-0013"))), 201)
	// reenvio do juros não grava um segundo registro.
	esperar(chamar("POST", "/operacoes", string(operacaoUmAnoAntes(t, "OP-2025-0013"))), 200)
	base := "/operacoes/OP-2025-0013"

	var eventos int
	db.QueryRow(`SELECT COUNT(*) FROM evento e JOIN operacao o ON o.id = e.operacao_id WHERE o.numero_operacao = 'OP-2025-0013' AND e.tipo = 'OPERACAO_REGISTRADA'`).Scan(&eventos)
	if eventos != 1 {
		t.Fatalf("eventos de registro: %d", eventos)
	}

	esperar(chamar("GET", base+"/historico?dataReferencia=2025-11-30", ""), 200,
		`"tipo":"OPERACAO_REGISTRADA","origem":"REGISTRADO","dataEfeito":"2025-10-30"`, `"lancamentos":[]`)

	esperar(chamar("POST", base+"/parcelas/1/pagamentos", `{"valor": 500, "dataPagamento": "2025-12-10", "identificadorPagamento": "app-13", "origem": "CLIENTE"}`), 201)

	w := chamar("GET", base+"/historico?dataReferencia=2025-12-10", "")
	esperar(w, 200,
		`"tipo":"PAGAMENTO","origem":"REGISTRADO","dataEfeito":"2025-12-10"`, `"valor":500,"identificadorPagamento":"app-13","origemPagamento":"CLIENTE"`,
		`"lancamentos":[{"numeroParcela":1,"multa":18.74,"jurosMora":3.12,"juros":185,"amortizacao":293.14,"desconto":0}]`,
		`"tipo":"PARCELA_VENCIDA","origem":"CALCULADO","dataEfeito":"2025-12-01","numeroParcela":1`, `"descricao":"parcela 1 vencida em 2025-12-01"`)
	var itens []struct {
		Tipo, Origem, DataEfeito string
	}
	if err := json.Unmarshal(w.Body.Bytes(), &itens); err != nil {
		t.Fatal(err)
	}
	ordem := []string{"OPERACAO_REGISTRADA", "PARCELA_VENCIDA", "PAGAMENTO"}
	if len(itens) != len(ordem) {
		t.Fatalf("itens: %s", w.Body)
	}
	for i, tipo := range ordem {
		if itens[i].Tipo != tipo {
			t.Fatalf("ordem: %s", w.Body)
		}
	}

	// o atraso calculado depende da data; o gravado não muda.
	esperar(chamar("GET", base+"/historico?dataReferencia=2026-03-01", ""), 200, `"tipo":"OPERACAO_INADIMPLENTE","origem":"CALCULADO","dataEfeito":"2026-03-01"`)
	esperar(chamar("GET", base+"/historico?dataReferencia=2026-02-28", ""), 200, `"tipo":"PARCELA_VENCIDA"`)
	if w := chamar("GET", base+"/historico?dataReferencia=2026-02-28", ""); strings.Contains(w.Body.String(), "OPERACAO_INADIMPLENTE") {
		t.Fatalf("inadimplente com 90 dias: %s", w.Body)
	}

	esperar(chamar("GET", base+"/historico?dataReferencia=2025-12-09", ""), 400, "dataReferencia não pode ser anterior à data da última movimentação financeira da operação")
	esperar(chamar("GET", base+"/historico?dataReferencia=2025-02-30", ""), 400, "dataReferencia deve estar no formato YYYY-MM-DD")
	esperar(chamar("GET", "/operacoes/invalida/historico", ""), 400, "identificador da operação inválido")
	esperar(chamar("GET", "/operacoes/999999/historico", ""), 404, "operação não encontrada")

	// evento gravado nunca é alterado: a consulta repetida devolve o mesmo.
	primeiro := chamar("GET", base+"/historico?dataReferencia=2025-12-10", "")
	segundo := chamar("GET", base+"/historico?dataReferencia=2025-12-10", "")
	if primeiro.Body.String() != segundo.Body.String() {
		t.Fatal("o histórico mudou entre duas consultas")
	}
}
