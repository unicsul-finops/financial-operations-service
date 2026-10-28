# Contratos da API

Contrato HTTP do **financial-operations-service**. Todos os endpoints recebem e devolvem JSON.

> **Endpoint novo?** Siga o [guia de convenções](guia-de-convencoes.md), que traz as regras comuns a todos os endpoints e o checklist da PR de contrato.

> **Fonte oficial:** a especificação OpenAPI 3 em [`api/openapi.yaml`](../../../api/openapi.yaml), navegável em `http://localhost:8083/swagger` com a API rodando. Este documento é o resumo em texto; se os dois divergirem, vale o `openapi.yaml`, que é validado nos testes ([ADR-008](../adr/ADR-008-openapi-escrito-a-mao.md)).

- **URL base local:** `http://localhost:8083`
- **Autenticação:** nenhuma nesta fase.
- **Datas:** texto no formato `YYYY-MM-DD`. `definidoEm` e `criadoEm` saem como timestamp UTC ISO-8601.
- **Data/hora recebida sem fuso:** interpretada como **GMT-03:00** ([ADR-009](../adr/ADR-009-operacao-a-partir-da-simulacao-de-juros.md)).
- **Valores monetários:** números JSON, calculados com decimal exato ([ADR-005](../adr/ADR-005-valores-monetarios-decimal.md)). Zeros à direita não são preservados (`185.00` sai como `185`).
- **Taxas:** o juros envia em **percentual** (`1.85` = 1,85%); a API devolve **toda taxa em fração** (`0.0185`).

## Resumo dos endpoints

| Método | Rota | Descrição | Sucesso |
| --- | --- | --- | --- |
| `GET` | `/health` | Verifica se o serviço está no ar | `200` |
| `POST` | `/operacoes` | Registra a operação definida no juros, com o calendário | `201` (nova) / `200` (reenvio) |
| `GET` | `/operacoes` | Lista as operações (sem as parcelas) | `200` |
| `GET` | `/operacoes/:identificador` | Consulta a situação na `dataReferencia` (opcional) | `200` |
| `GET` | `/operacoes/:identificador/parcelas` | Consulta parcelas, atraso e encargos na `dataReferencia` (opcional) | `200` |
| `GET` | `/swagger` | Swagger UI | `301` → `/swagger/index.html` |
| `GET` | `/openapi.yaml` | Especificação OpenAPI 3 | `200` |

As consultas são feitas pelo **identificador do juros**, que é o `numeroOperacao` (ex.: `/operacoes/OP-2026-0001`, em maiúsculas ou minúsculas). É assim que a operação é conhecida em toda a esteira. O `id` interno (inteiro, ex.: `/operacoes/1`) também é aceito, mas só faz sentido dentro deste serviço.

---

## `POST /operacoes`

Payload enviado pelo **serviço de juros**, com a simulação que o cliente escolheu ([ADR-009](../adr/ADR-009-operacao-a-partir-da-simulacao-de-juros.md)).

- O `operacao.identificador` é **obrigatório** e vira o `numeroOperacao`, o mesmo em toda a esteira. Este serviço não gera número próprio; gera só o `id` técnico, sequencial ([ADR-011](../adr/ADR-011-id-sequencial.md)).
- O `calendario` é **gravado como veio**, sem recálculo, depois de conferido.
- **Idempotente pelo identificador:** se ele já existir, nada é gravado e a resposta é `200`.
- A resposta só confirma o registro. O juros já tem os dados que enviou; quem precisar da operação completa usa os `GET`.

### Request

O payload completo está no exemplo `price` do `POST /operacoes` no Swagger e na coleção em [`collections/`](../../../collections) (calendário abreviado abaixo):

```json
{
  "operacao": {
    "id": 1,
    "identificador": "OP-2026-0001",
    "estado": "DEFINIDA",
    "criadoEm": "2026-10-08 14:03:22",
    "definidoEm": "2026-10-08 15:12:40"
  },
  "cliente": { "tipoPessoa": "PF", "documento": "52998224725" },
  "produto": {
    "codigo": "CONSIGNADO_INSS",
    "nome": "Crédito pessoal consignado INSS",
    "modalidadeBcb": "Crédito pessoal consignado INSS - Prefixado"
  },
  "condicoes": { "valor": 10000, "prazoMeses": 12, "dataLiberacao": "2026-10-30", "primeiroRelacionamento": true },
  "taxa": { "faixaRisco": "B", "taxaFinalMes": 1.85, "taxaFinalAno": 24.6, "tetoAplicado": true },
  "sistema": "PRICE",
  "resumo": { "parcelaFixa": 936.91, "totalJuros": 1242.88, "totalPago": 11242.88 },
  "encargos": {
    "iof": { "diario": 167.65, "adicional": 38, "total": 205.65 },
    "tarifaCadastro": 50,
    "total": 255.65
  },
  "valorLiberado": 9744.35,
  "cet": { "anualPercentual": 30.89, "mensalPercentual": 2.27 },
  "calendario": [
    { "numero": 1, "amortizacao": 751.91, "juros": 185, "valor": 936.91, "saldoDevedor": 9248.09, "vencimento": "2026-11-30", "diasCorridos": 31 },
    { "numero": 12, "amortizacao": 919.85, "juros": 17.02, "valor": 936.87, "saldoDevedor": 0, "vencimento": "2027-11-01", "diasCorridos": 367 }
  ]
}
```

### Campos

| Campo | Tipo | Obrigatório | Regra |
| --- | --- | --- | --- |
| `operacao.identificador` | texto | sim | `OP-AAAA-NNNN` (4 dígitos ou mais). Vira o `numeroOperacao`. |
| `operacao.estado` | texto | sim | Deve ser `DEFINIDA`. |
| `operacao.definidoEm` | texto | sim | `YYYY-MM-DD HH:MM:SS` (GMT-03:00) ou ISO-8601 com fuso. |
| `cliente.tipoPessoa` | texto | sim | `PF` ou `PJ`. |
| `cliente.documento` | texto | sim | Só dígitos: 11 para PF, 14 para PJ. Vira o `clienteId`. |
| `produto.codigo` | texto | sim | Até 50 caracteres. |
| `produto.nome` | texto | sim | Até 100 caracteres. |
| `produto.modalidadeBcb` | texto | sim | Até 150 caracteres. |
| `condicoes.valor` | número | sim | Maior que zero, até 2 casas. Vira o `valorAprovado` e o `saldoDevedor` inicial. |
| `condicoes.prazoMeses` | inteiro | sim | De 1 a 420, igual à quantidade de parcelas do `calendario`. |
| `condicoes.dataLiberacao` | data | sim | `YYYY-MM-DD`. O primeiro vencimento deve ser posterior. |
| `taxa.faixaRisco` | texto | sim | Até 10 caracteres. Faixa de preço do juros. |
| `taxa.taxaFinalMes` | número | sim | Percentual mensal, ≥ 0 e < 100, até 4 casas. |
| `taxa.taxaFinalAno` | número | sim | Percentual anual, ≥ 0, até 4 casas. |
| `taxa.tetoAplicado` | booleano | não | Padrão `false`. |
| `sistema` | texto | sim | `PRICE` ou `SAC`. |
| `resumo.totalJuros` | número | sim | Soma dos `juros` do calendário. Não é gravado. |
| `resumo.totalPago` | número | sim | Soma dos `valor` do calendário. Não é gravado. |
| `encargos.iof.diario`, `encargos.iof.adicional` | número | sim | ≥ 0, até 2 casas. |
| `encargos.iof.total` | número | sim | `diario + adicional`. |
| `encargos.tarifaCadastro` | número | sim | ≥ 0, até 2 casas. |
| `encargos.total` | número | sim | `iof.total + tarifaCadastro`. |
| `valorLiberado` | número | sim | `condicoes.valor - encargos.total`, maior que zero. |
| `cet.mensalPercentual`, `cet.anualPercentual` | número | sim | Percentual, ≥ 0, até 4 casas. |
| `calendario` | lista | sim | Ver abaixo. |

Os campos de texto têm os espaços das pontas removidos. `identificador`, `estado`, `tipoPessoa`, `produto.codigo`, `faixaRisco` e `sistema` são convertidos para maiúsculas antes da validação.

Aceitos e ignorados: `operacao.id`, `operacao.criadoEm`, `condicoes.primeiroRelacionamento`, `resumo.parcelaFixa`, `demonstrativo` e `calendario[].diasCorridos`.

### Conferência do calendário

O calendário não é recalculado, mas precisa fechar:

- uma parcela por mês de `prazoMeses`, com `numero` de `1` a `n`, em ordem;
- `vencimento` no formato `YYYY-MM-DD`, estritamente crescente, o primeiro depois da `dataLiberacao`;
- valores sem fração de centavo e `valor = amortizacao + juros`;
- `saldoDevedor` = saldo anterior − `amortizacao`, começando em `condicoes.valor` e terminando em `0`.

### Response `201 Created` / `200 OK`

`201` quando a operação foi criada, com o header `Location: /operacoes/OP-2026-0001`; `200` quando o identificador já existia (nada é gravado). O corpo é o mesmo nos dois casos:

```json
{
  "id": 1,
  "numeroOperacao": "OP-2026-0001",
  "status": "ATIVA",
  "criadoEm": "2026-10-10T17:32:14Z"
}
```

---

## `GET /operacoes`

Lista todas as operações, da mais recente para a mais antiga, no mesmo formato do `GET /operacoes/:identificador`. Sem operações cadastradas, devolve `[]`.

## `GET /operacoes/:identificador`

Situação atual da operação, com todos os dados registrados:

```text
GET /operacoes/OP-2026-0001
GET /operacoes/1              (pelo id interno, também aceito)
```

```json
{
  "id": 1,
  "numeroOperacao": "OP-2026-0001",
  "clienteId": "52998224725",
  "tipoPessoa": "PF",
  "status": "ATIVA",
  "definidoEm": "2026-10-08T18:12:40Z",
  "dataLiberacao": "2026-10-30",
  "saldoDevedor": 10000,
  "produto": {
    "codigo": "CONSIGNADO_INSS",
    "nome": "Crédito pessoal consignado INSS",
    "modalidadeBcb": "Crédito pessoal consignado INSS - Prefixado"
  },
  "condicoesFinanceiras": {
    "valorAprovado": 10000,
    "prazoMeses": 12,
    "quantidadeParcelas": 12,
    "taxaJuros": 0.0185,
    "taxaJurosAnual": 0.246,
    "faixaRisco": "B",
    "tetoAplicado": true,
    "sistemaAmortizacao": "PRICE",
    "primeiroVencimento": "2026-11-30"
  },
  "encargos": {
    "iof": { "diario": 167.65, "adicional": 38, "total": 205.65 },
    "tarifaCadastro": 50,
    "total": 255.65
  },
  "valorLiberado": 9744.35,
  "cet": { "mensal": 0.0227, "anual": 0.3089 },
  "criadoEm": "2026-10-10T17:32:14Z"
}
```

- `definidoEm` saiu 3 horas depois do enviado porque `15:12:40` em GMT-03:00 é `18:12:40` em UTC.
- O `saldoDevedor` começa no valor cheio: os encargos são descontados na liberação, não financiados.
- RN-09, RN-10 e RN-11: `?dataReferencia=YYYY-MM-DD` é opcional; omitido, vale hoje em GMT-03:00. Data vazia ou inexistente é inválida. Datas futuras são permitidas.
- A data não pode ser anterior ao último pagamento, amortização ou liquidação da operação; a mesma data é aceita. A consulta não reconstrói estados anteriores.
- Uma operação `ATIVA` com parcela vencida sai como `EM_ATRASO`; `LIQUIDADA` permanece `LIQUIDADA`. Nenhum atraso é gravado. O corte de inadimplência pertence à #8.
- Exemplo sem pagamentos: `2026-11-30` → `ATIVA`; `2026-12-10` → `EM_ATRASO`.

## `GET /operacoes/:identificador/parcelas`

Cronograma completo, ordenado por `numero`, com o mesmo parâmetro e validações de `dataReferencia` da consulta da operação (RN-09, RN-10, RN-11 e RN-13). Exemplo da primeira parcela sem pagamentos em `2026-12-10` (calendário completo no OpenAPI):

```json
[
  { "numero": 1, "vencimento": "2026-11-30", "valorParcela": 936.91, "amortizacao": 751.91, "juros": 185, "saldoDevedor": 9248.09, "status": "VENCIDA", "valorEmAberto": 936.91, "diasEmAtraso": 10, "encargos": { "multa": 18.74, "jurosMora": 3.12, "total": 21.86 }, "totalParaQuitar": 958.77 }
]
```

| Campo | Descrição |
| --- | --- |
| `numero` | Sequência da parcela, começando em 1. |
| `vencimento` | Data de vencimento, como veio do juros (dia útil). |
| `valorParcela` | `amortizacao + juros`. É o `valor` do calendário do juros. |
| `amortizacao` | Parte do principal paga na parcela. |
| `juros` | Juros do período. |
| `saldoDevedor` | Saldo após a parcela. É `0` na última. |
| `status` | `VENCIDA` se não paga por inteiro e a referência é posterior ao vencimento; prevalece sobre `PENDENTE` e `PARCIALMENTE_PAGA`. `PAGA` permanece `PAGA`. Apenas calculado, sem gravar. |
| `valorEmAberto` | Juros + amortização − juros pagos − amortização paga − desconto concedido (RN-13), sem multa/mora. `PAGA` sempre tem zero. |
| `diasEmAtraso` | Dias corridos desde o vencimento se vencida; zero antes, no vencimento ou se paga. |
| `encargos` | Objeto obrigatório com `multa`, `jurosMora` e `total` em aberto na referência, sempre presentes (RN-13). |
| `totalParaQuitar` | `valorEmAberto + encargos.total`, em reais; zero para PAGA. Não é a liquidação antecipada da operação (RN-18). |

### Encargos por atraso (RN-13)

Multa: 2% da base em aberto no primeiro dia de atraso, fixada nesse dia, menos multa paga. Mora: 1% ao mês, simples, por dia (divisor 30), sobre a base de cada trecho. O pagamento reduz a base a partir da sua data. Não há juros sobre multa nem capitalização.

Arredondar ao centavo, meio para cima, no resultado acumulado, sem arredondar cada dia ou trecho. Subtrair os pagamentos dos respectivos componentes. `total` soma multa e mora em aberto. Sem atraso: encargos zero. PAGA: valor em aberto, encargos e total para quitar zero, inclusive com desconto. Os encargos não são gravados.

| Situação da parcela 1 PRICE | Referência | Em aberto | Multa em aberto | Mora em aberto | Encargos em aberto | Total para quitar |
| --- | --- | --- | --- | --- | --- | --- |
| Sem pagamentos, no vencimento | 2026-11-30 | 936,91 | 0 | 0 | 0 | 936,91 |
| Sem pagamentos, 10 dias | 2026-12-10 | 936,91 | 18,74 | 3,12 | 21,86 | 958,77 |
| Após pagamento parcial de 500 em 10/12 (RN-14) | 2026-12-20 | 458,77 | 0 | 1,53 | 1,53 | 460,30 |
| PAGA | Referência válida após quitação | 0 | 0 | 0 | 0 | 0 |

O pagamento parcial distribui 18,74 para multa, 3,12 para mora, 185 para juros e 293,14 para amortização. Em 20/12 a mora acumulada é 4,65, menos 3,12 pagos = 1,53 em aberto. O estado gravado é PARCIALMENTE_PAGA, exibido VENCIDA na consulta (RN-11).

Os erros de referência inválida ou anterior à última movimentação são os mesmos da RN-09; esta história não cria erros de entrada novos. O parâmetro DataReferencia e o schema de erros são reaproveitados da #6.

A pasta Contrato #7 da coleção contém dois pedidos sem pagamento e respostas salvas para pagamento parcial e PAGA. Respostas salvas são exemplos do contrato, não evidência de execução. O cenário parcial exige preparar os lançamentos da RN-14; registrar pagamento pertence à #9. Os campos novos só funcionarão após aprovação do contrato e implementação em PR posterior.


## `GET /health`

```json
{ "status": "ok" }
```

---

## Status da operação

| Status | Situação |
| --- | --- |
| `ATIVA` | Operação registrada, sem parcelas vencidas na data de referência. |
| `EM_ATRASO` | Fase 2 ([#8](https://github.com/unicsul-finops/financial-operations-service/issues/8)): há parcela vencida sem pagamento. Calculado na consulta ([RN-11](../../requirements/regras-de-negocio-fase-2.md#rn-11-situação-calculada-na-consulta)). |
| `INADIMPLENTE` | Fase 2 ([#8](https://github.com/unicsul-finops/financial-operations-service/issues/8)): alguma parcela com mais de 90 dias de atraso. Calculado na consulta ([RN-12](../../requirements/regras-de-negocio-fase-2.md#rn-12-inadimplência)). |
| `RENEGOCIADA` | Previsto: novas condições negociadas. Depende de outro grupo. |
| `LIQUIDADA` | Fase 2 ([#11](https://github.com/unicsul-finops/financial-operations-service/issues/11)): quitada pela liquidação ou pelo pagamento da última parcela ([RN-18 e RN-19](../../requirements/regras-de-negocio-fase-2.md#rn-18-liquidação-antecipada-total)). |

## Erros

Todos os erros têm o formato:

```json
{ "erro": "mensagem em português" }
```

| HTTP | Quando | Exemplo de mensagem |
| --- | --- | --- |
| `400` | JSON malformado ou com tipo errado | `corpo da requisição inválido` |
| `400` | Regra de validação violada | `apenas operações com operacao.estado DEFINIDA podem ser registradas` |
| `400` | `:identificador` que não é número de operação nem id | `identificador da operação inválido, use o id ou o numeroOperacao (ex.: 1 ou OP-2026-0001)` |
| `400` | Data vazia, inexistente ou fora do formato | `dataReferencia deve estar no formato YYYY-MM-DD` |
| `400` | Data anterior à última movimentação financeira | `dataReferencia não pode ser anterior à data da última movimentação financeira da operação` |
| `404` | Operação não existe | `operação não encontrada` |
| `404` | Rota que não existe | `rota não encontrada` |
| `405` | Método não aceito numa rota que existe (ex.: `DELETE /operacoes/1`). O header `Allow` traz os métodos aceitos. | `método não permitido nesta rota` |

Mensagens de validação do `POST /operacoes`:

| Mensagem |
| --- |
| `operacao.identificador não pode ser vazio` |
| `operacao.identificador deve estar no formato OP-AAAA-NNNN` |
| `apenas operações com operacao.estado DEFINIDA podem ser registradas` |
| `operacao.definidoEm deve estar no formato YYYY-MM-DD HH:MM:SS ou ISO-8601 com fuso` |
| `cliente.tipoPessoa deve ser PF ou PJ` |
| `cliente.documento deve ter apenas dígitos` |
| `cliente.documento deve ter 11 dígitos para PF e 14 para PJ` |
| `produto.codigo` / `produto.nome` / `produto.modalidadeBcb` / `taxa.faixaRisco` `não pode ser vazio` |
| `produto.codigo` / `produto.nome` / `produto.modalidadeBcb` / `taxa.faixaRisco` `deve ter no máximo N caracteres` |
| `condicoes.valor deve ser maior que zero` |
| `condicoes.valor deve ter no máximo 2 casas decimais` |
| `condicoes.prazoMeses deve estar entre 1 e 420` |
| `condicoes.dataLiberacao deve estar no formato YYYY-MM-DD` |
| `taxa.taxaFinalMes` / `taxa.taxaFinalAno` / `cet.mensalPercentual` / `cet.anualPercentual` `é obrigatório` |
| `taxa.taxaFinalMes` / `taxa.taxaFinalAno` / `cet.mensalPercentual` / `cet.anualPercentual` `deve ser maior ou igual a zero` |
| `taxa.taxaFinalMes` / `taxa.taxaFinalAno` / `cet.mensalPercentual` / `cet.anualPercentual` `deve ter no máximo 4 casas decimais` |
| `taxa.taxaFinalMes deve ser o percentual mensal, ex.: 1.85 para 1,85% ao mês` |
| `sistema deve ser PRICE ou SAC` |
| `encargos.*` / `valorLiberado` / `resumo.*` `deve ser maior ou igual a zero` |
| `encargos.*` / `valorLiberado` / `resumo.*` `deve ter no máximo 2 casas decimais` |
| `encargos.iof.total deve ser diario + adicional` |
| `encargos.total deve ser iof.total + tarifaCadastro` |
| `valorLiberado deve ser condicoes.valor - encargos.total` |
| `valorLiberado deve ser maior que zero` |
| `calendario não pode ser vazio` |
| `calendario deve ter no máximo 420 parcelas` |
| `calendario deve ter uma parcela por mês de condicoes.prazoMeses` |
| `calendario: parcela na posição N deveria ter numero N` |
| `calendario: parcela N com vencimento fora do formato YYYY-MM-DD` |
| `calendario: parcela N deve vencer depois da anterior e da dataLiberacao` |
| `calendario: parcela N com valor negativo ou com mais de 2 casas decimais` |
| `calendario: parcela N com valor zerado` |
| `calendario: parcela N com valor diferente de amortizacao + juros` |
| `calendario: parcela N com saldoDevedor X, esperado Y` |
| `calendario: soma das amortizações X diferente de condicoes.valor Y` |
| `resumo.totalJuros deve ser a soma dos juros do calendario` |
| `resumo.totalPago deve ser a soma do valor das parcelas do calendario` |

> **Limitação conhecida:** falhas inesperadas do banco também são devolvidas como `400`, com a mensagem original do erro. Separá-las em `500` está previsto ([ADR-002](../adr/ADR-002-arquitetura-em-camadas.md)).

## Idempotência

O `POST /operacoes` é idempotente pelo `operacao.identificador`. Reenviar o mesmo identificador (por timeout, por exemplo) não cria outra operação: a resposta é `200` com a confirmação da operação já gravada. Dois envios simultâneos também resultam em uma única operação, garantida pela constraint `UNIQUE` do `numero_operacao`. Ver [ADR-009](../adr/ADR-009-operacao-a-partir-da-simulacao-de-juros.md).
