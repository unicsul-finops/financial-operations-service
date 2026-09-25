# ADR-003: ID da operação gerado pelo próprio serviço

## Status

Aceita

## Data

2026-09-24

## Contexto

Na primeira versão do contrato com o grupo de Decisão, o payload trazia um `operacaoID` gerado fora deste serviço. Isso causou incompatibilidades entre Score e Decisão: cada lado gerava identificadores em formatos diferentes, e não havia garantia de unicidade.

## Opções consideradas

- Receber o `operacaoID` da Decisão e validar a unicidade aqui.
- **Gerar o ID neste serviço, no momento do registro.**

## Decisão

O payload de `POST /operacoes` **não** contém ID. O serviço gera um **UUID** (`gen_random_uuid()` no PostgreSQL), que é a chave primária da tabela `operacao` e o identificador devolvido na resposta do POST.

## Consequências

- A unicidade é garantida pelo banco, sem depender de outro grupo.
- O contrato com a Decisão ficou mais simples.
- **Não há idempotência:** se a Decisão reenviar o mesmo POST (por timeout, por exemplo), serão criadas duas operações. Na Fase 1 esse risco é aceito. Se virar problema, a solução prevista é combinar com a Decisão um header de chave de idempotência.
- O UUID não é amigável para humanos. Isso é tratado no [ADR-006](ADR-006-numero-operacao-legivel.md).
