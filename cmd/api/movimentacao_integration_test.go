package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/shopspring/decimal"
	"github.com/unicsul-finops/financial-operations-service/internal/handler"
	"github.com/unicsul-finops/financial-operations-service/internal/repository"
	"github.com/unicsul-finops/financial-operations-service/internal/service"
)

// o exemplo PRICE um ano antes, porque a dataPagamento não pode ser futura. os períodos têm os mesmos dias,
// então os valores são os das RNs.
func operacaoUmAnoAntes(t *testing.T, identificador string) []byte {
	t.Helper()
	exemplo := carregarSpec(t).Paths.Value("/operacoes").Post.RequestBody.Value.Content["application/json"].Examples["price"].Value.Value
	bruto, _ := json.Marshal(exemplo)
	var payload map[string]any
	json.Unmarshal(bruto, &payload)
	payload["operacao"].(map[string]any)["identificador"] = identificador
	payload["condicoes"].(map[string]any)["dataLiberacao"] = "2025-10-30"
	for _, p := range payload["calendario"].([]any) {
		parcela := p.(map[string]any)
		venc := parcela["vencimento"].(string)
		var ano int
		fmt.Sscanf(venc[:4], "%d", &ano)
		parcela["vencimento"] = fmt.Sprintf("%d%s", ano-1, venc[4:])
	}
	bruto, _ = json.Marshal(payload)
	return bruto
}

func TestMovimentacaoIntegracao(t *testing.T) {
	db := bancoDeIntegracao(t)
	anterior := decimal.MarshalJSONWithoutQuotes
	decimal.MarshalJSONWithoutQuotes = true
	t.Cleanup(func() { decimal.MarshalJSONWithoutQuotes = anterior })
	router := novoRouter(handler.NovoOperacaoHandler(service.NovoOperacaoService(repository.NovoOperacaoRepository(db))))
	request := func(method, path, body string) (*httptest.ResponseRecorder, map[string]any) {
		t.Helper()
		w := httptest.NewRecorder()
		r := httptest.NewRequest(method, path, bytes.NewReader([]byte(body)))
		r.Header.Set("Content-Type", "application/json")
		router.ServeHTTP(w, r)
		var resposta map[string]any
		json.Unmarshal(w.Body.Bytes(), &resposta)
		return w, resposta
	}
	esperar := func(w *httptest.ResponseRecorder, status int, trechos ...string) {
		t.Helper()
		if w.Code != status {
			t.Fatalf("status %d, esperado %d: %s", w.Code, status, w.Body)
		}
		for _, trecho := range trechos {
			if !strings.Contains(w.Body.String(), trecho) {
				t.Fatalf("faltou %s em %s", trecho, w.Body)
			}
		}
	}

	// #9: pagamento parcial, reenvio e quitação da parcela 1 (RN-14).
	for _, id := range []string{"OP-2025-0009", "OP-2025-0011"} {
		if w, _ := request("POST", "/operacoes", string(operacaoUmAnoAntes(t, id))); w.Code != 201 {
			t.Fatalf("criação %s: %s", id, w.Body)
		}
	}
	pagamentos := "/operacoes/OP-2025-0009/parcelas/1/pagamentos"
	w, parcial := request("POST", pagamentos, `{"valor": 500, "dataPagamento": "2025-12-10", "identificadorPagamento": "app-1", "origem": "CLIENTE"}`)
	esperar(w, 201, `"distribuicao":{"multa":18.74,"jurosMora":3.12,"juros":185,"amortizacao":293.14}`,
		`"status":"PARCIALMENTE_PAGA","valorEmAberto":458.77`, `"status":"EM_ATRASO","saldoDevedor":9706.86`, `"origem":"CLIENTE"`)
	if w.Header().Get("Location") != "/operacoes/OP-2025-0009/historico" {
		t.Fatalf("Location: %s", w.Header().Get("Location"))
	}
	w, reenvio := request("POST", pagamentos, `{"valor": 500, "dataPagamento": "2025-12-10", "identificadorPagamento": "app-1"}`)
	esperar(w, 200)
	if reenvio["id"] != parcial["id"] || reenvio["registradoEm"] != parcial["registradoEm"] {
		t.Fatalf("reenvio diferente do gravado: %v / %v", reenvio, parcial)
	}
	w, _ = request("GET", "/operacoes/OP-2025-0009/parcelas?dataReferencia=2025-12-10", "")
	esperar(w, 200, `"numero":1,"vencimento":"2025-11-30","valorParcela":936.91,"amortizacao":751.91,"juros":185,"saldoDevedor":9248.09,"status":"VENCIDA","valorEmAberto":458.77`)
	w, _ = request("POST", pagamentos, `{"valor": 10, "dataPagamento": "2025-12-09"}`)
	esperar(w, 400, "dataPagamento não pode ser anterior à data da última movimentação financeira da operação")
	w, _ = request("POST", pagamentos, `{"valor": 460.30, "dataPagamento": "2025-12-20", "identificadorPagamento": "app-2"}`)
	esperar(w, 201, `"jurosMora":1.53`, `"amortizacao":458.77`, `"status":"PAGA"`, `"status":"ATIVA","saldoDevedor":9248.09`)
	w, _ = request("POST", pagamentos, `{"valor": 10, "dataPagamento": "2025-12-20"}`)
	esperar(w, 400, "parcela 1 já está paga")
	w, _ = request("POST", "/operacoes/OP-2025-0009/parcelas/13/pagamentos", `{"valor": 10, "dataPagamento": "2025-12-20"}`)
	esperar(w, 404, "parcela não encontrada")
	w, _ = request("POST", "/operacoes/OP-2025-0009/parcelas/x/pagamentos", `{"valor": 10}`)
	esperar(w, 400, "numero da parcela inválido")
	w, _ = request("POST", pagamentos, `{"valor": `)
	esperar(w, 400, "corpo da requisição inválido")
	w, _ = request("GET", "/operacoes/OP-2025-0009?dataReferencia=2025-12-20", "")
	esperar(w, 200, `"status":"ATIVA"`, `"saldoDevedor":9248.09`)

	var eventos, lancamentos int
	var totalLancado decimal.Decimal
	db.QueryRow(`SELECT COUNT(*) FROM evento e JOIN operacao o ON o.id = e.operacao_id WHERE o.numero_operacao = 'OP-2025-0009'`).Scan(&eventos)
	db.QueryRow(`SELECT COUNT(*), SUM(l.valor) FROM lancamento l JOIN operacao o ON o.id = l.operacao_id WHERE o.numero_operacao = 'OP-2025-0009'`).Scan(&lancamentos, &totalLancado)
	if eventos != 2 || lancamentos != 2 || !totalLancado.Equal(decimal.RequireFromString("960.30")) {
		t.Fatalf("gravou %d eventos e %d lançamentos somando %s", eventos, lancamentos, totalLancado)
	}

	// #11: parcelas 1 e 2 pagas em dia, consulta e liquidação (RN-18).
	base := "/operacoes/OP-2025-0011"
	w, _ = request("POST", base+"/parcelas/1/pagamentos", `{"valor": 936.91, "dataPagamento": "2025-11-30"}`)
	esperar(w, 201)
	w, _ = request("POST", base+"/parcelas/2/pagamentos", `{"valor": 936.91, "dataPagamento": "2025-12-30"}`)
	esperar(w, 201)
	w, _ = request("GET", base+"/liquidacao?dataReferencia=2025-12-30", "")
	esperar(w, 200, `"valorParaQuitar":8482.27,"saldoDevedor":8482.27`, `"vencidas":{"parcelas":[],"total":0}`, `"juros":0`)
	w, _ = request("GET", base+"/liquidacao?dataReferencia=2026-01-15", "")
	esperar(w, 200, `"valorParaQuitar":8558`, `"valorNominal":9369.06,"valorPresente":8558,"desconto":811.06,"amortizacao":8482.27,"juros":75.73`)
	w, _ = request("POST", base+"/liquidacao", `{"valor": 8558.01, "dataPagamento": "2026-01-15"}`)
	esperar(w, 400, "valor deve ser igual ao valor para quitar em 2026-01-15, que é 8558.00")
	w, _ = request("POST", base+"/liquidacao", `{"valor": 8558, "dataPagamento": "2026-01-15", "identificadorPagamento": "liq-1"}`)
	esperar(w, 201, `"vencidas":[]`, `"parcelas":[3,4,5,6,7,8,9,10,11,12]`, `"juros":75.73`, `"status":"LIQUIDADA","saldoDevedor":0`)
	w, _ = request("POST", base+"/liquidacao", `{"valor": 8558, "dataPagamento": "2026-01-15", "identificadorPagamento": "liq-1"}`)
	esperar(w, 200, `"status":"LIQUIDADA"`)
	w, _ = request("POST", base+"/parcelas/3/pagamentos", `{"valor": 10, "dataPagamento": "2026-01-15", "identificadorPagamento": "liq-1"}`)
	esperar(w, 400, "identificadorPagamento já foi usado em outra movimentação da operação")
	w, _ = request("GET", base+"/liquidacao?dataReferencia=2026-01-15", "")
	esperar(w, 400, "a operação já está liquidada")
	w, _ = request("POST", base+"/parcelas/3/pagamentos", `{"valor": 10, "dataPagamento": "2026-01-15"}`)
	esperar(w, 400, "a operação está liquidada e não aceita mais pagamentos")
	w, _ = request("GET", base+"?dataReferencia=2026-01-15", "")
	esperar(w, 200, `"status":"LIQUIDADA"`, `"saldoDevedor":0`)
	w, _ = request("GET", base+"/parcelas?dataReferencia=2026-01-15", "")
	if strings.Contains(w.Body.String(), `"PENDENTE"`) || strings.Count(w.Body.String(), `"status":"PAGA","valorEmAberto":0`) != 12 {
		t.Fatalf("parcelas depois da liquidação: %s", w.Body)
	}
	var semParcela decimal.Decimal
	db.QueryRow(`SELECT l.desconto FROM lancamento l JOIN operacao o ON o.id = l.operacao_id WHERE o.numero_operacao = 'OP-2025-0011' AND l.numero_parcela IS NULL`).Scan(&semParcela)
	if !semParcela.Equal(decimal.RequireFromString("811.06")) {
		t.Fatalf("lançamento da operação com desconto %s", semParcela)
	}
}
