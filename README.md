<div align="center">

# Controle Financeiro de Operações

**Microsserviço que registra e acompanha as operações de crédito aprovadas na Esteira de Crédito.**

![Status](https://img.shields.io/badge/status-fase%201%20conclu%C3%ADda-16a34a?style=for-the-badge)
![Go](https://img.shields.io/badge/go-1.26-00ADD8?style=for-the-badge&logo=go&logoColor=white)
![PostgreSQL](https://img.shields.io/badge/postgresql-18-4169E1?style=for-the-badge&logo=postgresql&logoColor=white)
![Projeto acadêmico](https://img.shields.io/badge/projeto-acad%C3%AAmico-2563eb?style=for-the-badge)
![Conventional Commits](https://img.shields.io/badge/commits-conventional-f97316?style=for-the-badge)

</div>

---

## Sobre o projeto

O **financial-operations-service** é um projeto acadêmico da equipe **UNICSUL FinOps**. Ele é a **última etapa da Esteira de Crédito**: recebe as operações já aprovadas pelo grupo de Decisão, registra as condições financeiras, gera o cronograma de parcelas e, nas próximas fases, acompanha pagamentos, atrasos e liquidação.

```text
Dados do cliente
      ↓
Score Engine :8080      calcula o score e classifica o risco
      ↓ HTTP POST
Decisão :8081           aprova, reprova ou envia para análise manual
      ↓ HTTP POST /operacoes (somente APROVADO)
Operações :8083         registra a operação e gera o cronograma   ← este serviço
      ↓
PostgreSQL (finops)
```

### O que já funciona (Fase 1)

- Recebe a operação aprovada, com as condições financeiras e o contexto do score.
- Gera o próprio identificador: `id` (UUID) e `numeroOperacao` legível (`OP-2026-000001`).
- Gera o cronograma de parcelas nos sistemas **PRICE** e **SAC**, com valores exatos ao centavo.
- Consulta a situação da operação e o cronograma, **pelo UUID ou pelo número da operação**.

O roadmap completo, com as fases seguintes, está em [Requisitos](docs/requirements/README.md).

---

## 1. Stack e estrutura

| Item | Tecnologia |
| --- | --- |
| Linguagem | Go 1.26 |
| HTTP | Gin |
| Banco | PostgreSQL (acesso com `database/sql` + `pgx`, sem ORM) |
| Dinheiro | `shopspring/decimal` + `NUMERIC` (nunca `float64`) |
| Configuração | Variáveis de ambiente / `.env` (`godotenv`) |

As escolhas estão justificadas nos [ADRs](docs/architecture/adr/README.md).

```text
financial-operations-service/
├── cmd/api/                  # ponto de entrada: liga dependências e rotas
├── internal/
│   ├── config/               # leitura de PORT e DATABASE_URL
│   ├── database/             # conexão com o PostgreSQL
│   ├── domain/               # tipos, status e erros de negócio
│   ├── handler/              # HTTP: request/response e status
│   ├── service/              # validações, regras e cálculo do cronograma
│   └── repository/           # SQL
├── migrations/               # schema do banco, aplicado em ordem
├── examples/                 # payloads de exemplo
├── docs/                     # arquitetura, contratos, requisitos e guias
├── .env.example              # modelo de configuração
└── go.mod
```

---

## 2. Pré-requisitos

Verifique se a máquina possui:

- **Go 1.26** ou superior;
- **PostgreSQL** instalado e rodando localmente (testado com a versão 18);
- **Git**.

Verificar o Go:

```powershell
go version
```

Verificar o PostgreSQL:

```powershell
psql --version
```

> No Windows, o `psql` normalmente **não** fica no PATH. Use o caminho completo da instalação, por exemplo `& "C:\Program Files\PostgreSQL\18\bin\psql.exe"`, em todos os comandos `psql` deste README. Outra opção é adicionar essa pasta `bin` ao PATH.

Verificar se o serviço do PostgreSQL está rodando (Windows):

```powershell
Get-Service postgresql*
```

O status deve ser `Running`.

---

## 3. Preparar o banco

Os passos desta seção são feitos **uma única vez** por máquina.

Crie o banco:

```powershell
psql -U postgres -c "CREATE DATABASE finops;"
```

Aplique as migrations **em ordem**, a partir da raiz do projeto:

```powershell
psql -U postgres -d finops -f migrations/0001_create_foundations.sql
psql -U postgres -d finops -f migrations/0002_add_numero_operacao.sql
```

| Migration | O que faz |
| --- | --- |
| `0001_create_foundations.sql` | Cria as tabelas `operacao` e `parcela` |
| `0002_add_numero_operacao.sql` | Adiciona `numero_operacao` (`OP-AAAA-NNNNNN`) com sequence |

Quando uma nova migration entrar no repositório, aplique **somente ela**, com o mesmo comando. Migrations já aplicadas nunca são editadas ([ADR-007](docs/architecture/adr/ADR-007-migrations-sql-versionadas.md)).

---

## 4. Configurar o ambiente

Na raiz do projeto, copie o modelo:

```powershell
Copy-Item .env.example .env
```

Edite o `.env` com o usuário e a senha do **seu** PostgreSQL:

```text
PORT=8083
DATABASE_URL=postgres://postgres:SUA_SENHA@localhost:5432/finops?sslmode=disable
```

| Variável | Obrigatória | Padrão | Descrição |
| --- | --- | --- | --- |
| `DATABASE_URL` | sim | — | Conexão com o PostgreSQL |
| `PORT` | não | `8083` | Porta HTTP da API |

> O `.env` está no `.gitignore` e **nunca** deve ser commitado.

---

## 5. Executar

A partir da **raiz do projeto**:

```powershell
go run ./cmd/api
```

> `go run .` não funciona: o pacote `main` fica em `cmd/api`. Rode sempre da raiz, porque é ali que o `.env` é procurado.

A saída deve terminar com:

```text
financial-operations-service ouvindo na porta 8083
```

Mantenha esse terminal aberto.

Para gerar um executável:

```powershell
go build -o bin/api.exe ./cmd/api
.\bin\api.exe
```

---

## 6. Verificar

Health check:

```text
http://localhost:8083/health
```

Resposta esperada:

```json
{ "status": "ok" }
```

Criar uma operação com o payload de exemplo (PowerShell):

```powershell
Invoke-RestMethod `
    -Uri "http://localhost:8083/operacoes" `
    -Method POST `
    -ContentType "application/json" `
    -InFile "examples/finops_operacao_create.json"
```

A resposta traz o `id`, o `numeroOperacao` e as 24 parcelas. Para o exemplo (15.000,00 em 24x a 1,99% a.m., PRICE), a parcela é **792,17**.

Consultar pelo número ou pelo id:

```text
http://localhost:8083/operacoes/OP-2026-000001
http://localhost:8083/operacoes/OP-2026-000001/parcelas
```

---

## 7. Endpoints

| Método | Rota | Descrição |
| --- | --- | --- |
| `GET` | `/health` | Verifica se o serviço está no ar |
| `POST` | `/operacoes` | Registra uma operação aprovada e gera o cronograma |
| `GET` | `/operacoes` | Lista as operações |
| `GET` | `/operacoes/:id` | Situação da operação (`:id` = UUID ou `numeroOperacao`) |
| `GET` | `/operacoes/:id/parcelas` | Cronograma de parcelas (`:id` = UUID ou `numeroOperacao`) |

Payload do `POST /operacoes`, combinado com o grupo de Decisão:

```json
{
  "clienteId": "12345678901",
  "tipoPessoa": "PF",
  "decisao": "APROVADO",
  "dataAprovacao": "2026-09-25",
  "condicoesFinanceiras": {
    "valorAprovado": 15000.00,
    "prazoMeses": 24,
    "quantidadeParcelas": 24,
    "taxaJuros": 0.0199,
    "sistemaAmortizacao": "PRICE",
    "primeiroVencimento": "2026-10-25"
  },
  "contextoScore": {
    "scoreFinal": 872,
    "faixaRisco": "EXCELENTE",
    "probabilidadeDefault": 0.0128,
    "modelo": "SCORE_PF v1.1.0",
    "motivo": "Score 872 atende ao mínimo para aprovação automática."
  }
}
```

- O payload **não** traz ID: o serviço gera.
- O payload **não** traz o calendário de parcelas: o serviço gera.
- `taxaJuros` é a taxa **mensal em decimal** (`0.0199` = 1,99% a.m.).

O contrato completo, com todas as regras de validação, respostas e mensagens de erro, está em [Contratos da API](docs/architecture/contracts/README.md).

---

## 8. Testes

```powershell
go test ./...
```

Os testes cobrem o cálculo do cronograma (PRICE, SAC, taxa zero, vencimentos em fim de mês e fechamento do saldo em zero, conferidos ao centavo) e todas as validações de entrada. Eles não precisam de banco.

Antes de abrir uma PR, rode também:

```powershell
gofmt -l .
go vet ./...
```

---

## 9. Execução em computadores diferentes

Os serviços da esteira podem rodar em máquinas diferentes. Exemplo:

```text
Máquina A — Decisão        192.168.0.10:8081
Máquina B — Operações      192.168.0.30:8083
```

Nesse caso, a Decisão deve enviar para:

```text
http://192.168.0.30:8083/operacoes
```

O IP na URL é o da máquina que **recebe** a requisição. Portanto, Decisão → Operações usa o IP da máquina de **Operações**.

Na máquina deste serviço, libere a porta 8083 no firewall do Windows para conexões de entrada.

---

## 10. Problemas comuns

| Sintoma | Causa provável |
| --- | --- |
| `DATABASE_URL não configurada` | Falta o `.env`, ou a API foi rodada fora da raiz do projeto |
| `password authentication failed` | Senha errada no `DATABASE_URL` |
| `database "finops" does not exist` | Faltou o `CREATE DATABASE` da seção 3 |
| `relation "operacao" does not exist` | Migrations não aplicadas |
| `column "numero_operacao" does not exist` | Falta aplicar a migration `0002` |
| `connection refused` na porta 5432 | Serviço do PostgreSQL parado |
| Porta 8083 em uso | Outra instância da API já está rodando |

Mais detalhes e soluções em [Troubleshooting](docs/guides/troubleshooting.md).

---

## Documentação

| Documento | Conteúdo |
| --- | --- |
| [Requisitos](docs/requirements/README.md) | Features, fases e regras de negócio |
| [Contratos da API](docs/architecture/contracts/README.md) | Endpoints, payloads, validações e erros |
| [Decisões arquiteturais](docs/architecture/adr/README.md) | ADRs: por que o projeto é como é |
| [Diagramas](docs/architecture/diagrams/README.md) | Contexto, camadas, fluxo e modelo de dados |
| [Troubleshooting](docs/guides/troubleshooting.md) | Problemas comuns e soluções |
| [Fluxo Git](docs/development/git-workflow.md) | Branches, commits e Pull Requests no dia a dia |
| [Governança do projeto](docs/development/project-governance.md) | Papéis, Kanban e regras da equipe |
| [Guia de contribuição](CONTRIBUTING.md) | Regras para contribuição |

---

## Fluxo de trabalho

```text
Issue → Ready → Branch → Commits → Pull Request → Revisão → Merge → Done
```

- As tarefas são registradas como **Issues** e acompanhadas pelo GitHub Project.
- Cada tarefa é desenvolvida em uma branch própria (ex.: `feature/12-registrar-pagamento`).
- Os commits seguem o padrão [Conventional Commits](https://www.conventionalcommits.org/pt-br/v1.0.0/) (ex.: `feat(pagamentos): registra pagamento de parcela`).
- Toda alteração passa por Pull Request e revisão antes de entrar na `main`.
- O merge é realizado pelo arquiteto responsável após a aprovação.

O passo a passo está em [Fluxo Git](docs/development/git-workflow.md).

## Gestão do projeto

| Status | Finalidade |
| --- | --- |
| `Backlog` | Tarefas registradas, ainda não priorizadas. |
| `Ready` | Tarefas detalhadas e prontas para desenvolvimento. |
| `In Progress` | Tarefas em desenvolvimento. |
| `In Review` | Alterações aguardando revisão. |
| `Done` | Alterações aprovadas e integradas. |

## Equipe

Projeto desenvolvido por uma equipe de cinco integrantes da **Universidade Cruzeiro do Sul**, com organização centralizada na [UNICSUL FinOps](https://github.com/unicsul-finops).

## Contribuição

Antes de iniciar uma tarefa, consulte o [Fluxo Git](docs/development/git-workflow.md) e confirme que existe uma Issue atribuída. Não são permitidos commits diretos na branch `main`.

---

<div align="center">

Desenvolvido pela equipe **UNICSUL FinOps**.

</div>
