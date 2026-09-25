# Registros de Decisões Arquiteturais (ADR)

Esta pasta registra as decisões técnicas relevantes do **financial-operations-service**. Cada ADR descreve o contexto, as opções consideradas, a decisão e as consequências, para que qualquer integrante entenda **por que** o projeto é como é.

## Índice

| ADR | Título | Status |
| --- | --- | --- |
| [ADR-001](ADR-001-stack-go-gin-postgres.md) | Stack: Go, Gin, database/sql com pgx e PostgreSQL | Aceita |
| [ADR-002](ADR-002-arquitetura-em-camadas.md) | Arquitetura em camadas com validação no service | Aceita |
| [ADR-003](ADR-003-id-gerado-pelo-servico.md) | ID da operação gerado pelo próprio serviço | Aceita |
| [ADR-004](ADR-004-cronograma-gerado-internamente.md) | Cronograma de parcelas gerado internamente (PRICE e SAC) | Aceita |
| [ADR-005](ADR-005-valores-monetarios-decimal.md) | Valores monetários com decimal exato | Aceita |
| [ADR-006](ADR-006-numero-operacao-legivel.md) | Número de operação legível e busca por ambos os identificadores | Aceita |
| [ADR-007](ADR-007-migrations-sql-versionadas.md) | Migrations SQL versionadas e não destrutivas | Aceita |

## Quando escrever um ADR

Registre um ADR quando a decisão:

- afetar mais de uma camada ou o contrato com outros grupos;
- for difícil ou cara de reverter;
- tiver alternativas razoáveis que alguém poderia questionar depois.

## Como escrever

1. Copie o modelo abaixo para `ADR-NNN-titulo-curto.md`, com o próximo número disponível.
2. Abra a PR com o status `Proposta`. Após a aprovação do arquiteto, altere para `Aceita`.
3. Um ADR aceito **não é editado** para mudar a decisão. Se a decisão mudar, crie um novo ADR e marque o antigo como `Substituída por ADR-NNN`.
4. Adicione a linha correspondente no índice acima.

## Modelo

```markdown
# ADR-NNN: Título da decisão

## Status

Proposta | Aceita | Substituída por ADR-NNN

## Data

AAAA-MM-DD

## Contexto

Qual problema ou necessidade motivou a decisão.

## Opções consideradas

- Opção A
- Opção B

## Decisão

O que foi decidido e por quê.

## Consequências

- Efeitos positivos.
- Custos, riscos e limitações assumidos.
```
