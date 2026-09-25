# ADR-008: Swagger com OpenAPI 3 escrito à mão e validado nos testes

## Status

Aceita

## Data

2026-09-24

## Contexto

Os outros grupos da esteira já expõem Swagger. Precisávamos publicar a documentação interativa deste serviço, de forma que ela reflita fielmente o contrato ([Contratos da API](../contracts/README.md)) e não fique desatualizada conforme as rotas mudarem.

Particularidades do contrato que a documentação precisa representar:

- valores monetários calculados com `decimal.Decimal`, que no JSON saem como número;
- o parâmetro `:id` aceita **UUID ou `numeroOperacao`**;
- `contextoScore` opcional e omitido da resposta quando ausente;
- o `POST` responde com `parcelas`, e as consultas não.

## Opções consideradas

- **swaggo/swag:** o spec é gerado a partir de comentários nos handlers, com `swag init`.
- **OpenAPI 3 escrito à mão**, embutido no binário e servido com Swagger UI.

## Decisão

Escrevemos a especificação **OpenAPI 3.0** à mão, em [`api/openapi.yaml`](../../../api/openapi.yaml).

- O arquivo é embutido no binário com `go:embed` (`api/openapi.go`), então o Swagger funciona de qualquer pasta em que a API for executada.
- O Swagger UI é servido por `gin-swagger` em `/swagger`, com os arquivos da interface embutidos (sem depender de CDN, funciona sem internet no laboratório). O spec cru fica em `/openapi.yaml`.
- `servers` usa a URL relativa `/`. Assim, quem abre o Swagger de outra máquina (`http://IP:8083/swagger`) testa contra esse mesmo IP, e não contra o próprio `localhost`.

O swaggo foi descartado porque:

- gera apenas Swagger 2.0, sem os recursos do OpenAPI 3 usados aqui (`allOf` para o POST com parcelas, `examples` nomeados por caso de erro, parâmetro com duas formas);
- não entende `decimal.Decimal` sem anotações de override em cada campo;
- espalharia blocos de anotação pelos handlers, que hoje são finos ([ADR-002](ADR-002-arquitetura-em-camadas.md)), e exigiria rodar `swag init` e versionar os arquivos gerados.

Para o spec escrito à mão não se desatualizar, há dois testes em `cmd/api/main_test.go`:

| Teste | O que garante |
| --- | --- |
| `TestOpenAPIValido` | O `openapi.yaml` é um OpenAPI válido, e **cada exemplo bate com o seu schema** (via `kin-openapi`). |
| `TestOpenAPICobreTodasAsRotas` | Toda rota registrada no router está no spec, e todo path do spec existe no router. |

Para permitir essa comparação, a montagem das rotas saiu do `main` para a função `novoRouter`.

## Consequências

- A documentação cobre exatamente o contrato, com exemplos reais de request, response e erros.
- Criar ou remover uma rota sem atualizar o spec quebra o `go test`.
- **Limite da verificação:** os testes conferem as rotas e a consistência interna do spec, mas não que os campos do schema correspondam aos das structs Go. Uma mudança de campo num payload exige atualizar o `openapi.yaml` manualmente, e a revisão da PR deve conferir isso.
- Novas dependências: `swaggo/gin-swagger` e `swaggo/files` (UI) e `getkin/kin-openapi` (validação).
