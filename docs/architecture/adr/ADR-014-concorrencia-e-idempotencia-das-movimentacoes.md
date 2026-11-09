# ADR-014: Concorrência e idempotência das movimentações

## Status

Aceita

## Data

2026-10-30

## Contexto

Pagamento de parcela, amortização antecipada e liquidação (RN-14, RN-17 e RN-18) mudam o dinheiro da operação. Cada um precisa **ler** a situação atual (parcelas, lançamentos, saldo, encargos em aberto), **decidir** quanto vai para cada componente e **gravar** o evento, os lançamentos, os status das parcelas e o novo saldo ([ADR-012](ADR-012-eventos-e-lancamentos.md)).

Dois riscos apareceram ao dividir as histórias:

1. **Concorrência:** dois pagamentos da mesma operação ao mesmo tempo podem ler o mesmo saldo e gravar duas vezes a mesma amortização. O saldo fica errado e nenhum erro aparece.
2. **Reenvio:** quem chama (o app do cliente, por exemplo) pode repetir a requisição por timeout ou falha de rede. Sem proteção, o mesmo dinheiro é registrado duas vezes.

O registro da operação já resolvia o reenvio pelo identificador do juros ([ADR-009](ADR-009-operacao-a-partir-da-simulacao-de-juros.md)). Faltava o mesmo para as movimentações.

## Opções consideradas

Para a concorrência:

- Nada além do isolamento padrão (`READ COMMITTED`) e torcer para não acontecer.
- Trava otimista: uma coluna `versao` na operação e erro `409` quando ela mudou entre a leitura e a escrita.
- **Trava pessimista: `SELECT ... FOR UPDATE` na linha da operação, com a leitura, a decisão e a escrita numa transação só.**

Para o reenvio:

- Deixar o chamador cuidar disso.
- **Identificador opcional enviado pelo chamador (`identificadorPagamento`), único por operação, que devolve o que já foi gravado.**

Para organizar o código:

- O repository fazer as contas.
- O service abrir e fechar a transação.
- **O repository controla a transação e chama uma função `decidir` do service, que faz as contas sobre a carteira já travada.**

## Decisão

- **`Movimentar(id, numero, identificadorExterno, decidir)`** no repository abre a transação, lê a operação com `FOR UPDATE`, carrega a `Carteira` (parcelas e lançamentos) e:
  1. se o `identificadorExterno` já existe nessa operação, devolve o evento gravado sem chamar o `decidir` e sem gravar nada (`criado = false`);
  2. se não, chama `decidir(carteira)`, que é uma função **pura** do service e devolve um `NovoEvento`;
  3. grava o evento, os lançamentos, os status das parcelas, as parcelas recalculadas (RN-17) e o saldo, e faz o `COMMIT`.
- **`identificadorPagamento` é opcional**, até 50 caracteres, e vira `evento.identificador_externo`, com `UNIQUE (operacao_id, identificador_externo)` no banco. Sem ele, cada chamada é um pagamento novo.
- **Reenvio com o mesmo identificador para outra coisa** (outra parcela ou outro tipo de movimentação) responde `400` com `identificadorPagamento já foi usado em outra movimentação da operação`.
- **A resposta do reenvio é a mesma da primeira vez**, porque sai de `evento.detalhe` (JSONB), e o status é `200` em vez de `201`.
- As consultas de leitura (`ConsultarCarteira`, `ConsultarHistorico`) usam uma transação `REPEATABLE READ` somente leitura, para que parcelas e lançamentos venham do mesmo instante.

## Consequências

- Dois pagamentos simultâneos da mesma operação são executados **um depois do outro**. O segundo calcula sobre o saldo que o primeiro deixou. Operações diferentes não se bloqueiam.
- O chamador pode repetir a requisição com segurança sempre que mandar o `identificadorPagamento`.
- As regras ficam testáveis sem banco: `decidirPagamento`, `decidirAmortizacao` e `decidirLiquidacao` recebem uma `Carteira` e devolvem um `NovoEvento`, e os testes unitários usam carteiras montadas em memória.
- A trava dura só o tempo da transação, que é curta (algumas consultas e inserções). Num volume grande de pagamentos da mesma operação isso viraria fila, o que não é um cenário real para uma operação de crédito.
- Quem cria uma movimentação nova (um estorno, por exemplo) deve usar `Movimentar` com um `decidir` próprio, e não escrever direto nas tabelas.
