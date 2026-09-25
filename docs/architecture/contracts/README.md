# Contratos da API

Contrato HTTP do **financial-operations-service**. Todos os endpoints recebem e devolvem JSON.

> **Fonte oficial:** a especificação OpenAPI 3 em [`api/openapi.yaml`](../../../api/openapi.yaml), navegável em `http://localhost:8083/swagger` com a API rodando. Este documento é o resumo em texto; se os dois divergirem, vale o `openapi.yaml`, que é validado nos testes ([ADR-008](../adr/ADR-008-openapi-escrito-a-mao.md)).

- **URL base local:** `http://localhost:8083`
- **Autenticação:** nenhuma nesta fase.
- **Datas:** texto no formato `YYYY-MM-DD`. O `criadoEm` é um timestamp UTC ISO-8601.
- **Valores monetários e taxas:** números JSON, calculados com decimal exato ([ADR-005](../adr/ADR-005-valores-monetarios-decimal.md)). Zeros à direita não são preservados (`298.50` sai como `298.5`).

## Resumo dos endpoints

| Método | Rota | Descrição | Sucesso |
| --- | --- | --- | --- |
| `GET` | `/health` | Verifica se o serviço está no ar | `200` |
| `POST` | `/operacoes` | Registra uma operação aprovada e gera o cronograma | `201` |
| `GET` | `/operacoes` | Lista as operações (sem as parcelas) | `200` |
| `GET` | `/operacoes/:id` | Consulta a situação de uma operação | `200` |
| `GET` | `/operacoes/:id/parcelas` | Consulta o cronograma de parcelas | `200` |
| `GET` | `/swagger` | Swagger UI | `301` → `/swagger/index.html` |
| `GET` | `/openapi.yaml` | Especificação OpenAPI 3 | `200` |

O `:id` aceita o **UUID** (`id`) **ou** o **número da operação** (`numeroOperacao`, ex.: `OP-2026-000001`, em maiúsculas ou minúsculas). Ver [ADR-006](../adr/ADR-006-numero-operacao-legivel.md).

---

## `POST /operacoes`

Contrato combinado com o grupo de **Decisão**. Registra uma operação **já aprovada**.

- O payload **não** traz ID: o serviço gera o `id` (UUID) e o `numeroOperacao` ([ADR-003](../adr/ADR-003-id-gerado-pelo-servico.md)).
- O payload **não** traz o calendário de parcelas: o serviço gera o cronograma ([ADR-004](../adr/ADR-004-cronograma-gerado-internamente.md)).

### Request

```json
{
  "clienteId": "12345678901",
  "tipoPessoa": "PF",
  "decisao": "APROVADO",
  "dataAprovacao": "2026-09-25",
  "condicoesFinanceiras": {
    "valorAprovado": 15000.00,
    "prazoMeses": 24,
    "quantidadeParcelas": 24,
    "taxaJuros": 0.0199,
    "sistemaAmortizacao": "PRICE",
    "primeiroVencimento": "2026-10-25"
  },
  "contextoScore": {
    "scoreFinal": 872,
    "faixaRisco": "EXCELENTE",
    "probabilidadeDefault": 0.0128,
    "modelo": "SCORE_PF v1.1.0",
    "motivo": "Score 872 atende ao mínimo para aprovação automática."
  }
}
```

Um exemplo pronto está em [`examples/finops_operacao_create.json`](../../../examples/finops_operacao_create.json).

### Campos

| Campo | Tipo | Obrigatório | Regra |
| --- | --- | --- | --- |
| `clienteId` | texto | sim | Não vazio, até 20 caracteres. O formato de CPF/CNPJ não é validado aqui. |
| `tipoPessoa` | texto | sim | `PF` ou `PJ`. |
| `decisao` | texto | sim | Deve ser `APROVADO`. |
| `dataAprovacao` | data | sim | `YYYY-MM-DD`. |
| `condicoesFinanceiras.valorAprovado` | número | sim | Maior que zero, até 2 casas decimais. |
| `condicoesFinanceiras.prazoMeses` | inteiro | sim | Igual a `quantidadeParcelas`. |
| `condicoesFinanceiras.quantidadeParcelas` | inteiro | sim | De 1 a 420. |
| `condicoesFinanceiras.taxaJuros` | número | sim | Taxa **mensal em decimal** (`0.0199` = 1,99% a.m.). Maior ou igual a 0 e menor que 1, até 6 casas decimais. |
| `condicoesFinanceiras.sistemaAmortizacao` | texto | sim | `PRICE` ou `SAC`. |
| `condicoesFinanceiras.primeiroVencimento` | data | sim | `YYYY-MM-DD`, posterior a `dataAprovacao`. |
| `contextoScore` | objeto | não | Só é armazenado para consulta; não entra em nenhum cálculo. |
| `contextoScore.scoreFinal` | inteiro | não | Maior ou igual a 0. |
| `contextoScore.faixaRisco` | texto | não | Até 30 caracteres. |
| `contextoScore.probabilidadeDefault` | número | não | Entre 0 e 1. |
| `contextoScore.modelo` | texto | não | Até 50 caracteres. |
| `contextoScore.motivo` | texto | não | Livre. |

Os campos de texto têm os espaços das pontas removidos. `tipoPessoa`, `decisao` e `sistemaAmortizacao` são convertidos para maiúsculas antes da validação.

### Response `201 Created`

A operação criada, com o cronograma completo em `parcelas` (exemplo abreviado):

```json
{
  "id": "8524e7b3-7178-4aa6-99f0-eca1bff40de4",
  "numeroOperacao": "OP-2026-000005",
  "clienteId": "12345678901",
  "tipoPessoa": "PF",
  "status": "ATIVA",
  "dataAprovacao": "2026-09-25",
  "saldoDevedor": 15000,
  "condicoesFinanceiras": {
    "valorAprovado": 15000,
    "prazoMeses": 24,
    "quantidadeParcelas": 24,
    "taxaJuros": 0.0199,
    "sistemaAmortizacao": "PRICE",
    "primeiroVencimento": "2026-10-25"
  },
  "contextoScore": {
    "scoreFinal": 872,
    "faixaRisco": "EXCELENTE",
    "probabilidadeDefault": 0.0128,
    "modelo": "SCORE_PF v1.1.0",
    "motivo": "Score 872 atende ao mínimo para aprovação automática."
  },
  "criadoEm": "2026-09-25T02:27:23Z",
  "parcelas": [
    { "numero": 1, "vencimento": "2026-10-25", "valorParcela": 792.17, "amortizacao": 493.67, "juros": 298.5, "saldoDevedor": 14506.33, "status": "PENDENTE" },
    { "numero": 2, "vencimento": "2026-11-25", "valorParcela": 792.17, "amortizacao": 503.49, "juros": 288.68, "saldoDevedor": 14002.84, "status": "PENDENTE" },
    { "numero": 24, "vencimento": "2028-09-25", "valorParcela": 792.14, "amortizacao": 776.68, "juros": 15.46, "saldoDevedor": 0, "status": "PENDENTE" }
  ]
}
```

- `contextoScore` **é omitido** da resposta quando não foi enviado.
- Na última parcela, o valor pode diferir alguns centavos das demais, porque ela absorve os arredondamentos. É por isso que o exemplo mostra `792.14`.

---

## `GET /operacoes`

Lista todas as operações, da mais recente para a mais antiga. O formato de cada item é o mesmo da resposta do POST, **sem** o campo `parcelas`. Sem operações cadastradas, devolve `[]`.

## `GET /operacoes/:id`

Situação atual da operação: mesmo formato do item da listagem.

```text
GET /operacoes/8524e7b3-7178-4aa6-99f0-eca1bff40de4
GET /operacoes/OP-2026-000005
```

Na Fase 1 o `status` é sempre `ATIVA` e o `saldoDevedor` é igual ao `valorAprovado`, porque ainda não há pagamentos.

## `GET /operacoes/:id/parcelas`

Cronograma completo, ordenado por `numero`:

```json
[
  { "numero": 1, "vencimento": "2026-10-25", "valorParcela": 792.17, "amortizacao": 493.67, "juros": 298.5, "saldoDevedor": 14506.33, "status": "PENDENTE" }
]
```

| Campo | Descrição |
| --- | --- |
| `numero` | Sequência da parcela, começando em 1. |
| `vencimento` | Data de vencimento. |
| `valorParcela` | `amortizacao + juros`. |
| `amortizacao` | Parte do principal paga na parcela. |
| `juros` | Juros do período sobre o saldo anterior. |
| `saldoDevedor` | Saldo após a parcela. É `0` na última. |
| `status` | `PENDENTE` na Fase 1. Previstos para a Fase 2: `PAGA`, `PARCIALMENTE_PAGA` e `VENCIDA`. |

## `GET /health`

```json
{ "status": "ok" }
```

---

## Status da operação

| Status | Situação |
| --- | --- |
| `ATIVA` | Operação registrada e em dia. Único status usado na Fase 1. |
| `EM_ATRASO` | Previsto: há parcela vencida sem pagamento. |
| `INADIMPLENTE` | Previsto: atraso acima do limite definido pelas regras. |
| `RENEGOCIADA` | Previsto: novas condições negociadas. |
| `LIQUIDADA` | Previsto: saldo devedor zerado. |

## Erros

Todos os erros têm o formato:

```json
{ "erro": "mensagem em português" }
```

| HTTP | Quando | Exemplo de mensagem |
| --- | --- | --- |
| `400` | JSON malformado ou com tipo errado | `corpo da requisição inválido` |
| `400` | Regra de validação violada | `apenas operações com decisao APROVADO podem ser registradas` |
| `400` | `:id` que não é UUID nem número de operação | `identificador da operação inválido, use o id ou o numeroOperacao (ex.: OP-2026-000001)` |
| `404` | Operação não existe | `operação não encontrada` |

Mensagens de validação do `POST /operacoes`:

| Mensagem |
| --- |
| `clienteId não pode ser vazio` |
| `clienteId deve ter no máximo 20 caracteres` |
| `tipoPessoa deve ser PF ou PJ` |
| `apenas operações com decisao APROVADO podem ser registradas` |
| `dataAprovacao deve estar no formato YYYY-MM-DD` |
| `primeiroVencimento deve estar no formato YYYY-MM-DD` |
| `primeiroVencimento deve ser posterior à dataAprovacao` |
| `valorAprovado deve ser maior que zero` |
| `valorAprovado deve ter no máximo 2 casas decimais` |
| `quantidadeParcelas deve ser maior que zero` |
| `quantidadeParcelas deve ser no máximo 420` |
| `prazoMeses deve ser igual a quantidadeParcelas` |
| `taxaJuros é obrigatória` |
| `taxaJuros deve ser maior ou igual a zero` |
| `taxaJuros deve ser a taxa mensal em decimal, ex.: 0.0199 para 1,99% ao mês` |
| `taxaJuros deve ter no máximo 6 casas decimais` |
| `sistemaAmortizacao deve ser PRICE ou SAC` |
| `contextoScore.scoreFinal deve ser maior ou igual a zero` |
| `contextoScore.probabilidadeDefault deve estar entre 0 e 1` |
| `contextoScore.faixaRisco deve ter no máximo 30 caracteres` |
| `contextoScore.modelo deve ter no máximo 50 caracteres` |

> **Limitação conhecida:** falhas inesperadas do banco também são devolvidas como `400`, com a mensagem original do erro. Separá-las em `500` está previsto ([ADR-002](../adr/ADR-002-arquitetura-em-camadas.md)).

## Idempotência

O serviço **não** é idempotente: reenviar o mesmo `POST /operacoes` cria uma nova operação, com novo `id` e novo `numeroOperacao`. Ver [ADR-003](../adr/ADR-003-id-gerado-pelo-servico.md).
