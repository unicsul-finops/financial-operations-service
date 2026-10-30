package main

import (
	"encoding/json"
	"testing"

	"github.com/shopspring/decimal"
)

// #12: resumo depois do pagamento parcial de 500,00 (RN-14), com a conferência de RN-20 e o valor da liquidação.
func TestResumoIntegracao(t *testing.T) {
	db := bancoDeIntegracao(t)
	chamar, esperar := chamadasDeTeste(t, db)
	esperar(chamar("POST", "/operacoes", string(operacaoUmAnoAntes(t, "OP-2025-0012"))), 201)
	base := "/operacoes/OP-2025-0012"

	esperar(chamar("GET", base+"/resumo?dataReferencia=2025-11-30", ""), 200,
		`"status":"ATIVA"`, `"totalContratado":10000,"totalPrevisto":11242.88,"totalPago":0`, `"saldoDevedor":10000`)
	esperar(chamar("POST", base+"/parcelas/1/pagamentos", `{"valor": 500, "dataPagamento": "2025-12-10"}`), 201)

	w := chamar("GET", base+"/resumo?dataReferencia=2025-12-10", "")
	esperar(w, 200, `"numeroOperacao":"OP-2025-0012","dataReferencia":"2025-12-10","status":"EM_ATRASO","diasEmAtraso":10`,
		`"totalPago":500,"valorAmortizado":293.14,"jurosPagos":185,"encargosPagos":21.86,"descontoAntecipacao":0`,
		`"valorVencido":458.77,"encargosEmAberto":0,"saldoDevedor":9706.86`)
	var resumo struct{ TotalPago, ValorAmortizado, JurosPagos, EncargosPagos, ValorParaQuitar decimal.Decimal }
	json.Unmarshal(w.Body.Bytes(), &resumo)
	if !resumo.TotalPago.Equal(resumo.ValorAmortizado.Add(resumo.JurosPagos).Add(resumo.EncargosPagos)) {
		t.Fatalf("conferência do totalPago: %s", w.Body)
	}

	// o valor para quitar do resumo é o da liquidação na mesma data.
	var liquidacao struct{ ValorParaQuitar decimal.Decimal }
	lw := chamar("GET", base+"/liquidacao?dataReferencia=2025-12-10", "")
	esperar(lw, 200)
	json.Unmarshal(lw.Body.Bytes(), &liquidacao)
	if resumo.ValorParaQuitar.IsZero() || !resumo.ValorParaQuitar.Equal(liquidacao.ValorParaQuitar) {
		t.Fatalf("valorParaQuitar %s, liquidação %s", resumo.ValorParaQuitar, liquidacao.ValorParaQuitar)
	}

	esperar(chamar("GET", base+"/resumo?dataReferencia=2025-12-09", ""), 400, "dataReferencia não pode ser anterior à data da última movimentação financeira da operação")
	esperar(chamar("GET", base+"/resumo?dataReferencia=2025-02-30", ""), 400, "dataReferencia deve estar no formato YYYY-MM-DD")
	esperar(chamar("GET", "/operacoes/invalida/resumo", ""), 400, "identificador da operação inválido")
	esperar(chamar("GET", "/operacoes/999999/resumo", ""), 404, "operação não encontrada")
}
