package main

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/shopspring/decimal"
	"github.com/unicsul-finops/financial-operations-service/internal/handler"
	"github.com/unicsul-finops/financial-operations-service/internal/repository"
	"github.com/unicsul-finops/financial-operations-service/internal/service"
)

// usa schema exclusivo: nunca altera as operações do banco de desenvolvimento.
func bancoDeIntegracao(t *testing.T) *sql.DB {
	t.Helper()
	url := os.Getenv("FINOPS_TEST_DATABASE_URL")
	if url == "" {
		t.Skip("defina FINOPS_TEST_DATABASE_URL para testar com PostgreSQL")
	}
	cfg, err := pgx.ParseConfig(url)
	if err != nil {
		t.Fatal("configuração de teste inválida")
	}
	cfg.ConnectTimeout = 5 * time.Second
	admin := stdlib.OpenDB(*cfg)
	if err := admin.Ping(); err != nil {
		admin.Close()
		t.Fatal("PostgreSQL de teste indisponível")
	}
	schema := fmt.Sprintf("finops_test_%d", time.Now().UnixNano())
	if _, err := admin.Exec("CREATE SCHEMA " + schema); err != nil {
		admin.Close()
		t.Fatal(err)
	}
	t.Cleanup(func() {
		defer admin.Close()
		if _, err := admin.Exec("DROP SCHEMA " + schema + " CASCADE"); err != nil {
			t.Error(err)
		}
	})
	cfg.RuntimeParams["search_path"] = schema
	db := stdlib.OpenDB(*cfg)
	t.Cleanup(func() { db.Close() })
	for _, path := range []string{"../../migrations/0001_create_foundations.sql", "../../migrations/0002_fase_2_pagamentos_e_eventos.sql"} {
		source, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := db.Exec(string(source)); err != nil {
			t.Fatal(err)
		}
	}
	return db
}

func TestAtrasoIntegracao(t *testing.T) {
	db := bancoDeIntegracao(t)
	anterior := decimal.MarshalJSONWithoutQuotes
	decimal.MarshalJSONWithoutQuotes = true
	t.Cleanup(func() { decimal.MarshalJSONWithoutQuotes = anterior })
	router := novoRouter(handler.NovoOperacaoHandler(service.NovoOperacaoService(repository.NovoOperacaoRepository(db))), nil)
	request := func(method, path string, body []byte) *httptest.ResponseRecorder {
		t.Helper()
		w := httptest.NewRecorder()
		r := httptest.NewRequest(method, path, bytes.NewReader(body))
		r.Header.Set("Content-Type", "application/json")
		router.ServeHTTP(w, r)
		return w
	}
	payload, err := json.Marshal(carregarSpec(t).Paths.Value("/operacoes").Post.RequestBody.Value.Content["application/json"].Examples["price"].Value.Value)
	if err != nil {
		t.Fatal(err)
	}
	if w := request("POST", "/operacoes", payload); w.Code != 201 {
		t.Fatalf("criação: %d %s", w.Code, w.Body)
	}
	base := "/operacoes/OP-2026-0001"
	for _, c := range []struct {
		data, status, op string
		dias             int
	}{{"2026-11-30", "PENDENTE", "ATIVA", 0}, {"2026-12-10", "VENCIDA", "EM_ATRASO", 10}} {
		w := request("GET", base+"/parcelas?dataReferencia="+c.data, nil)
		var parcelas []struct {
			Status        string
			DiasEmAtraso  int
			ValorEmAberto decimal.Decimal
		}
		if err := json.Unmarshal(w.Body.Bytes(), &parcelas); err != nil {
			t.Fatal(err, w.Body)
		}
		if w.Code != 200 || len(parcelas) != 12 || parcelas[0].Status != c.status || parcelas[0].DiasEmAtraso != c.dias || !parcelas[0].ValorEmAberto.Equal(decimal.RequireFromString("936.91")) {
			t.Fatalf("parcelas: %d %s", w.Code, w.Body)
		}
		w = request("GET", base+"?dataReferencia="+c.data, nil)
		var op struct{ Status string }
		json.Unmarshal(w.Body.Bytes(), &op)
		if w.Code != 200 || op.Status != c.op {
			t.Fatalf("operação: %d %s", w.Code, w.Body)
		}
	}
	// RN-12: EM_ATRASO até 90 dias, INADIMPLENTE a partir do 91º (01/03/2027), na consulta e na listagem.
	for _, c := range []struct {
		data, status string
		dias         int
	}{{"2026-12-01", "EM_ATRASO", 1}, {"2027-02-28", "EM_ATRASO", 90}, {"2027-03-01", "INADIMPLENTE", 91}} {
		var op struct {
			Status       string
			DiasEmAtraso int
		}
		w := request("GET", base+"?dataReferencia="+c.data, nil)
		json.Unmarshal(w.Body.Bytes(), &op)
		if w.Code != 200 || op.Status != c.status || op.DiasEmAtraso != c.dias {
			t.Fatalf("operação em %s: %d %s", c.data, w.Code, w.Body)
		}
		var lista []struct {
			Status       string
			DiasEmAtraso int
		}
		w = request("GET", "/operacoes?dataReferencia="+c.data, nil)
		json.Unmarshal(w.Body.Bytes(), &lista)
		if w.Code != 200 || len(lista) != 1 || lista[0].Status != c.status || lista[0].DiasEmAtraso != c.dias {
			t.Fatalf("listagem em %s: %d %s", c.data, w.Code, w.Body)
		}
	}
	if w := request("GET", "/operacoes?dataReferencia=2026-02-30", nil); w.Code != 400 || !strings.Contains(w.Body.String(), "dataReferencia deve estar no formato YYYY-MM-DD") {
		t.Fatalf("listagem com data inválida: %d %s", w.Code, w.Body)
	}
	for _, path := range []string{base, base + "/parcelas"} {
		for _, data := range []string{"", "2026-02-30", "10-12-2026"} {
			w := request("GET", path+"?dataReferencia="+data, nil)
			if w.Code != 400 || !strings.Contains(w.Body.String(), "dataReferencia deve estar no formato YYYY-MM-DD") {
				t.Fatalf("data inválida: %d %s", w.Code, w.Body)
			}
		}
		hoje := time.Now().In(time.FixedZone("GMT-03:00", -10800)).Format(time.DateOnly)
		sem := request("GET", path, nil)
		com := request("GET", path+"?dataReferencia="+hoje, nil)
		if sem.Code != 200 || sem.Body.String() != com.Body.String() {
			t.Fatalf("hoje em GMT-03: %s / %s", sem.Body, com.Body)
		}
	}
	var status string
	if err := db.QueryRow("SELECT status FROM operacao WHERE numero_operacao='OP-2026-0001'").Scan(&status); err != nil || status != "ATIVA" {
		t.Fatal("consulta alterou operação", err, status)
	}
	if err := db.QueryRow("SELECT status FROM parcela WHERE numero=1").Scan(&status); err != nil || status != "PENDENTE" {
		t.Fatal("consulta alterou parcela", err, status)
	}
	if w := request("GET", "/operacoes/999999/parcelas", nil); w.Code != http.StatusNotFound {
		t.Fatalf("404: %d", w.Code)
	}
	if w := request("GET", "/operacoes/invalida", nil); w.Code != http.StatusBadRequest {
		t.Fatalf("400 identificador: %d", w.Code)
	}

	var id int64
	db.QueryRow("SELECT id FROM operacao LIMIT 1").Scan(&id)
	// lançamento financeiro da RN-14: encargos não reduzem o valor em aberto.
	var evento int64
	err = db.QueryRow(`INSERT INTO evento(operacao_id,tipo,data_efeito,valor) VALUES($1,'PAGAMENTO','2026-12-10',500) RETURNING id`, id).Scan(&evento)
	if err != nil {
		t.Fatal(err)
	}
	_, err = db.Exec(`INSERT INTO lancamento(evento_id,operacao_id,numero_parcela,multa,juros_mora,juros,amortizacao,valor) VALUES($1,$2,1,18.74,3.12,185,293.14,500)`, evento, id)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec("UPDATE parcela SET status='PARCIALMENTE_PAGA' WHERE numero=1"); err != nil {
		t.Fatal(err)
	}
	w := request("GET", base+"/parcelas?dataReferencia=2026-12-10", nil)
	if w.Code != 200 || !strings.Contains(w.Body.String(), `"valorEmAberto":458.77`) {
		t.Fatalf("pagamento parcial RN-14: %s", w.Body)
	}

	for _, c := range []struct{ tipo, data, anterior string }{{"PAGAMENTO", "2026-12-10", "2026-12-09"}, {"AMORTIZACAO_ANTECIPADA", "2026-12-11", "2026-12-10"}, {"LIQUIDACAO", "2026-12-12", "2026-12-11"}} {
		if c.tipo != "PAGAMENTO" {
			if _, err := db.Exec("INSERT INTO evento(operacao_id,tipo,data_efeito) VALUES($1,$2,$3::date)", id, c.tipo, c.data); err != nil {
				t.Fatal(err)
			}
		}
		for _, path := range []string{base, base + "/parcelas"} {
			w := request("GET", path+"?dataReferencia="+c.anterior, nil)
			if w.Code != 400 || !strings.Contains(w.Body.String(), "dataReferencia não pode ser anterior") {
				t.Fatalf("limite %s: %d %s", c.tipo, w.Code, w.Body)
			}
			if w := request("GET", path+"?dataReferencia="+c.data, nil); w.Code != 200 {
				t.Fatalf("mesma data %s: %s", c.tipo, w.Body)
			}
		}
	}
	// registro sem dinheiro não estabelece o limite de data.
	if _, err := db.Exec("INSERT INTO evento(operacao_id,tipo,data_efeito) VALUES($1,'OPERACAO_REGISTRADA','2030-01-01')", id); err != nil {
		t.Fatal(err)
	}
	if w := request("GET", base+"?dataReferencia=2026-12-12", nil); w.Code != 200 {
		t.Fatal(w.Body)
	}
	if _, err := db.Exec("UPDATE operacao SET status='LIQUIDADA'; UPDATE parcela SET status='PAGA'"); err != nil {
		t.Fatal(err)
	}
	w = request("GET", base+"?dataReferencia=2027-12-01", nil)
	if w.Code != 200 || !strings.Contains(w.Body.String(), `"status":"LIQUIDADA"`) {
		t.Fatal(w.Body)
	}
	w = request("GET", base+"/parcelas?dataReferencia=2027-12-01", nil)
	var pagas []struct {
		Status        string
		ValorEmAberto decimal.Decimal
		DiasEmAtraso  int
	}
	if err := json.Unmarshal(w.Body.Bytes(), &pagas); err != nil {
		t.Fatal(err)
	}
	for _, p := range pagas {
		if p.Status != "PAGA" || !p.ValorEmAberto.IsZero() || p.DiasEmAtraso != 0 {
			t.Fatal("parcela paga com atraso ou saldo", p)
		}
	}
}
