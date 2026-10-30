package main

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/shopspring/decimal"
	"github.com/unicsul-finops/financial-operations-service/internal/handler"
	"github.com/unicsul-finops/financial-operations-service/internal/repository"
	"github.com/unicsul-finops/financial-operations-service/internal/service"
)

// chamadas HTTP contra o router de verdade, com as decimais como número no JSON.
func chamadasDeTeste(t *testing.T, db *sql.DB) (func(metodo, caminho, corpo string) *httptest.ResponseRecorder, func(*httptest.ResponseRecorder, int, ...string)) {
	t.Helper()
	anterior := decimal.MarshalJSONWithoutQuotes
	decimal.MarshalJSONWithoutQuotes = true
	t.Cleanup(func() { decimal.MarshalJSONWithoutQuotes = anterior })
	router := novoRouter(handler.NovoOperacaoHandler(service.NovoOperacaoService(repository.NovoOperacaoRepository(db))), nil)
	chamar := func(metodo, caminho, corpo string) *httptest.ResponseRecorder {
		t.Helper()
		w := httptest.NewRecorder()
		r := httptest.NewRequest(metodo, caminho, bytes.NewReader([]byte(corpo)))
		r.Header.Set("Content-Type", "application/json")
		router.ServeHTTP(w, r)
		return w
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
	return chamar, esperar
}

// #10: parcelas 1 e 2 pagas em dia e antecipação das 2 últimas (RN-16 e RN-17).
func TestAmortizacaoIntegracao(t *testing.T) {
	db := bancoDeIntegracao(t)
	chamar, esperar := chamadasDeTeste(t, db)
	esperar(chamar("POST", "/operacoes", string(operacaoUmAnoAntes(t, "OP-2025-0010"))), 201)
	base := "/operacoes/OP-2025-0010"

	// parcela vencida em aberto: recusa antes de qualquer valor.
	esperar(chamar("GET", base+"/amortizacoes/simulacao?quantidadeParcelas=2&dataReferencia=2025-12-01", ""), 400,
		"a parcela 1 está vencida e em aberto, pague o atraso antes de antecipar parcelas")
	esperar(chamar("POST", base+"/parcelas/1/pagamentos", `{"valor": 936.91, "dataPagamento": "2025-11-30"}`), 201)
	esperar(chamar("POST", base+"/parcelas/2/pagamentos", `{"valor": 936.91, "dataPagamento": "2025-12-30"}`), 201)

	esperar(chamar("GET", base+"/amortizacoes/simulacao?quantidadeParcelas=2&dataReferencia=2025-12-30", ""), 200,
		`"quantidadeParcelas":2`, `"valorPresente":794.42`, `"valorPresente":779.95`,
		`"totais":{"valorParcelas":1873.78,"desconto":299.41,"valorPresente":1574.37,"amortizacao":1574.37,"juros":0,"valorAPagar":1574.37}`,
		`"saldoDevedorAntes":8482.27,"saldoDevedorApos":6907.9,"quantidadeParcelasRestantes":8`)
	esperar(chamar("GET", base+"/amortizacoes/simulacao?quantidadeParcelas=2&dataReferencia=2026-01-15", ""), 200,
		`"valorPresente":801.51`, `"valorPresente":786.92`,
		`"totais":{"valorParcelas":1873.78,"desconto":285.35,"valorPresente":1588.43,"amortizacao":1574.38,"juros":14.05,"valorAPagar":1588.43}`,
		`"saldoDevedorApos":6907.89`)

	for _, caso := range []struct{ query, erro string }{
		{"", "quantidadeParcelas é obrigatório"},
		{"?quantidadeParcelas=0", "quantidadeParcelas deve ser um inteiro maior que zero"},
		{"?quantidadeParcelas=1.5", "quantidadeParcelas deve ser um inteiro maior que zero"},
		{"?quantidadeParcelas=2&dataReferencia=2025-12-29", "dataReferencia não pode ser anterior à data do último pagamento (2025-12-30)"},
		{"?quantidadeParcelas=2&dataReferencia=2025-02-30", "dataReferencia deve estar no formato YYYY-MM-DD"},
		{"?quantidadeParcelas=10&dataReferencia=2025-12-30", "quantidadeParcelas cobre todas as 10 parcelas em aberto, para quitar tudo use a liquidação da operação"},
		{"?quantidadeParcelas=11&dataReferencia=2025-12-30", "quantidadeParcelas não pode ser maior que as 10 parcelas em aberto"},
	} {
		esperar(chamar("GET", base+"/amortizacoes/simulacao"+caso.query, ""), 400, caso.erro)
	}
	esperar(chamar("GET", "/operacoes/999999/amortizacoes/simulacao?quantidadeParcelas=2", ""), 404)

	// a simulação não grava nada.
	var eventos int
	db.QueryRow(`SELECT COUNT(*) FROM evento e JOIN operacao o ON o.id = e.operacao_id WHERE o.numero_operacao = 'OP-2025-0010'`).Scan(&eventos)
	// o registro da operação e os dois pagamentos.
	if eventos != 3 {
		t.Fatalf("a simulação gravou eventos: %d", eventos)
	}

	pedido := `{"quantidadeParcelas": 2, "dataPagamento": "2026-01-15", "identificadorPagamento": "PAG-1"}`
	w := chamar("POST", base+"/amortizacoes", pedido)
	esperar(w, 201, `"tipo":"AMORTIZACAO_ANTECIPADA"`, `"identificadorPagamento":"PAG-1"`, `"dataPagamento":"2026-01-15"`,
		`"juros":14.05,"valorAPagar":1588.43`, `"saldoDevedorAntes":8482.27,"saldoDevedorApos":6907.89,"quantidadeParcelasRestantes":8`)
	if w.Header().Get("Location") != base {
		t.Fatalf("Location: %s", w.Header().Get("Location"))
	}
	reenvio := chamar("POST", base+"/amortizacoes", pedido)
	esperar(reenvio, 200, `"tipo":"AMORTIZACAO_ANTECIPADA"`, `"saldoDevedorApos":6907.89`)
	if reenvio.Body.String() != w.Body.String() || strings.Contains(reenvio.Body.String(), "parcelasRecalculadas") {
		t.Fatalf("reenvio diferente do gravado: %s / %s", reenvio.Body, w.Body)
	}
	esperar(chamar("POST", base+"/parcelas/3/pagamentos", `{"valor": 10, "dataPagamento": "2026-01-15", "identificadorPagamento": "PAG-1"}`), 400,
		"identificadorPagamento já foi usado em outra movimentação da operação")
	esperar(chamar("POST", base+"/amortizacoes", `{"quantidadeParcelas": 1, "dataPagamento": "2026-01-14"}`), 400,
		"dataPagamento não pode ser anterior à data do último pagamento (2026-01-15)")
	esperar(chamar("POST", base+"/amortizacoes", `{"dataPagamento": "2026-01-15"}`), 400, "quantidadeParcelas é obrigatório")
	esperar(chamar("POST", base+"/amortizacoes", `{"quantidadeParcelas": 1.5}`), 400, "quantidadeParcelas deve ser um inteiro maior que zero")
	esperar(chamar("POST", base+"/amortizacoes", `{"quantidadeParcelas": `), 400, "corpo da requisição inválido")

	// parcelas 11 e 12 pagas; 3 a 10 com a repartição nova, valor de 936,91 e saldo zerado na 10.
	w = chamar("GET", base+"/parcelas?dataReferencia=2026-01-15", "")
	esperar(w, 200, `"numero":11,"vencimento":"2026-09-30","valorParcela":936.91`, `"status":"PAGA","valorEmAberto":0`)
	var parcelas []struct {
		Numero                    int
		ValorParcela, Amortizacao decimal.Decimal
		Juros, SaldoDevedor       decimal.Decimal
		Status                    string
	}
	if err := json.Unmarshal(w.Body.Bytes(), &parcelas); err != nil {
		t.Fatal(err)
	}
	for _, p := range parcelas[2:10] {
		if p.Status != "PENDENTE" || !p.Juros.Add(p.Amortizacao).Equal(decimal.RequireFromString("936.91")) {
			t.Fatalf("parcela %d: %+v", p.Numero, p)
		}
	}
	if !parcelas[9].SaldoDevedor.IsZero() || !parcelas[2].Juros.Equal(decimal.RequireFromString("127.8")) {
		t.Fatalf("repartição: parcela 3 %+v, parcela 10 %+v", parcelas[2], parcelas[9])
	}

	var saldo, detalhe string
	var valor, desconto decimal.Decimal
	db.QueryRow(`SELECT o.saldo_devedor::text FROM operacao o WHERE o.numero_operacao = 'OP-2025-0010'`).Scan(&saldo)
	db.QueryRow(`SELECT e.valor, e.detalhe::text FROM evento e JOIN operacao o ON o.id = e.operacao_id WHERE o.numero_operacao = 'OP-2025-0010' AND e.tipo = 'AMORTIZACAO_ANTECIPADA'`).Scan(&valor, &detalhe)
	db.QueryRow(`SELECT SUM(l.desconto) FROM lancamento l JOIN evento e ON e.id = l.evento_id WHERE e.tipo = 'AMORTIZACAO_ANTECIPADA'`).Scan(&desconto)
	if saldo != "6907.89" || !valor.Equal(decimal.RequireFromString("1588.43")) || !desconto.Equal(decimal.RequireFromString("285.35")) || !strings.Contains(detalhe, "parcelasRecalculadas") {
		t.Fatalf("gravou saldo %s, valor %s, desconto %s, detalhe %s", saldo, valor, desconto, detalhe)
	}

	// depois da antecipação a liquidação fecha só o que sobrou.
	esperar(chamar("GET", base+"/liquidacao?dataReferencia=2026-01-15", ""), 200, `"saldoDevedor":6907.89`)
}
