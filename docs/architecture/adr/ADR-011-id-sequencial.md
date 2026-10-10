# ADR-011: ID da operação sequencial, no lugar do UUID

## Status

Aceita

> Substitui o formato do ID definido no [ADR-003](ADR-003-id-gerado-pelo-servico.md). O restante do ADR-003 (o ID é gerado por este serviço, no registro) continua valendo.

## Data

2026-10-10

## Contexto

O [ADR-003](ADR-003-id-gerado-pelo-servico.md) definiu o `id` da operação como UUID (`gen_random_uuid()`). Com a integração com o serviço de juros ([ADR-009](ADR-009-operacao-a-partir-da-simulacao-de-juros.md)):

- o juros usa um `id` inteiro sequencial (`"id": 1`), e os dois serviços ficavam com formatos diferentes para o mesmo papel de chave técnica;
- o número que identifica a operação na esteira passou a ser o `numeroOperacao` (o identificador do juros), então o `id` não precisa mais ser único entre sistemas, só dentro deste banco.

O UUID tinha sido escolhido por não depender de coordenação. Como o ID é gerado só pelo PostgreSQL deste serviço, uma sequência do banco já garante a unicidade.

## Opções consideradas

- Manter o UUID.
- **Inteiro sequencial gerado pelo PostgreSQL (`BIGINT GENERATED ALWAYS AS IDENTITY`).**

## Decisão

- O `id` de `operacao` e de `parcela` é `BIGINT GENERATED ALWAYS AS IDENTITY`, começando em `1`. O `GENERATED ALWAYS` impede que um `INSERT` informe o ID por engano.
- Na API o `id` é um número JSON (`"id": 1`).
- As consultas (`GET /operacoes/:identificador` e `/parcelas`) são feitas normalmente pelo `numeroOperacao` (`OP-AAAA-NNNN`), que é como a esteira conhece a operação. O `id` (só dígitos, maior que zero) também é aceito; os formatos não se confundem, então o service decide pelo formato, como já fazia com o UUID.
- A mudança entrou na mesma `0001` do reinício das migrations ([ADR-010](ADR-010-reinicio-das-migrations.md)), que ainda não tinha ido para a `main`.

## Consequências

- `id` e `numeroOperacao` ficam legíveis e no mesmo estilo do serviço de juros.
- O `id` deste serviço e o `id` do juros são **independentes**: a operação `1` daqui não é necessariamente a `1` de lá. Para cruzar dados entre os serviços, o campo certo é sempre o `numeroOperacao`.
- **Pode haver buracos:** um ID consumido por um `INSERT` que falhou não é reaproveitado. Para chave técnica isso não importa.
- IDs sequenciais revelam a ordem e o volume de operações e são fáceis de adivinhar. Sem autenticação nesta fase, o UUID também não protegeria nada; se a API ficar exposta além da esteira, isso precisa ser tratado com autenticação, não com o formato do ID.
