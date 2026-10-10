# Guia de convenções da API

Regras que **todo endpoint** do financial-operations-service segue. Elas já valem para os endpoints da Fase 1; os da Fase 2 precisam seguir igual, para a API parecer feita por uma pessoa só.

Antes de abrir a PR de contrato de uma história, confira o [checklist](#checklist-da-pr-de-contrato) no fim deste guia. As regras de negócio estão em [regras-de-negocio-fase-2.md](../../requirements/regras-de-negocio-fase-2.md), e o contrato atual completo em [`api/openapi.yaml`](../../../api/openapi.yaml).

## Como trabalhar: contrato primeiro

1. A **primeira PR** de uma história é o contrato: só o `api/openapi.yaml` e a coleção em `collections/`, sem código Go.
2. O arquiteto revisa o contrato contra este guia e as RNs.
3. Com o contrato aprovado, a implementação segue o que foi combinado. Se precisar mudar o contrato no meio, a mudança vai junto na PR, explicada na descrição.

Assim quem consome a API, e as outras trilhas do time, podem contar com o contrato antes de o código existir.

## Rotas

- Recursos no **plural e em português**, sem verbo na rota: `/operacoes`, `/parcelas`, `/pagamentos`. A ação é o método HTTP.
- Recurso de uma operação fica embaixo dela: `/operacoes/{identificador}/parcelas/{numero}/pagamentos`.
- A operação é identificada pelo **`{identificador}`**, que aceita o `numeroOperacao` (`OP-2026-0001`, o jeito normal) ou o `id`. Use o parâmetro `IdentificadorOperacao` que já existe no `openapi.yaml`, não crie outro.
- Parcela é identificada pelo `{numero}` dela (1, 2, 3…), não por um id interno.
- Consultas de valor calculado que não são um recurso gravado usam `GET` com nome de substantivo: `/liquidacao`, `/resumo`, `/amortizacoes/simulacao`.

## Métodos e status HTTP

| Situação | Método | Status |
| --- | --- | --- |
| Consulta | `GET` | `200` |
| Criou algo | `POST` | `201` + header `Location` apontando para o recurso, de preferência pelo identificador |
| Reenvio com o mesmo identificador externo (nada foi gravado) | `POST` | `200` com o que já estava gravado |
| JSON malformado ou regra de negócio violada | qualquer | `400` |
| Operação, parcela ou rota que não existe | qualquer | `404` |
| Método não aceito numa rota que existe | qualquer | `405` (já tratado pelo router) |

Não use `422`, `409` nem `204`: o resto da API não usa, e quem consome teria que tratar mais casos.

## Formato do JSON

- Campos em **camelCase e em português**: `dataPagamento`, `valorEmAberto`, `numeroParcela`.
- Valores fixos (status, tipos) em **MAIÚSCULAS com `_`**: `PARCIALMENTE_PAGA`, `AMORTIZACAO_ANTECIPADA`. Na entrada, são aceitos em minúsculas e convertidos.
- Lista vazia sai como **`[]`**, nunca `null`.
- Um campo só some da resposta (`omitempty`) quando a ausência tem significado e isso está escrito na descrição do schema. Na dúvida, sempre presente.
- Uma resposta de ação (pagar, amortizar, liquidar) é **autoexplicativa**: mostra o que foi feito e como ficou (por exemplo, quanto foi para cada componente e o saldo depois), para quem chamou não precisar refazer conta.

## Dinheiro, taxas e datas

| Tipo | Regra | Exemplo |
| --- | --- | --- |
| Dinheiro | Número JSON, até 2 casas, `decimal.Decimal` no Go e `NUMERIC(15,2)` no banco. Nunca `float64`. Entrada com mais de 2 casas é recusada, não arredondada. | `936.91` |
| Taxa | Número JSON **em fração** nas respostas. A única entrada em percentual é o payload do juros. | `0.0185` = 1,85% |
| Data | Texto `YYYY-MM-DD`, dia em GMT-03:00. | `"2026-12-10"` |
| Data e hora | Texto ISO-8601 em **UTC**, com `Z`. | `"2026-10-08T18:12:40Z"` |

O JSON não preserva zero à direita: `185.00` sai como `185`. Nos exemplos do `openapi.yaml`, escreva como sai (`185`).

## Data de referência

- Toda consulta cujo resultado depende do dia (atraso, encargos, valor para quitar, resumo, histórico) aceita **`?dataReferencia=YYYY-MM-DD`**. Sem o parâmetro, vale hoje em GMT-03:00 (RN-09).
- Toda ação com dinheiro recebe **`dataPagamento`** no corpo. Sem o campo, vale hoje. Não pode ser futura nem anterior à `dataLiberacao`.
- Crie o parâmetro `dataReferencia` uma vez em `components/parameters` e reaproveite com `$ref`.

## Reenvio (idempotência)

- Toda ação com dinheiro aceita um identificador externo **opcional** no corpo (`identificadorPagamento` no pagamento; use o mesmo nome nas outras ações).
- Mesmo identificador na mesma operação → nada é gravado, resposta `200` com o que já estava gravado.
- Sem o identificador, a ação é sempre executada.
- No banco, ele fica em `evento.identificador_externo`, que já é único por operação.

## Erros

- Sempre `{"erro": "mensagem"}`, com uma mensagem só.
- Mensagem em **português, minúscula, citando o caminho do campo** como ele aparece no JSON: `"valor deve ser maior que zero"`, `"dataPagamento deve estar no formato YYYY-MM-DD"`, `"parcela 3 já está paga"`.
- A mensagem diz o que está errado **e o que é aceito**, quando couber: `"a parcela 5 ainda não pode ser paga, só as vencidas e a próxima a vencer"`.
- Toda mensagem de erro que o endpoint pode devolver aparece como exemplo nomeado no `openapi.yaml` e na tabela de erros do [contrato](README.md#erros).

## Código

As camadas e as convenções de código estão no [ADR-002](../adr/ADR-002-arquitetura-em-camadas.md) e na seção "Por onde começar a ler" do [README](../../../README.md). Em resumo:

- **handler:** structs de request/response e status HTTP. Não valida regra.
- **service:** toda validação e regra de negócio. Validação de payload separada dos casos de uso, como em `validacao.go` e `calendario.go`.
- **repository:** só SQL. Operações com mais de uma escrita usam **uma transação**.
- Mudança no banco é uma **migration nova** em `migrations/`; as que já existem não são editadas ([ADR-007](../adr/ADR-007-migrations-sql-versionadas.md)).
- Comentário explica o **porquê**, em português, minúsculo, curto.
- Teste com valores **ao centavo**, sem tolerância, usando os exemplos das RNs.

## Checklist da PR de contrato

- [ ] Rotas seguem as convenções acima e usam o parâmetro `IdentificadorOperacao`.
- [ ] Cada endpoint tem `summary`, `description` citando as RNs, `operationId` e a tag `Operações`.
- [ ] Schemas de request e response com `required`, tipos e descrição dos campos que não são óbvios.
- [ ] Exemplo de request e de response com os **números das RNs** (por exemplo, o pagamento de 500,00 da RN-14).
- [ ] Um exemplo nomeado para **cada mensagem de erro** possível.
- [ ] `dataReferencia` nas consultas e `dataPagamento` nas ações com dinheiro.
- [ ] Identificador externo opcional nas ações com dinheiro.
- [ ] Pedidos novos na coleção em `collections/`, com testes de status e dos valores principais.
- [ ] Tabela de endpoints e de erros do [contrato](README.md) atualizada.
- [ ] `go test ./...` passa (ele valida o `openapi.yaml` e os exemplos contra os schemas).

Quando o código entrar, o teste `TestOpenAPICobreTodasAsRotas` confere que toda rota do router está no contrato e vice-versa.
