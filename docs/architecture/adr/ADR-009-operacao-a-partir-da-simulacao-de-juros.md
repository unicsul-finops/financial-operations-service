# ADR-009: Operação criada a partir da simulação do serviço de juros

## Status

Aceita

> Substitui o [ADR-004](ADR-004-cronograma-gerado-internamente.md) e o [ADR-006](ADR-006-numero-operacao-legivel.md), e resolve a falta de idempotência registrada nas consequências do [ADR-003](ADR-003-id-gerado-pelo-servico.md).

## Data

2026-10-10

## Contexto

O fluxo da esteira passou a ser **Score → Decisão → Juros → este serviço**. Quem nos chama agora é o serviço de juros, que envia a simulação que o cliente escolheu, com o calendário de parcelas já calculado (o payload completo está no exemplo `price` do `POST /operacoes` em [`api/openapi.yaml`](../../../api/openapi.yaml)).

Isso derruba a premissa do ADR-004, de que ninguém enviaria o calendário. O payload do juros também difere do contrato anterior (o da Decisão) em pontos que precisaram de regra:

- traz um `identificador` (`OP-2026-0001`) que rastreia a operação em toda a esteira;
- traz um `estado` (`DEFINIDA`) e o momento da definição (`definidoEm`), mas não a decisão nem a data de aprovação;
- traz as taxas e o CET em **percentual** (`taxaFinalMes: 1.85`);
- os vencimentos caem no **dia útil seguinte** (30/01/2027, sábado, vira 01/02/2027), regra diferente da que usávamos;
- as datas/horas vêm **sem fuso** (`"2026-10-08 15:12:40"`);
- traz dados que não guardávamos: produto, encargos (IOF e tarifa), valor liberado e CET;
- não traz mais o contexto do score.

Conferimos os exemplos recebidos (PRICE e SAC): `valor = amortizacao + juros` em todas as parcelas, o saldo encadeia até `0`, a soma das amortizações é o valor da operação e a soma das parcelas é o `totalPago`.

## Opções consideradas

Para o calendário:

- Continuar gerando o calendário aqui, a partir da taxa e do prazo enviados pelo juros.
- **Gravar o calendário recebido, validando a consistência sem recalcular.**

Para o número da operação:

- Manter a numeração própria (`OP-AAAA-NNNNNN`) e guardar o identificador do juros como referência externa.
- Usar o identificador do juros e gerar um número aqui quando ele não vier.
- **Usar o identificador do juros como `numeroOperacao`, obrigatório.**

Para a resposta do `POST`:

- Devolver a operação completa, com o cronograma.
- **Devolver só a confirmação do registro.**

Para um reenvio do mesmo identificador:

- Responder `409 Conflict`.
- **Responder `200` com a operação já gravada (idempotente).**

Para o fuso das datas/horas sem fuso:

- Exigir o fuso explícito no formato ISO (`2026-10-08T15:12:40-03:00`).
- **Assumir GMT-03:00 quando o fuso não vier**, por preferência do time de juros.

## Decisão

### Calendário recebido, não recalculado

O calendário é gravado **como veio**, na mesma transação que a operação. Gerar de novo foi descartado: a regra de vencimento do juros é diferente e qualquer diferença de centavo faria o contrato divergir do que o cliente viu na simulação. O cálculo dos juros de cada parcela **não** é refeito, porque a regra pertence ao serviço de juros.

Antes de gravar, `service.validarCalendario` confere, e recusa com `400` se algo não bater:

- uma parcela por mês de `condicoes.prazoMeses` (até 420), numeradas de `1` a `n`;
- vencimentos estritamente crescentes, o primeiro depois da `dataLiberacao`;
- valores sem fração de centavo e `valor = amortizacao + juros`;
- saldo encadeado a partir de `condicoes.valor`, terminando em `0`.

O `resumo` não é gravado, mas `totalJuros` e `totalPago` precisam bater com as somas do calendário. Os encargos também são conferidos: `iof.total = diario + adicional`, `encargos.total = iof.total + tarifaCadastro` e `valorLiberado = condicoes.valor − encargos.total`.

### Identificador e idempotência

- O `operacao.identificador` é **obrigatório** e vira o `numeroOperacao` (coluna `UNIQUE`). A sequence própria deixou de existir e este serviço não gera número. Gerar um número quando o juros não mandasse foi descartado: um número gerado aqui poderia coincidir com um que o juros emitisse depois (cada lado com seu contador), e a operação nova do juros seria tratada como reenvio e responderia `200` com a operação errada, sem erro aparente. Além disso, a operação não seria rastreável na esteira e um reenvio dela duplicaria. Identificador vazio é erro do juros e recebe `400`.
- O `id` técnico continua sendo gerado aqui e é a chave primária; o formato dele passou a ser sequencial ([ADR-011](ADR-011-id-sequencial.md)).
- Se o identificador já existe, nada é gravado e a resposta é `200`. Dois envios simultâneos são tratados pela constraint: o segundo cai na violação de `UNIQUE` e também recebe `200`.
- As consultas continuam aceitando o `id` ou o número, agora no formato `OP-AAAA-NNNN` (4 dígitos ou mais).

### Resposta do `POST`

O juros acabou de enviar todos os dados, então devolvê-los de volta não acrescenta nada. O `POST` responde só `id`, `numeroOperacao`, `status` e `criadoEm` (`201` com o header `Location` quando cria; `200` no reenvio). A operação completa e o cronograma ficam nos `GET`, que são a consulta do que este serviço passa a controlar (saldo devedor e status das parcelas, que mudam com os pagamentos).

### Mapeamento

| Juros | Operação |
| --- | --- |
| `operacao.identificador` | `numeroOperacao` |
| `operacao.estado` | só `DEFINIDA` é aceito; não é gravado |
| `operacao.definidoEm` | `definidoEm` (no lugar da antiga `dataAprovacao`) |
| `cliente.documento` / `cliente.tipoPessoa` | `clienteId` / `tipoPessoa` (11 dígitos para PF, 14 para PJ) |
| `produto.*` | `produto.*` |
| `condicoes.valor` | `valorAprovado` e `saldoDevedor` inicial |
| `condicoes.prazoMeses` | `prazoMeses` e `quantidadeParcelas` |
| `condicoes.dataLiberacao` | `dataLiberacao` |
| `taxa.taxaFinalMes ÷ 100` / `taxa.taxaFinalAno ÷ 100` | `taxaJuros` / `taxaJurosAnual` |
| `taxa.faixaRisco` / `taxa.tetoAplicado` | `faixaRisco` / `tetoAplicado` |
| `sistema` | `sistemaAmortizacao` |
| `encargos.*`, `valorLiberado` | `encargos.*`, `valorLiberado` |
| `cet.*Percentual ÷ 100` | `cet.mensal` / `cet.anual` |
| `calendario[0].vencimento` | `primeiroVencimento` |
| `calendario[].valor` | `parcelas[].valorParcela` |

Não são gravados: `operacao.id`, `operacao.criadoEm`, `condicoes.primeiroRelacionamento`, `resumo.parcelaFixa`, `demonstrativo` (derivável dos campos acima) e `calendario[].diasCorridos`. O contexto do score deixou de existir.

O `saldoDevedor` inicial é o valor da operação, e não o `totalDevido`: os encargos são descontados na liberação, não financiados.

### Taxas sempre em fração

O juros manda percentual; a conversão é feita num único ponto, na validação de entrada. No banco e nas respostas, **toda taxa fica em fração** (`0.0185` = 1,85%), como já era o `taxaJuros`. Uma taxa mensal de 100% ou mais é recusada, porque indica erro de unidade.

### Fuso horário

Uma data/hora recebida **sem fuso** é interpretada como **GMT-03:00 fixo**, e não "horário de Brasília": hoje os dois dão no mesmo, mas se o horário de verão voltar a regra continua sendo GMT-03:00. Se vier no ISO com fuso explícito, o fuso enviado é respeitado. Do nosso lado nada muda: o banco guarda `TIMESTAMPTZ` e as respostas saem em UTC (`...Z`).

## Consequências

- O serviço depende do juros para ter o calendário; em troca, o contrato gravado é exatamente o que o cliente viu na simulação.
- O `service.GerarCronograma` foi removido. Se uma renegociação precisar recalcular parcelas, ele está no histórico do Git, mas a regra de vencimento dele é diferente da do juros.
- Reenvios por timeout deixam de duplicar operações.
- O schema mudou o bastante para as migrations recomeçarem do zero ([ADR-010](ADR-010-reinicio-das-migrations.md)).
- Assumir GMT-03:00 sem fuso explícito é um acordo implícito: se o juros mudar o fuso sem avisar, os horários ficam errados sem nenhum erro aparente.
- Esquecer a conversão de percentual para fração multiplicaria as taxas por 100 sem erro aparente; por isso ela fica num ponto só e é coberta pelos testes.
