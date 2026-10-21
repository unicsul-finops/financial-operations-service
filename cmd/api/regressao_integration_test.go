package main

import (
	"bytes"
	"encoding/json"
	"net/http/httptest"
	"reflect"
	"strconv"
	"testing"

	"github.com/shopspring/decimal"
	"github.com/unicsul-finops/financial-operations-service/internal/handler"
	"github.com/unicsul-finops/financial-operations-service/internal/repository"
	"github.com/unicsul-finops/financial-operations-service/internal/service"
)

func decodificarResposta[T any](t *testing.T, w *httptest.ResponseRecorder) T {
	t.Helper()
	var valor T
	if err := json.Unmarshal(w.Body.Bytes(), &valor); err != nil {
		t.Fatalf("resposta inválida: %s: %v", w.Body, err)
	}
	return valor
}

// cobre o ciclo da fase 1 no mesmo router e banco usados pela implementação nova.
func TestRegressaoIntegracao(t *testing.T) {
	db := bancoDeIntegracao(t)
	anterior := decimal.MarshalJSONWithoutQuotes
	decimal.MarshalJSONWithoutQuotes = true
	t.Cleanup(func() { decimal.MarshalJSONWithoutQuotes = anterior })
	router := novoRouter(handler.NovoOperacaoHandler(service.NovoOperacaoService(repository.NovoOperacaoRepository(db))))
	request := func(metodo, caminho string, corpo []byte, status int) *httptest.ResponseRecorder {
		t.Helper()
		w := httptest.NewRecorder()
		r := httptest.NewRequest(metodo, caminho, bytes.NewReader(corpo))
		r.Header.Set("Content-Type", "application/json")
		router.ServeHTTP(w, r)
		if w.Code != status {
			t.Fatalf("%s %s: esperado %d, recebido %d: %s", metodo, caminho, status, w.Code, w.Body)
		}
		return w
	}
	request("GET", "/health", nil, 200)
	if w := request("GET", "/operacoes", nil, 200); w.Body.String() != "[]" {
		t.Fatalf("lista vazia: %s", w.Body)
	}
	exemplos := carregarSpec(t).Paths.Value("/operacoes").Post.RequestBody.Value.Content["application/json"].Examples
	for _, c := range []struct{ exemplo, numero, parcela string }{
		{"price", "OP-2026-0001", "936.91"},
		{"sac", "OP-2026-0002", "1018.33"},
	} {
		t.Run(c.exemplo, func(t *testing.T) {
			payload, err := json.Marshal(exemplos[c.exemplo].Value.Value)
			if err != nil {
				t.Fatal(err)
			}
			w := request("POST", "/operacoes", payload, 201)
			criada := decodificarResposta[handler.OperacaoCriadaResponse](t, w)
			if criada.ID <= 0 || criada.NumeroOperacao != c.numero || criada.Status != "ATIVA" || criada.CriadoEm == "" || w.Header().Get("Location") != "/operacoes/"+c.numero {
				t.Fatalf("confirmação de criação: %s", w.Body)
			}
			reenvio := decodificarResposta[handler.OperacaoCriadaResponse](t, request("POST", "/operacoes", payload, 200))
			if criada != reenvio {
				t.Fatal("reenvio mudou a confirmação")
			}
			var quantidade int
			if err := db.QueryRow("SELECT COUNT(*) FROM parcela WHERE operacao_id=$1", criada.ID).Scan(&quantidade); err != nil || quantidade != 12 {
				t.Fatal("reenvio duplicou parcelas ou houve erro de leitura", quantidade, err)
			}
			base := "/operacoes/" + c.numero
			op := decodificarResposta[handler.OperacaoResponse](t, request("GET", base+"?dataReferencia=2026-11-30", nil, 200))
			porID := decodificarResposta[handler.OperacaoResponse](t, request("GET", "/operacoes/"+strconv.FormatInt(criada.ID, 10)+"?dataReferencia=2026-11-30", nil, 200))
			minusculo := "/operacoes/op-2026-0001"
			if c.exemplo == "sac" {
				minusculo = "/operacoes/op-2026-0002"
			}
			porMinusculo := decodificarResposta[handler.OperacaoResponse](t, request("GET", minusculo+"?dataReferencia=2026-11-30", nil, 200))
			if !reflect.DeepEqual(op, porID) || !reflect.DeepEqual(op, porMinusculo) {
				t.Fatal("busca por id ou minúsculas diverge")
			}
			if op.NumeroOperacao != c.numero || op.Status != "ATIVA" || !op.SaldoDevedor.Equal(decimal.NewFromInt(10000)) || !op.CondicoesFinanceiras.TaxaJuros.Equal(decimal.RequireFromString("0.0185")) || op.DefinidoEm != "2026-10-08T18:12:40Z" {
				t.Fatalf("dados da operação mudaram: %+v", op)
			}
			parcelas := decodificarResposta[[]handler.ParcelaResponse](t, request("GET", base+"/parcelas?dataReferencia=2026-11-30", nil, 200))
			if len(parcelas) != 12 || parcelas[0].Vencimento != "2026-11-30" || !parcelas[0].ValorParcela.Equal(decimal.RequireFromString(c.parcela)) || !parcelas[11].SaldoDevedor.IsZero() {
				t.Fatalf("calendário mudou: %+v", parcelas)
			}
			soma := decimal.Zero
			for i, p := range parcelas {
				if p.Numero != i+1 || p.Status != "PENDENTE" || !p.ValorParcela.Equal(p.Amortizacao.Add(p.Juros)) {
					t.Fatalf("parcela inesperada: %+v", p)
				}
				soma = soma.Add(p.Amortizacao)
			}
			if !soma.Equal(decimal.NewFromInt(10000)) {
				t.Fatal("principal não fecha ao centavo", soma)
			}
			porIDParcelas := decodificarResposta[[]handler.ParcelaResponse](t, request("GET", "/operacoes/"+strconv.FormatInt(criada.ID, 10)+"/parcelas?dataReferencia=2026-11-30", nil, 200))
			if !reflect.DeepEqual(parcelas, porIDParcelas) {
				t.Fatal("parcelas por id divergem")
			}
		})
	}
	listadas := decodificarResposta[[]handler.OperacaoResponse](t, request("GET", "/operacoes", nil, 200))
	if len(listadas) != 2 {
		t.Fatalf("esperava duas operações, recebeu %d", len(listadas))
	}
	if listadas[0].NumeroOperacao != "OP-2026-0002" || listadas[1].NumeroOperacao != "OP-2026-0001" {
		t.Fatal("ordem da listagem mudou")
	}
	for _, c := range []struct {
		metodo, caminho, corpo string
		status                 int
	}{
		{"POST", "/operacoes", "{", 400},
		{"POST", "/operacoes", "{}", 400},
		{"GET", "/operacoes/invalida", "", 400},
		{"GET", "/operacoes/999999", "", 404},
		{"GET", "/operacoes/999999/parcelas", "", 404},
		{"GET", "/inexistente", "", 404},
		{"DELETE", "/operacoes/1", "", 405},
	} {
		w := request(c.metodo, c.caminho, []byte(c.corpo), c.status)
		erro := decodificarResposta[map[string]string](t, w)
		if len(erro) != 1 || erro["erro"] == "" {
			t.Fatalf("formato de erro mudou: %s", w.Body)
		}
	}
	request("GET", "/openapi.yaml", nil, 200)
	request("GET", "/swagger/index.html", nil, 200)
}
