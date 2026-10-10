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

A migration não foi aplicada nesse banco. Aplique a `0001_create_foundations.sql` (ver o [README](../../README.md#3-preparar-o-banco)).

### `column "definido_em" does not exist` (ou outra coluna nova)

O banco é de uma versão anterior à integração com o juros. As migrations recomeçaram do zero ([ADR-010](../architecture/adr/ADR-010-reinicio-das-migrations.md)), então recrie o banco e aplique a `0001` nova:

```powershell
psql -U postgres -c "DROP DATABASE finops;"
psql -U postgres -c "CREATE DATABASE finops;"
psql -U postgres -d finops -f migrations/0001_create_foundations.sql
```

As operações de teste antigas são perdidas.

### `400 corpo da requisição inválido`

O JSON está malformado ou algum campo veio com o tipo errado. Exemplos: número entre aspas em `condicoes.prazoMeses`, ou vírgula sobrando no final do objeto. Compare com o exemplo `price` do `POST /operacoes` no Swagger (`http://localhost:8083/swagger`) ou com o corpo do "Criar operação (PRICE)" da coleção em [`collections/`](../../collections).

### `400 calendario: parcela N ...`

O calendário enviado pelo juros não fecha: parcela fora de ordem, `valor` diferente de `amortizacao + juros`, ou `saldoDevedor` que não bate com o saldo anterior menos a amortização. A mensagem diz qual parcela e qual conta. O calendário é gravado como veio, então o ajuste precisa ser feito no serviço de juros. Ver [ADR-009](../architecture/adr/ADR-009-operacao-a-partir-da-simulacao-de-juros.md).

### `400 taxa.taxaFinalMes deve ser o percentual mensal...`

A taxa veio com 100 ou mais, o que indica erro de unidade. O juros envia percentual: `1.85` para 1,85% ao mês.

### `400 identificador da operação inválido...`

O identificador da URL não é um número de operação no formato `OP-AAAA-NNNN` nem um id interno (inteiro, ex.: `1`). Confira se não há espaços ou caracteres a mais na URL.

### `404 operação não encontrada`

O formato do identificador está certo, mas a operação não existe **nesse banco**. Confira se a API está apontando para o banco em que a operação foi criada (`DATABASE_URL`).

### As taxas saem diferentes do que o juros enviou (`0.0185` em vez de `1.85`)

É o esperado: o juros envia percentual e a API devolve toda taxa em fração. `0.0185` é 1,85%.

### O `definidoEm` sai 3 horas depois do enviado

Também é esperado. O juros envia sem fuso, o valor é interpretado como GMT-03:00 e a resposta sai em UTC: `2026-10-08 15:12:40` vira `2026-10-08T18:12:40Z`.

### Os valores saem sem o zero final (`185` em vez de `185.00`)

É o comportamento esperado: é o mesmo número. O JSON não preserva zeros à direita. Ver [ADR-005](../architecture/adr/ADR-005-valores-monetarios-decimal.md).

## Integração com o juros

### O juros não consegue enviar para este serviço

1. Confirme que a API está no ar: `http://localhost:8083/health` deve responder `{"status":"ok"}`.
2. Se estiverem em máquinas diferentes, o juros deve usar o **IP da máquina deste serviço**, por exemplo `http://192.168.0.30:8083`, e não `localhost`.
3. Verifique se o firewall do Windows permite conexões de entrada na porta 8083.

### O reenvio respondeu `200` em vez de `201`

É o esperado. O `operacao.identificador` já estava registrado, então nada foi gravado de novo e a resposta é a operação existente. Para criar outra operação, o identificador precisa ser outro.
