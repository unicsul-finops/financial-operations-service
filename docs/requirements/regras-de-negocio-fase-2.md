# Regras de negócio da Fase 2

**Status:** Aceito pelo arquiteto em 10/10/2026.

Regras para o ciclo de vida da operação depois de registrada: pagamento, atraso, encargos, inadimplência, amortização e liquidação antecipada, resumo e histórico. Elas atendem o [Sprint Backlog](README.md#features-e-fases) com entrega até 09/11/2026.

Ninguém do grupo tem experiência no mercado de crédito, então cada regra foi tirada de **legislação e normas do Banco Central**, com a fonte indicada. Quando a norma deixa escolha, a opção adotada é a mais simples que continua correta, e a alternativa descartada fica registrada.

## Resumo das decisões

| # | Assunto | Decisão proposta |
| --- | --- | --- |
| RN-09 | Data de referência | Todo cálculo usa uma data de referência (dia, em GMT-03:00). Padrão: hoje. |
| RN-10 | Atraso | Parcela vence no dia seguinte ao vencimento sem pagamento total. Dias em atraso = dias corridos. |
| RN-11 | Situação calculada | `VENCIDA`, `EM_ATRASO` e `INADIMPLENTE` são calculados na consulta, não gravados. |
| RN-12 | Inadimplência | Operação `INADIMPLENTE` quando alguma parcela passa de **90 dias** de atraso. |
| RN-13 | Encargos por atraso | Multa de **2%** + juros de mora de **1% ao mês** pro rata die, sobre o valor em aberto da parcela. |
| RN-14 | Pagamento | Paga as parcelas da mais antiga para a mais nova; dentro da parcela: encargos → juros → amortização. |
| RN-15 | Saldo devedor | É o principal ainda não pago (soma das amortizações em aberto). |
| RN-16 | Valor presente | Desconto pela taxa do contrato, contado em períodos entre vencimentos, como o juros calcula. |
| RN-17 | Amortização antecipada | Quita as **últimas N parcelas** pelo valor presente (reduz o prazo). |
| RN-18 | Liquidação antecipada | Vencidas com encargos + a vencer pelo valor presente. Sem tarifa. Operação vira `LIQUIDADA`. |
| RN-19 | Encerramento | Pagar a última parcela em aberto também liquida a operação. |
| RN-20 | Resumo financeiro | Campos e fórmulas definidos na seção própria. |
| RN-21 | Histórico | Eventos gravados e imutáveis para tudo o que movimenta dinheiro. |

---

## RN-09: Data de referência

Todos os cálculos de atraso, encargos e valor presente dependem de **uma data**. Ela é sempre um dia (`YYYY-MM-DD`) em GMT-03:00, o mesmo fuso combinado com o juros ([ADR-009](../architecture/adr/ADR-009-operacao-a-partir-da-simulacao-de-juros.md)).

- **Pagamento, amortização e liquidação** informam a `dataPagamento`. Ela não pode ser futura nem anterior à `dataLiberacao`. Se não vier, vale hoje.
- **Consultas** (operação, parcelas, resumo, histórico, valor de quitação) aceitam `?dataReferencia=YYYY-MM-DD`. Sem o parâmetro, vale hoje.

**Por quê:** sem isso, testar um atraso de 91 dias exigiria esperar 91 dias. Com a data de referência, a coleção do Postman/Bruno simula qualquer cenário. Na vida real, a data do pagamento também vem de fora (o dia em que o dinheiro entrou), não do relógio do servidor.

## RN-10: Atraso

- Uma parcela está **vencida** quando a data de referência é **posterior** ao vencimento e ela não foi paga por inteiro. Pagar no dia do vencimento não é atraso.
- **Dias em atraso** = data de referência − vencimento, em dias corridos.
- Os vencimentos já chegam ajustados para dia útil pelo juros (30/01/2027, um sábado, vence em 01/02/2027). Por isso este serviço não precisa de calendário de feriados. A [Lei 7.089/1983](https://www2.camara.leg.br/legin/fed/lei/1980-1987/lei-7089-23-marco-1983-356734-publicacaooriginal-1-pl.html) proíbe cobrar mora quando o vencimento cai em dia não útil e o pagamento é feito no dia útil seguinte, e o calendário do juros já respeita isso.

Exemplo: a parcela 1 do `OP-2026-0001` vence em 30/11/2026. Na data de referência 10/12/2026, está **10 dias** em atraso.

## RN-11: Situação calculada na consulta

| Situação | Gravada no banco? | Regra |
| --- | --- | --- |
| Parcela `PENDENTE`, `PARCIALMENTE_PAGA`, `PAGA` | Sim | Muda só com pagamento. |
| Parcela `VENCIDA` | **Não**, calculada | Não paga por inteiro e data de referência posterior ao vencimento. |
| Operação `ATIVA`, `LIQUIDADA` | Sim | `LIQUIDADA` só por liquidação ou pela última parcela paga. |
| Operação `EM_ATRASO` | **Não**, calculada | Ao menos uma parcela vencida. |
| Operação `INADIMPLENTE` | **Não**, calculada | Ver RN-12. |

**Por quê:** situação de atraso muda com o passar do tempo, sem ninguém chamar a API. Gravar exigiria um job agendado rodando todo dia, e o projeto não tem nenhum. Se o job falhasse, o banco ficaria desatualizado. Calculada na hora, a situação nunca fica velha e funciona com a data de referência.

**Alternativa descartada:** job diário que grava as transições. Fica para quando houver necessidade de notificar outro sistema no momento exato da mudança.

**Consequência:** nas respostas, o `status` da operação e o das parcelas sai calculado para a data de referência. A coluna `status` do banco guarda só os estados que dependem de uma ação.

## RN-12: Inadimplência

- A operação fica **`INADIMPLENTE`** quando alguma parcela tem **mais de 90 dias** de atraso.
- Entre 1 e 90 dias, fica **`EM_ATRASO`**.
- Pagando as parcelas vencidas, a operação volta a `EM_ATRASO` ou `ATIVA`, conforme o que ainda faltar. Como a situação é calculada (RN-11), isso acontece sozinho.

**Por quê:** 90 dias é o corte usado pelo Banco Central. A Resolução CMN 4.966/2021 ([normativos do BCB](https://normativos.bcb.gov.br/)), em vigor desde 2025, trata como "ativo problemático" o crédito com atraso superior a 90 dias, e os indicadores de inadimplência divulgados pelo BC usam o mesmo corte.

Exemplo: a parcela 1 vence em 30/11/2026. Sem pagamento, a operação fica `EM_ATRASO` a partir de 01/12/2026 e `INADIMPLENTE` a partir de **01/03/2027** (91 dias).

## RN-13: Encargos por atraso

Sobre o **valor em aberto** da parcela vencida (valor da parcela menos o que já foi pago dela):

| Encargo | Regra | Fonte |
| --- | --- | --- |
| **Multa** | **2%**, cobrada uma vez, a partir do primeiro dia de atraso | [CDC, art. 52, §1º](https://www.planalto.gov.br/ccivil_03/leis/l8078compilado.htm): multa de mora não pode passar de 2% da prestação |
| **Juros de mora** | **1% ao mês, simples, pro rata die**: 1% × dias em atraso ÷ 30 | [Súmula 379 do STJ](https://stj.jus.br/publicacaoinstitucional/index.php/sumstj/article/download/5552/5675): em contratos bancários, até 1% ao mês |

- Os encargos **não** incidem uns sobre os outros (sem juros sobre multa) e não são capitalizados.
- Os valores são calculados para a data de referência e arredondados para centavos (meio para cima).
- Os encargos **pagos** ficam gravados no pagamento (RN-14 e RN-21). Os encargos **em aberto** são calculados na consulta.

**O que a norma permite e ficou de fora:** a [Resolução CMN 4.882/2020](https://normativos.bcb.gov.br/) permite cobrar, no atraso, **só** três coisas: juros remuneratórios do contrato por dia de atraso, multa e juros de mora. Proposta: **não cobrar os juros remuneratórios do atraso** nesta fase. Fica mais simples de calcular e de explicar, e é mais favorável ao cliente. Pode entrar depois sem mudar o resto.

Exemplo: a parcela 1 (936,91), paga em 10/12/2026 com 10 dias de atraso:

| Item | Cálculo | Valor |
| --- | --- | --- |
| Parcela | | 936,91 |
| Multa | 936,91 × 2% | 18,74 |
| Juros de mora | 936,91 × 1% × 10/30 | 3,12 |
| **Total devido** | | **958,77** |

## RN-14: Pagamento

`POST /operacoes/{identificador}/pagamentos` com `valor`, `dataPagamento` e, opcionalmente, `identificadorPagamento`.

- **`identificadorPagamento` (opcional):** quem registra o pagamento pode mandar um identificador próprio. Se um pagamento com o mesmo identificador já existir, nada é gravado de novo e a resposta é `200` com o pagamento já registrado, a mesma lógica de reenvio da operação. Sem o campo, o pagamento é sempre registrado. Ver a pergunta 1 em [O que confirmar com o serviço de juros](#o-que-confirmar-com-o-serviço-de-juros).

**Validações:**

- `valor` maior que zero, com até 2 casas decimais.
- A operação não pode estar `LIQUIDADA`.
- O `valor` não pode passar do que é devido **até a próxima parcela a vencer**: as vencidas com encargos, mais a próxima. Para pagar mais que isso, o caminho é a amortização antecipada (RN-17), que dá desconto. Pagamento normal não dá desconto.

**Ordem de abatimento (imputação):**

1. **Entre parcelas:** da mais antiga para a mais nova. O [Código Civil, art. 355](https://www.planalto.gov.br/ccivil_03/leis/2002/l10406compilada.htm) manda imputar primeiro nas dívidas vencidas.
2. **Dentro de cada parcela:** encargos (multa e mora) → juros → amortização. O [Código Civil, art. 354](https://www.planalto.gov.br/ccivil_03/leis/2002/l10406compilada.htm) manda abater primeiro os juros vencidos e depois o capital.

**Resultado:**

- Parcela paga por inteiro → `PAGA`. Paga em parte → `PARCIALMENTE_PAGA`.
- O saldo devedor da operação cai pela parte que foi para a **amortização** (RN-15).
- Tudo numa transação só, com evento no histórico (RN-21).

Exemplo: pagamento **parcial de 500,00** na parcela 1, em 10/12/2026 (10 dias de atraso):

| Destino | Valor |
| --- | --- |
| Encargos (18,74 + 3,12) | 21,86 |
| Juros da parcela | 185,00 |
| Amortização | 293,14 |
| **Total** | **500,00** |

A parcela fica `PARCIALMENTE_PAGA` e o saldo devedor cai de 10.000,00 para **9.706,86**.

## RN-15: Saldo devedor

- **Saldo devedor = principal ainda não pago**, ou seja, a soma das amortizações em aberto.
- Não inclui juros futuros, encargos nem tarifas: os encargos de registro (IOF, tarifa) foram descontados na liberação ([ADR-009](../architecture/adr/ADR-009-operacao-a-partir-da-simulacao-de-juros.md)).
- O valor para **quitar** a operação numa data é outro número (RN-18) e aparece separado no resumo.

## RN-16: Valor presente

Usado na amortização (RN-17) e na liquidação (RN-18), para descontar os juros que ainda não correram.

**Regra:** cada parcela a vencer é descontada pela **taxa mensal do contrato**, contando **períodos entre vencimentos**, com a fração proporcional do período em curso:

```text
VP = valor da parcela / (1 + i) ^ (k − f)
```

- `i`: taxa mensal do contrato, em fração (`0.0185`);
- `k`: posição da parcela entre as que ainda vão vencer (a próxima é 1, a seguinte é 2…);
- `f`: fração já decorrida do período atual = dias desde o vencimento anterior ÷ dias do período (para o primeiro período, conta a partir da `dataLiberacao`).

**Por quê:**

- O [CDC, art. 52, §2º](https://www.planalto.gov.br/ccivil_03/leis/l8078compilado.htm) garante a quitação antecipada, total ou parcial, com **redução proporcional dos juros**.
- A [Resolução CMN 3.516/2007](https://normativos.bcb.gov.br/Lists/Normativos/Attachments/48006/Res_3516_v3_P.pdf), com a redação dada pela Resolução CMN 4.320/2014, manda calcular o valor presente pela **taxa pactuada** e **proíbe tarifa** de liquidação antecipada.
- **Contar por períodos, e não por dias corridos ÷ 30**, é o que mantém a conta igual à do juros, que cobra 1,85% por período mesmo quando o período tem 31 ou 33 dias. Com dias ÷ 30, quitar no dia de um vencimento sairia **mais barato que o próprio saldo devedor** (8.470,16 contra 8.482,27 no exemplo), o que está errado. Com períodos, no dia do vencimento o valor presente das parcelas restantes é exatamente o saldo devedor.

## RN-17: Amortização antecipada (parcial)

`POST /operacoes/{identificador}/amortizacoes` com `quantidadeParcelas` e `dataPagamento`.

- Quita as **últimas N parcelas** do calendário, cada uma pelo seu valor presente (RN-16). O cliente **reduz o prazo**, e as parcelas que sobram continuam com o mesmo valor e o mesmo vencimento.
- **Só sem atraso:** se houver parcela vencida, o pedido é recusado (`400`) e o atraso precisa ser pago antes. Isso evita misturar desconto com encargos.
- Não pode quitar todas as parcelas. Para isso existe a liquidação (RN-18).
- As parcelas quitadas ficam `PAGA`. O saldo devedor cai pelo valor presente pago.
- **Repartição das parcelas restantes:** com o saldo menor, a divisão entre juros e amortização das parcelas que sobram muda. Ela é recalculada pela taxa do contrato (`juros = saldo × i`; `amortização = valor − juros`), mantendo valor e vencimento. A última parcela absorve os centavos, como no cálculo do juros.
- Um `GET /operacoes/{identificador}/amortizacoes/simulacao?quantidadeParcelas=N&dataReferencia=…` devolve o valor antes de o cliente decidir.

**Alternativa descartada:** manter o prazo e **reduzir o valor** das parcelas. Isso exige gerar um calendário novo com valores novos, ou seja, refazer o cálculo que é do serviço de juros ([ADR-009](../architecture/adr/ADR-009-operacao-a-partir-da-simulacao-de-juros.md)). Fica para quando houver acordo com eles.

Exemplo: em 30/12/2026, com as parcelas 1 e 2 pagas, o cliente antecipa as **2 últimas**:

| Parcela | Vencimento | Valor | Valor presente | Desconto |
| --- | --- | --- | --- | --- |
| 11 | 30/09/2027 | 936,91 | 794,42 | 142,49 |
| 12 | 01/11/2027 | 936,87 | 779,95 | 156,92 |
| **Total** | | **1.873,78** | **1.574,37** | **299,41** |

O saldo devedor cai de 8.482,27 para **6.907,90**. As parcelas 3 a 10 continuam com 936,91 cada, e a repartição recalculada termina com saldo **0,00** na parcela 10.

## RN-18: Liquidação antecipada (total)

- `GET /operacoes/{identificador}/liquidacao?dataReferencia=…` informa o **valor para quitar** na data.
- `POST /operacoes/{identificador}/liquidacao` com `dataPagamento` e `valor`. O valor precisa ser **igual** ao calculado para a data, o que evita quitar com um número velho.

**Valor para quitar** = parcelas vencidas em aberto **com encargos** (sem desconto) + parcelas a vencer pelo **valor presente** (RN-16). Sem tarifa ([Res. CMN 3.516/2007](https://normativos.bcb.gov.br/Lists/Normativos/Attachments/48006/Res_3516_v3_P.pdf)).

**Resultado:** todas as parcelas `PAGA`, saldo devedor `0,00`, operação **`LIQUIDADA`**, evento no histórico. Uma operação `LIQUIDADA` não aceita mais pagamento, amortização nem liquidação.

Exemplos com as parcelas 1 e 2 pagas em dia:

| Data | Fração do período | Valor para quitar | Soma nominal das parcelas restantes |
| --- | --- | --- | --- |
| 30/12/2026 (dia do vencimento da 2) | 0 | **8.482,27**, igual ao saldo devedor | 9.369,06 |
| 15/01/2027 (16 de 33 dias do período) | 0,4848 | **8.558,00** | 9.369,06 |

## RN-19: Encerramento automático

Quando um pagamento normal (RN-14) quita a **última parcela em aberto**, a operação também passa a `LIQUIDADA`, com o mesmo evento da liquidação. Atende a feature "Encerrar operação liquidada" do backlog.

## RN-20: Resumo financeiro

`GET /operacoes/{identificador}/resumo?dataReferencia=…`

| Campo | Definição |
| --- | --- |
| `totalContratado` | Valor da operação (`valorAprovado`). |
| `totalPrevisto` | Soma das parcelas do calendário original. |
| `totalPago` | Soma de tudo o que o cliente pagou (parcelas, encargos, amortizações, liquidação). |
| `valorAmortizado` | Parte do que foi pago que abateu principal. |
| `jurosPagos` | Parte do que foi pago que foi para juros remuneratórios. |
| `encargosPagos` | Multa + juros de mora pagos. |
| `descontoAntecipacao` | Desconto concedido em amortizações e liquidação (soma nominal − valor presente). |
| `valorVencido` | Valor em aberto das parcelas vencidas, sem encargos. |
| `encargosEmAberto` | Multa + mora das parcelas vencidas, na data de referência. |
| `saldoDevedor` | Principal em aberto (RN-15). |
| `valorParaQuitar` | Valor da liquidação na data de referência (RN-18). |

Conferência que vale sempre: `totalPago = valorAmortizado + jurosPagos + encargosPagos`.

## RN-21: Histórico de eventos

Tabela **só de inclusão**: evento gravado não é alterado nem apagado.

| Evento | Quando |
| --- | --- |
| `OPERACAO_REGISTRADA` | `POST /operacoes` cria a operação. |
| `PAGAMENTO` | Pagamento registrado (RN-14), com a distribuição por parcela: encargos, juros e amortização. |
| `AMORTIZACAO_ANTECIPADA` | Amortização (RN-17), com as parcelas quitadas, os valores presentes e o desconto. |
| `LIQUIDACAO` | Liquidação antecipada (RN-18) ou encerramento automático (RN-19). |

Cada evento grava: tipo, data do efeito (`dataPagamento`), momento do registro, valor e um detalhe em JSON com a distribuição.

Atraso e inadimplência **não** viram eventos gravados, porque a situação é calculada (RN-11). Na consulta do histórico, eles aparecem como entradas **calculadas**: "parcela 1 vencida em 01/12/2026", "operação inadimplente em 01/03/2027". Cada entrada indica `origem: CALCULADO` para não ser confundida com um evento gravado.

`GET /operacoes/{identificador}/historico?dataReferencia=…`, em ordem cronológica.

---

## Fora do escopo desta fase

| Item | Motivo |
| --- | --- |
| Juros remuneratórios durante o atraso | Permitidos pela Res. CMN 4.882/2020, mas opcionais (RN-13). |
| Reduzir o valor das parcelas na amortização | Exige recalcular o calendário do juros (RN-17). |
| IOF sobre atraso ou devolução de IOF na quitação | Regra tributária à parte; o IOF já é tratado pelo juros na contratação. |
| Calendário de feriados | Os vencimentos já chegam em dia útil. |
| Estorno de pagamento | Sem demanda no backlog. Se aparecer, entra como evento novo, sem apagar o original. |
| Renegociação | Depende de outro grupo, como já estava no backlog. |

## O que confirmar com o serviço de juros

1. **Algum sistema vai nos avisar dos pagamentos das parcelas, ou o pagamento é registrado manualmente pela nossa API?** Sabemos como a operação chega (o juros manda o `POST /operacoes`), mas não quem informa que uma parcela foi paga. Se for um sistema automático, ele pode reenviar por timeout, e o `identificadorPagamento` (RN-14) passa a ser obrigatório para não pagar duas vezes. Se for manual (Swagger ou coleção), ele continua opcional. As regras não mudam em nenhum dos casos.
2. **Recalcular a repartição na amortização (RN-17)** é aceitável para eles? Os valores e vencimentos das parcelas que sobram não mudam; só a divisão entre juros e amortização.
3. **Convenção de períodos (RN-16):** confirmar que eles cobram a taxa cheia por período, independentemente dos dias. O exemplo enviado indica que sim.

## Fontes

- [Código de Defesa do Consumidor (Lei 8.078/1990), art. 52](https://www.planalto.gov.br/ccivil_03/leis/l8078compilado.htm): multa de até 2% e liquidação antecipada com redução proporcional dos juros.
- [Código Civil (Lei 10.406/2002), arts. 354 e 355](https://www.planalto.gov.br/ccivil_03/leis/2002/l10406compilada.htm): ordem de imputação do pagamento.
- [Súmula 379 do STJ](https://stj.jus.br/publicacaoinstitucional/index.php/sumstj/article/download/5552/5675): juros moratórios de até 1% ao mês em contratos bancários.
- [Resolução CMN 3.516/2007](https://normativos.bcb.gov.br/Lists/Normativos/Attachments/48006/Res_3516_v3_P.pdf), com a redação da Res. CMN 4.320/2014: valor presente pela taxa pactuada e vedação de tarifa de liquidação antecipada.
- Resolução CMN 4.882/2020 ([normativos do BCB](https://normativos.bcb.gov.br/)): encargos permitidos no atraso (juros remuneratórios, multa e juros de mora). Resumo em [okai.com.br](https://okai.com.br/documento/2020-12-23/resolucao-cmn-n-4-882).
- Resolução CMN 4.966/2021 ([normativos do BCB](https://normativos.bcb.gov.br/), busque pelo número): ativo problemático com atraso superior a 90 dias.
- [Lei 7.089/1983](https://www2.camara.leg.br/legin/fed/lei/1980-1987/lei-7089-23-marco-1983-356734-publicacaooriginal-1-pl.html): sem mora quando o vencimento cai em dia não útil e o pagamento é feito no dia útil seguinte.

> As referências são para fundamentar decisões de um projeto acadêmico, não um parecer jurídico. Os textos das normas devem ser conferidos na fonte oficial antes de qualquer uso real.
