# 01. Visão geral e glossário

## O que é este serviço

O **financial-operations-service** (apelido: **finops**, ou "Operações") é um microsserviço em Go que **acompanha a vida financeira de uma operação de crédito depois que ela foi contratada**. Ele é a última etapa da Esteira de Crédito, um sistema acadêmico dividido entre vários grupos da Universidade Cruzeiro do Sul:

```text
Score (:8080)  →  Decisão (:8081)  →  Juros  →  Operações (:8083)  ← este serviço
 risco do          aprova ou           precifica e     registra e acompanha
 cliente           reprova             fecha com o     parcelas, pagamentos,
                                       cliente         atrasos e quitação
```

Em uma frase: **o juros diz o que foi contratado; este serviço controla o que foi pago, o que está atrasado e quanto falta.**

## O que ele faz

| Responsabilidade | Exemplo | Regras |
| --- | --- | --- |
| Registrar a operação contratada, com o calendário de parcelas | Juros envia `OP-2026-0001`: R$ 10.000 em 12x PRICE a 1,85% a.m. | RN-01 a RN-08 |
| Dizer a situação numa data | Em 10/12/2026 a parcela 1 está 10 dias vencida; a operação está `EM_ATRASO` | RN-09 a RN-12 |
| Calcular encargos de atraso | Multa de 2% + mora de 1% a.m. pro rata die | RN-13 |
| Registrar pagamento de parcela | R$ 500 na parcela 1: primeiro multa, depois mora, juros e amortização | RN-14, RN-15 |
| Antecipar parcelas com desconto | Quitar as 3 últimas pelo valor presente | RN-16, RN-17 |
| Quitar a operação inteira | Valor para quitar hoje e quitação pelo valor exato | RN-18, RN-19 |
| Resumir e contar a história | Total pago, juros pagos, saldo; linha do tempo de eventos | RN-20, RN-21 |

## O que ele **não** faz

- **Não calcula taxa, parcela nem calendário.** Isso é do juros. Aqui o calendário é recebido e conferido ([ADR-009](../architecture/adr/ADR-009-operacao-a-partir-da-simulacao-de-juros.md)).
- **Não chama nenhum outro serviço.** Só responde ([Integrações](../architecture/integracoes.md)).
- **Não movimenta dinheiro de verdade.** Registra que um pagamento aconteceu; quem recebe o dinheiro é outro sistema.
- **Não renegocia** ([Requisitos não implementados](../requirements/nao-implementados.md)).

## Números para lembrar

| Item | Valor |
| --- | --- |
| Porta | `8083` |
| Exemplo de operação | `OP-2026-0001`: 10.000,00 em 12x PRICE, 1,85% a.m., parcela de **936,91**, 1º vencimento 30/11/2026 |
| Multa | 2% do valor em aberto, fixada no 1º dia de atraso |
| Mora | 1% ao mês, simples, por dia (÷ 30) |
| Inadimplência | Mais de **90 dias** de atraso em alguma parcela |
| Fuso | GMT-03:00 para datas de entrada; UTC nas respostas com hora |

---

## Glossário

### Termos financeiros

| Termo | Significado aqui |
| --- | --- |
| **Operação** | Um contrato de crédito: um cliente, um valor, um prazo, uma taxa e um calendário de parcelas. Identificada pelo `numeroOperacao`. |
| **Parcela** | Uma prestação do calendário. Tem `valor = juros + amortizacao`, um vencimento e o saldo devedor depois dela. |
| **Amortização** | A parte da parcela que reduz a dívida (o principal). |
| **Juros (remuneratórios)** | A parte da parcela que remunera o banco pelo período. |
| **PRICE** | Sistema de amortização com parcelas iguais: juros caem e amortização sobe ao longo do tempo. |
| **SAC** | Sistema de amortização constante: amortização igual, parcelas decrescentes. |
| **Saldo devedor** | O principal que ainda não foi pago, ou seja, a soma das amortizações em aberto (RN-15). Não inclui juros futuros nem encargos. |
| **Valor aprovado** | O valor emprestado (`condicoes.valor`). É o saldo devedor inicial. |
| **Valor liberado** | O que o cliente recebe de fato: valor aprovado − IOF − tarifa de cadastro. |
| **IOF** | Imposto sobre a operação, calculado pelo juros na contratação (diário + adicional). |
| **CET** | Custo Efetivo Total: taxa que inclui juros, IOF e tarifas. Informativo; não entra em cálculo aqui. |
| **Data de liberação** | O dia em que o dinheiro foi entregue ao cliente. Nenhuma movimentação pode ser anterior a ela. |
| **Vencida** | Parcela não paga por inteiro depois do dia do vencimento. Pagar no próprio dia não é atraso (RN-10). |
| **Dias em atraso** | Dias corridos entre o vencimento e a data de referência. Na operação, é o maior atraso entre as parcelas em aberto. |
| **Multa** | 2% sobre o valor em aberto no 1º dia de atraso. Limite do CDC, art. 52. |
| **Juros de mora** | 1% ao mês, simples, pro rata die, sobre o valor em aberto de cada dia. Limite da Súmula 379 do STJ. |
| **Encargos** | Multa + juros de mora. Quando a palavra aparece nos dados do juros (`encargos`), significa IOF + tarifa da contratação: cuidado com o contexto. |
| **Total para quitar (parcela)** | Valor em aberto da parcela + encargos em aberto, numa data. |
| **Valor presente** | Quanto vale hoje uma parcela futura, descontada pela taxa do contrato (RN-16). |
| **Amortização antecipada** | Pagar parcelas futuras antes do vencimento, pelo valor presente. Modo `ULTIMAS` (reduz o prazo, mais desconto) ou `PROXIMAS` (as primeiras em aberto, menos desconto) (RN-17). |
| **Liquidação antecipada** | Quitar a operação inteira antes do fim: vencidas com encargos + a vencer pelo valor presente (RN-18). |
| **Inadimplente** | Operação com alguma parcela com mais de 90 dias de atraso (Res. CMN 4.966/2021) (RN-12). |

### Termos do código

| Termo | Significado |
| --- | --- |
| `numeroOperacao` | O identificador do juros (`OP-AAAA-NNNN`), igual em toda a esteira. É como as rotas identificam a operação. |
| `id` | Chave técnica sequencial do banco. Aceita nas rotas, mas só tem sentido aqui ([ADR-011](../architecture/adr/ADR-011-id-sequencial.md)). |
| `dataReferencia` | O dia para o qual uma consulta é calculada (RN-09). Padrão: hoje. É o que permite "viajar no tempo" nos testes. |
| `dataPagamento` | O dia em que o dinheiro entrou, informado por quem chama. |
| **Carteira** (`domain.Carteira`) | Retrato de uma operação lido do banco: operação, parcelas e todos os lançamentos. Entrada de todos os cálculos. |
| **Evento** | Um fato gravado da operação: `OPERACAO_REGISTRADA`, `PAGAMENTO`, `AMORTIZACAO_ANTECIPADA`, `LIQUIDACAO`. Nunca é alterado. |
| **Lançamento** | Para onde foi o dinheiro de um evento, por parcela: multa, mora, juros, amortização e desconto. |
| **Distribuição** | Os quatro componentes de um valor pago, na ordem multa → mora → juros → amortização. |
| **Movimentação** | Qualquer evento que mexe em dinheiro (pagamento, amortização, liquidação). Passa sempre pelo `Movimentar` do repository. |
| `decidir` | Função pura do service que recebe a carteira travada e devolve o `NovoEvento` a gravar ([ADR-014](../architecture/adr/ADR-014-concorrencia-e-idempotencia-das-movimentacoes.md)). |
| **Reenvio** | Chamada repetida com o mesmo identificador. Devolve `200` e o que já estava gravado. |
| **Gravado × calculado** | Gravado está numa coluna; calculado é refeito em toda consulta (atraso, encargos, resumo). Veja [Dados](06-dados-e-consistencia.md). |
| **RN-NN** | Regra de negócio numerada. RN-01 a RN-08 em [Requisitos](../requirements/README.md); RN-09 a RN-21 em [Regras da Fase 2](../requirements/regras-de-negocio-fase-2.md). |
| **ADR-NNN** | Registro de decisão arquitetural ([índice](../architecture/adr/README.md)). |
