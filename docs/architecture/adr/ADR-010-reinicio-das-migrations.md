# ADR-010: Reinício das migrations na integração com o juros

## Status

Aceita

## Data

2026-10-10

## Contexto

Com o [ADR-009](ADR-009-operacao-a-partir-da-simulacao-de-juros.md), a operação passou a nascer do payload do serviço de juros. O schema anterior, de quando o payload vinha da Decisão, mudou quase inteiro:

- o número da operação deixou de vir da sequence `operacao_numero_seq` (migration `0002`) e passou a vir do juros;
- as colunas do contexto do score deixaram de ser usadas;
- a `data_aprovacao` foi substituída por `definido_em`;
- entraram produto, encargos, valor liberado, CET, taxa anual e faixa de preço, todos obrigatórios;
- o `id` das tabelas passou de UUID para inteiro sequencial ([ADR-011](ADR-011-id-sequencial.md)).

Seguindo à risca o [ADR-007](ADR-007-migrations-sql-versionadas.md) (nunca editar migration aplicada, nada destrutivo), seria preciso uma `0003` com colunas novas aceitando nulo (por causa das operações antigas), a sequence e as colunas de score ainda no banco sem uso, e o código tratando campos opcionais que nunca chegam vazios numa operação nova.

Não existe ambiente com dados reais: o projeto é acadêmico e os únicos bancos são os locais, de cada integrante, com operações de teste.

## Opções consideradas

- `0003` incremental, mantendo compatibilidade com as operações já gravadas.
- **Reiniciar as migrations uma única vez: uma `0001` nova com o schema final, e cada integrante recria o banco.**

## Decisão

- A `0001_create_foundations.sql` foi reescrita com o schema final e a `0002_add_numero_operacao.sql` foi removida. Fica um arquivo só.
- Quem já tinha o banco anterior recria o `finops` (`DROP DATABASE` + `CREATE DATABASE`) e aplica a `0001` nova. O passo a passo está no README.
- Este reinício é uma **exceção registrada**. A partir daqui, a regra do ADR-007 volta a valer integralmente: migration aplicada não é editada e mudanças de schema entram em arquivos novos, sem `DROP` de dados.

## Consequências

- Instalar o projeto do zero continua simples: um banco, um arquivo de migration.
- O código não carrega tipos opcionais nem colunas sem uso por causa de dados de teste antigos.
- As operações de teste antigas são perdidas ao recriar o banco. É aceito porque nenhuma delas é real.
- Quem esquecer de recriar o banco vai ver erro de coluna inexistente ao subir a API ou criar uma operação; o caso está no [guia de problemas comuns](../../guides/troubleshooting.md).
- Se o projeto passar a ter um ambiente com dados reais, um reinício como este deixa de ser opção.
