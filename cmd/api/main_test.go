package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sort"
	"strings"
	"testing"

	"github.com/getkin/kin-openapi/openapi3"
	"github.com/gin-gonic/gin"
	"github.com/unicsul-finops/financial-operations-service/api"
)

// rotas de documentação não fazem parte do contrato, então ficam fora da comparação.
var rotasDeDocumentacao = map[string]bool{
	"GET /openapi.yaml": true,
	"GET /swagger":      true,
	"GET /swagger/*any": true,
}

func carregarSpec(t *testing.T) *openapi3.T {
	t.Helper()

	spec, err := openapi3.NewLoader().LoadFromData(api.OpenAPI)
	if err != nil {
		t.Fatalf("openapi.yaml não carregou: %v", err)
	}

	return spec
}

// valida o spec contra a especificação OpenAPI, incluindo os exemplos contra os schemas.
func TestOpenAPIValido(t *testing.T) {
	if err := carregarSpec(t).Validate(context.Background()); err != nil {
		t.Fatalf("openapi.yaml inválido: %v", err)
	}
}

// se alguém criar ou remover uma rota sem atualizar o openapi.yaml, este teste quebra.
func TestOpenAPICobreTodasAsRotas(t *testing.T) {
	gin.SetMode(gin.TestMode)

	registradas := map[string]bool{}
	for _, rota := range novoRouter(nil).Routes() {
		chave := rota.Method + " " + rota.Path
		if rotasDeDocumentacao[chave] {
			continue
		}
		// o gin usa :id, o openapi usa {id}.
		registradas[rota.Method+" "+converterParametros(rota.Path)] = true
	}

	documentadas := map[string]bool{}
	for caminho, item := range carregarSpec(t).Paths.Map() {
		for metodo := range item.Operations() {
			documentadas[metodo+" "+caminho] = true
		}
	}

	if faltando := diferenca(registradas, documentadas); len(faltando) > 0 {
		t.Errorf("rotas sem documentação no openapi.yaml: %v", faltando)
	}
	if sobrando := diferenca(documentadas, registradas); len(sobrando) > 0 {
		t.Errorf("rotas documentadas que não existem no router: %v", sobrando)
	}
}

func TestSwaggerUIResponde(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := novoRouter(nil)

	casos := []struct {
		caminho, localizacao string
		status               int
	}{
		{"/swagger", "/swagger/index.html", http.StatusMovedPermanently},
		{"/swagger/", "/swagger/index.html", http.StatusMovedPermanently},
		{"/swagger/index.html", "", http.StatusOK},
		{"/openapi.yaml", "", http.StatusOK},
	}

	for _, c := range casos {
		resposta := httptest.NewRecorder()
		router.ServeHTTP(resposta, httptest.NewRequest(http.MethodGet, c.caminho, nil))

		if resposta.Code != c.status {
			t.Errorf("%s: status %d, esperava %d", c.caminho, resposta.Code, c.status)
		}
		if c.localizacao != "" && resposta.Header().Get("Location") != c.localizacao {
			t.Errorf("%s: redirecionou para %q, esperava %q", c.caminho, resposta.Header().Get("Location"), c.localizacao)
		}
	}
}

func converterParametros(caminho string) string {
	partes := strings.Split(caminho, "/")
	for i, parte := range partes {
		if strings.HasPrefix(parte, ":") {
			partes[i] = "{" + strings.TrimPrefix(parte, ":") + "}"
		}
	}
	return strings.Join(partes, "/")
}

func diferenca(a, b map[string]bool) []string {
	resultado := []string{}
	for chave := range a {
		if !b[chave] {
			resultado = append(resultado, chave)
		}
	}
	sort.Strings(resultado)
	return resultado
}
