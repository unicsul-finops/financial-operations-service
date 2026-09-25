# ADR-007: Migrations SQL versionadas e não destrutivas

## Status

Aceita

## Data

2026-09-24

## Contexto

Sem ORM ([ADR-001](ADR-001-stack-go-gin-postgres.md)), o schema do banco precisa ser versionado junto com o código, e cada integrante precisa conseguir reproduzir o mesmo banco localmente, com o PostgreSQL instalado na própria máquina.

## Opções consideradas

- Ferramenta de migrations (golang-migrate, goose) executada pela aplicação.
- Schema aplicado automaticamente na inicialização da API.
- **Arquivos `.sql` numerados em `migrations/`, aplicados manualmente com `psql`, em ordem.**

## Decisão

- As migrations ficam em `migrations/NNNN_descricao.sql` e são aplicadas **em ordem numérica** com `psql`.
- **Uma migration já aplicada nunca é editada.** Mudanças de schema entram em um arquivo novo (ex.: a `0002` adicionou `numero_operacao` sem tocar na `0001`).
- As migrations **não são destrutivas**: nada de `DROP` de colunas ou tabelas com dados. Quando há backfill, a migration roda dentro de `BEGIN`/`COMMIT`, para não ficar pela metade.
- A aplicação **não** aplica migrations ao iniciar.

## Consequências

- O processo é simples, sem ferramenta extra: basta o `psql` que já vem com o PostgreSQL.
- O histórico do schema fica legível no próprio repositório.
- Não há controle automático de quais migrations já foram aplicadas; cada pessoa precisa aplicar as novas manualmente. Se o número de migrations crescer, vale reavaliar uma ferramenta como golang-migrate (e registrar em um novo ADR).
