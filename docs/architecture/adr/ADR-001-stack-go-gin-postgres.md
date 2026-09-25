# ADR-001: Stack — Go, Gin, database/sql com pgx e PostgreSQL

## Status

Aceita

## Data

2026-09-24

## Contexto

O serviço de Operações Financeiras é a última etapa da Esteira de Crédito (Score → Decisão → Operações). Precisávamos escolher linguagem, framework HTTP, forma de acesso ao banco e banco de dados.

Outro grupo do mesmo domínio já tinha um protótipo funcional (`crud-go`) em Go + Gin + `database/sql` + PostgreSQL. Aproveitar a mesma stack reduz a curva de aprendizado e permite reaproveitar padrões de código.

## Opções consideradas

- **Go + Gin + database/sql (pgx) + PostgreSQL**
- Go + Gin + ORM (GORM)
- Java + Spring Boot + JPA
- Node.js + Express

## Decisão

Utilizaremos:

- **Go 1.26** como linguagem;
- **Gin** como framework HTTP;
- **`database/sql` com o driver `pgx/v5/stdlib`**, com SQL escrito à mão, sem ORM;
- **PostgreSQL** como banco de dados;
- **godotenv** para carregar a configuração de um arquivo `.env` em desenvolvimento.

O SQL explícito foi preferido a um ORM porque as regras financeiras dependem de tipos exatos (`NUMERIC`), transações e `RETURNING`, e o SQL visível facilita a revisão.

## Consequências

- Mesma stack do protótipo de referência, o que permite reaproveitar estilo e padrões.
- Binário único e leve, com inicialização rápida.
- Sem ORM, cada query e cada `Scan` são escritos manualmente. Há mais código repetitivo, mas nenhum comportamento escondido.
- Mudanças de schema exigem migrations SQL explícitas (ver [ADR-007](ADR-007-migrations-sql-versionadas.md)).
