# Troubleshooting

Problemas comuns ao rodar o **financial-operations-service** localmente e como resolvê-los.

> Nos exemplos, `psql` é o executável que vem com o PostgreSQL. No Windows ele normalmente **não** está no PATH. Use o caminho completo, por exemplo `& "C:\Program Files\PostgreSQL\18\bin\psql.exe"`, ou adicione a pasta `bin` ao PATH.

## A API não sobe

### `DATABASE_URL não configurada`

A API não encontrou a variável `DATABASE_URL`.

- Confirme que existe um arquivo `.env` na **raiz do projeto** (copie do `.env.example`).
- Rode a API **a partir da raiz**: `go run ./cmd/api`. O `.env` é procurado na pasta de onde o comando é executado. Rodar de dentro de `cmd/api` não o encontra.

### `password authentication failed for user "postgres"`

A senha no `DATABASE_URL` está errada. Confira o usuário e a senha definidos na instalação do PostgreSQL:

```text
DATABASE_URL=postgres://USUARIO:SENHA@localhost:5432/finops?sslmode=disable
```

Se a senha tiver caracteres especiais (`@`, `:`, `/`, `#`, `%`), eles precisam ser codificados na URL. Exemplo: `@` vira `%40`.

### `database "finops" does not exist`

O banco ainda não foi criado:

```powershell
psql -U postgres -c "CREATE DATABASE finops;"
```

Depois aplique as migrations (ver o [README](../../README.md#3-preparar-o-banco)).

### `connect: connection refused` / `dial tcp [::1]:5432`

O PostgreSQL não está rodando ou não está na porta 5432.

- Windows: abra **Serviços** (`services.msc`) e verifique se o serviço `postgresql-x64-NN` está **Em execução**. Se não estiver, inicie-o. Pelo PowerShell como administrador: `Start-Service postgresql-x64-18`.
- Confirme a porta configurada na instalação e ajuste o `DATABASE_URL` se não for 5432.

### `bind: Only one usage of each socket address` / `address already in use`

Já existe um processo na porta 8083, geralmente outra instância da API esquecida num terminal. Para descobrir qual (PowerShell):

```powershell
Get-NetTCPConnection -LocalPort 8083 -State Listen | ForEach-Object { Get-Process -Id $_.OwningProcess }
```

Encerre esse processo, ou suba a API em outra porta com `PORT=8084` no `.env`. Lembre que os outros grupos esperam **8083**.

### `go: command not found` / módulos não baixam

- Verifique a instalação: `go version` (é necessário Go 1.26 ou superior).
- Rode `go mod download` na raiz do projeto.

## Erros nas requisições

### `relation "operacao" does not exist`

As migrations não foram aplicadas nesse banco. Aplique a `0001` e depois a `0002`.

### `column "numero_operacao" does not exist`

Só a migration `0001` foi aplicada. Aplique a `0002_add_numero_operacao.sql`.

### `relation "operacao_numero_seq" already exists` ao aplicar a `0002`

A `0002` já tinha sido aplicada. Como ela roda dentro de uma transação, a segunda tentativa não altera nada. Nenhuma ação necessária.

### `400 corpo da requisição inválido`

O JSON está malformado ou algum campo veio com o tipo errado. Exemplos: número entre aspas em `quantidadeParcelas`, ou vírgula sobrando no final do objeto. Compare com [`examples/finops_operacao_create.json`](../../examples/finops_operacao_create.json).

### `400 taxaJuros deve ser a taxa mensal em decimal...`

A taxa foi enviada em porcentagem. Envie `0.0199` para 1,99% ao mês, não `1.99`.

### `400 identificador da operação inválido...`

O `:id` da URL não é um UUID nem um número de operação no formato `OP-AAAA-NNNNNN`. Confira se não há espaços ou caracteres a mais na URL.

### `404 operação não encontrada`

O formato do identificador está certo, mas a operação não existe **nesse banco**. Confira se a API está apontando para o banco em que a operação foi criada (`DATABASE_URL`).

### Os valores saem sem o zero final (`298.5` em vez de `298.50`)

É o comportamento esperado: é o mesmo número. O JSON não preserva zeros à direita. Ver [ADR-005](../architecture/adr/ADR-005-valores-monetarios-decimal.md).

### A última parcela tem valor diferente das outras

Também é esperado. A última parcela absorve os centavos que sobram dos arredondamentos, para o saldo final ser exatamente zero. Ver [ADR-004](../architecture/adr/ADR-004-cronograma-gerado-internamente.md).

## Integração com a Decisão

### A Decisão não consegue enviar para este serviço

1. Confirme que a API está no ar: `http://localhost:8083/health` deve responder `{"status":"ok"}`.
2. Se estiverem em máquinas diferentes, a Decisão deve usar o **IP da máquina deste serviço**, por exemplo `http://192.168.0.30:8083`, e não `localhost`.
3. Verifique se o firewall do Windows permite conexões de entrada na porta 8083.

### Operação duplicada

O serviço não é idempotente: cada `POST /operacoes` cria uma nova operação. Se a Decisão reenviou a requisição (por timeout, por exemplo), haverá duas. Ver [ADR-003](../architecture/adr/ADR-003-id-gerado-pelo-servico.md).
