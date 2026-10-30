<div align="center">

<img src="docs/assets/banner.svg" alt="finops: controle financeiro de operações" width="100%">

# Controle Financeiro de Operações

**Microsserviço que registra e acompanha as operações de crédito aprovadas na Esteira de Crédito:<br>parcelas, pagamentos, atrasos, amortização e liquidação.**

![Status](https://img.shields.io/badge/status-fase%202%20conclu%C3%ADda-16a34a?style=for-the-badge)
![Go](https://img.shields.io/badge/go-1.26-00ADD8?style=for-the-badge&logo=go&logoColor=white)
![PostgreSQL](https://img.shields.io/badge/postgresql-18-4169E1?style=for-the-badge&logo=postgresql&logoColor=white)
![OpenAPI](https://img.shields.io/badge/openapi-3.0-6BA539?style=for-the-badge&logo=openapiinitiative&logoColor=white)
![Docker](https://img.shields.io/badge/docker-compose-2496ED?style=for-the-badge&logo=docker&logoColor=white)
![Projeto acadêmico](https://img.shields.io/badge/projeto-acad%C3%AAmico-2563eb?style=for-the-badge)
![Conventional Commits](https://img.shields.io/badge/commits-conventional-f97316?style=for-the-badge)

[Veja funcionando](#veja-funcionando) · [Começo rápido](#começo-rápido) · [Endpoints](#7-endpoints) · [Contratos](docs/architecture/contracts/README.md) · [Regras de negócio](docs/requirements/regras-de-negocio-fase-2.md) · [Decisões](docs/architecture/adr/README.md)

</div>

---

## Sobre o projeto

O **financial-operations-service** é um projeto acadêmico da equipe **UNICSUL FinOps**. Ele é a **última etapa da Esteira de Crédito**: recebe do serviço de juros as operações já definidas com o cliente, com o calendário de parcelas pronto, registra a operação e, nas próximas fases, acompanha pagamentos, atrasos e liquidação.

```text
Dados do cliente
      ↓
Score Engine :8080      calcula o score e classifica o risco
      ↓ HTTP POST
Decisão :8081           aprova, reprova ou envia para análise manual
      ↓
Juros                   precifica, simula PRICE/SAC e fecha a operação com o cliente
      ↓ HTTP POST /operacoes (somente DEFINIDA)
Operações :8083         confere e registra a operação e o calendário   ← este serviço
      ↓
PostgreSQL (finops)
```

### O que já funciona

| Área | O que faz | Regras |
| --- | --- | --- |
| **Registro** | Recebe a operação do serviço de juros, com produto, taxas, encargos, CET e o calendário (**PRICE** ou **SAC**). Confere se o calendário fecha ao centavo e grava como veio. Reenviar o mesmo identificador não duplica. | [ADR-009](docs/architecture/adr/ADR-009-operacao-a-partir-da-simulacao-de-juros.md) |
| **Situação** | `ATIVA`, `EM_ATRASO` (1 a 90 dias) e `INADIMPLENTE` (a partir do 91º dia), sempre **calculados** para uma `dataReferencia`, sem gravar o atraso. | RN-09 a RN-12 |
| **Encargos** | Multa de 2% e juros de mora de 1% ao mês pro rata die, com o total para quitar de cada parcela. | RN-13 |
| **Pagamento** | Por parcela, parcial ou total, na ordem multa, mora, juros e amortização. Reenvio seguro pelo `identificadorPagamento`. | RN-14, RN-15 |
| **Amortização** | Antecipa parcelas pelo valor presente, as **últimas** (reduz o prazo) ou as **próximas** a vencer (menos desconto), com simulação antes de gravar. | RN-16, RN-17 |
| **Liquidação** | Valor para quitar a operação inteira numa data e quitação pelo valor exato. | RN-18, RN-19 |
| **Resumo e histórico** | Totais da operação e linha do tempo de eventos gravados, com atraso e inadimplência calculados. | RN-20, RN-21 |
| **Painel web** | Dashboard, lista, detalhe e uma área para cada ação, num HTML só embutido na API. | [Painel](#painel) |
| **Integração** | CORS liberado para fronts de outras origens e container para rodar sem Go. | [CORS](#cors) · [Container](#executar-com-container-sem-go-e-sem-postgresql) |

O dinheiro é sempre decimal exato (`NUMERIC` no banco e `shopspring/decimal` no Go), nunca ponto flutuante. O roadmap completo está em [Requisitos](docs/requirements/README.md).

## Veja funcionando

O painel vem embutido na API: suba o serviço e abra `http://localhost:8083/`.

<table>
  <tr>
    <td width="50%" align="center" valign="top">
      <a href="docs/assets/tela-dashboard.png"><img src="docs/assets/tela-dashboard.png" alt="Dashboard com totais, situação das operações e maiores atrasos"></a>
      <br><sub><b>Dashboard</b>: totais da carteira, operações por situação e os maiores atrasos</sub>
    </td>
    <td width="50%" align="center" valign="top">
      <a href="docs/assets/tela-operacoes.png"><img src="docs/assets/tela-operacoes.png" alt="Lista de operações com busca e filtro por situação"></a>
      <br><sub><b>Operações</b>: busca e filtro por situação, com atraso e saldo devedor</sub>
    </td>
  </tr>
  <tr>
    <td width="50%" align="center" valign="top">
      <a href="docs/assets/tela-resumo.png"><img src="docs/assets/tela-resumo.png" alt="Resumo financeiro de uma operação"></a>
      <br><sub><b>Resumo</b>: quanto falta para quitar, o que foi pago e como foi dividido</sub>
    </td>
    <td width="50%" align="center" valign="top">
      <a href="docs/assets/tela-historico.png"><img src="docs/assets/tela-historico.png" alt="Linha do tempo de eventos da operação"></a>
      <br><sub><b>Histórico</b>: eventos gravados e o atraso calculado na data</sub>
    </td>
  </tr>
  <tr>
    <td width="50%" align="center" valign="top">
      <a href="docs/assets/tela-pagamento.png"><img src="docs/assets/tela-pagamento.png" alt="Pagamento de parcela com a distribuição do dinheiro"></a>
      <br><sub><b>Pagamento</b>: mostra para onde foi cada centavo, multa, mora, juros e amortização</sub>
    </td>
    <td width="50%" align="center" valign="top">
      <a href="docs/assets/tela-amortizacao.png"><img src="docs/assets/tela-amortizacao.png" alt="Simulação de amortização nas próximas parcelas"></a>
      <br><sub><b>Amortização</b>: simulação nas próximas parcelas, antes de confirmar</sub>
    </td>
  </tr>
</table>

## Começo rápido

**Com Go e PostgreSQL** (o caminho normal de desenvolvimento, detalhado nas seções 2 a 5):

```powershell
git clone https://github.com/unicsul-finops/financial-operations-service.git
cd financial-operations-service
copy .env.example .env      # ajuste a senha do seu PostgreSQL em DATABASE_URL
go run ./cmd/api
```

**Só com Docker** (sem Go nem PostgreSQL, como na máquina do professor):

```powershell
docker compose up -d --build
```

Nos dois casos: painel em `http://localhost:8083/`, documentação em `http://localhost:8083/swagger`.


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
├── api/                      # openapi.yaml (Swagger), embutido no binário
├── cmd/api/                  # ponto de entrada: liga dependências e rotas
├── web/                      # painel (index.html), embutido no binário
├── internal/
│   ├── config/               # leitura de PORT e DATABASE_URL
│   ├── database/             # conexão com o PostgreSQL
│   ├── domain/               # tipos, status e erros de negócio
│   ├── handler/              # HTTP: request/response e status
│   ├── service/              # casos de uso, validação do payload e conferência do calendário
│   └── repository/           # SQL
├── migrations/               # schema do banco, aplicado em ordem
├── collections/              # coleção de testes (Postman e Bruno)
├── docs/                     # arquitetura, contratos, requisitos, guias e as imagens do README (docs/assets)
├── .env.example              # modelo de configuração
└── go.mod
```

### Por onde começar a ler

Uma requisição passa pelas camadas nesta ordem ([ADR-002](docs/architecture/adr/ADR-002-arquitetura-em-camadas.md)):

| Arquivo | O que tem |
| --- | --- |
| `cmd/api/main.go` | Todas as rotas, em `novoRouter` |
| `internal/handler/operacao_handler.go` | Formato do request e da response (JSON) e o status HTTP de cada caso |
| `internal/service/operacao_service.go` | Os casos de uso: criar (com o reenvio), buscar, listar |
| `internal/service/validacao.go` | As regras do payload do juros e a conversão das taxas e do fuso |
| `internal/service/calendario.go` | A conferência do calendário de parcelas |
| `internal/repository/operacao_repository.go` | O SQL |
| `internal/domain/` | Os tipos, os status e os erros que as camadas compartilham |

Para criar um endpoint novo: a rota em `novoRouter`, o handler, o caso de uso no service, a consulta no repository, a documentação no `api/openapi.yaml` (o teste falha se a rota não estiver documentada) e um pedido na coleção em `collections/`. Mudança no banco entra numa migration nova em `migrations/`, sem editar as que já existem.

---

## 2. Pré-requisitos

Verifique se a máquina possui:

- **Go 1.26.5** ou superior (conforme `go.mod`);
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
psql -U postgres -d finops -f migrations/0002_fase_2_pagamentos_e_eventos.sql
```

| Migration | O que faz |
| --- | --- |
| `0001_create_foundations.sql` | Cria as tabelas `operacao` e `parcela` |
| `0002_fase_2_pagamentos_e_eventos.sql` | Cria as tabelas `evento` e `lancamento`, para pagamentos, amortização, liquidação e histórico ([ADR-012](docs/architecture/adr/ADR-012-eventos-e-lancamentos.md)) |

> **Já tinha o banco de uma versão anterior** (de quando o payload vinha da Decisão)? As migrations recomeçaram do zero na integração com o juros ([ADR-010](docs/architecture/adr/ADR-010-reinicio-das-migrations.md)). Recrie o banco antes de aplicar a `0001` nova (as operações de teste antigas são perdidas):
>
> ```powershell
> psql -U postgres -c "DROP DATABASE finops;"
> psql -U postgres -c "CREATE DATABASE finops;"
> ```

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
| `CORS_ORIGINS` | não | `*` | Origens que podem chamar a API pelo navegador, separadas por vírgula. `*` libera todas; `off` desliga o CORS |

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
swagger em http://localhost:8083/swagger
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

Criar uma operação com o payload de exemplo do juros: abra `http://localhost:8083/swagger`, entre em `POST /operacoes`, clique em **Try it out**, escolha o exemplo **price** e clique em **Execute**.

A resposta só confirma o registro: `id`, `numeroOperacao` (`OP-2026-0001`), `status` e `criadoEm`. Executar de novo responde `200` com a mesma operação, sem duplicar.

Consultar a operação completa e as 12 parcelas (para o exemplo, 10.000,00 em 12x a 1,85% a.m., PRICE, a parcela é **936,91**):

```text
http://localhost:8083/operacoes/OP-2026-0001
http://localhost:8083/operacoes/OP-2026-0001/parcelas
```

Para testar tudo de uma vez, use a coleção em [`collections/`](collections). Ela está no formato do Postman e abre tanto no **Postman** (Import) quanto no **Bruno** (Import Collection → Postman Collection). Rode a coleção inteira, na ordem. O `baseUrl` padrão é `http://localhost:8083`; para outra máquina, importe também o `local.postman_environment.json` e troque o endereço.

---

## 7. Endpoints

| Método | Rota | Descrição |
| --- | --- | --- |
| `GET` | `/health` | Verifica se o serviço está no ar |
| `POST` | `/operacoes` | Registra a operação definida no juros, com o calendário |
| `GET` | `/operacoes` | Lista as operações, com a situação na `dataReferencia` |
| `GET` | `/operacoes/:identificador` | Situação da operação (`numeroOperacao`, ou o id interno) |
| `GET` | `/operacoes/:identificador/parcelas` | Cronograma de parcelas, com atraso, encargos e total para quitar |
| `POST` | `/operacoes/:identificador/parcelas/:numero/pagamentos` | Registra o pagamento de uma parcela (RN-14) |
| `GET` | `/operacoes/:identificador/amortizacoes/simulacao` | Simula a antecipação de parcelas (RN-16 e RN-17) |
| `POST` | `/operacoes/:identificador/amortizacoes` | Registra a antecipação de parcelas |
| `GET` | `/operacoes/:identificador/liquidacao` | Valor para quitar a operação na data (RN-18) |
| `POST` | `/operacoes/:identificador/liquidacao` | Quita a operação antes do prazo |
| `GET` | `/operacoes/:identificador/resumo` | Resumo financeiro (RN-20) |
| `GET` | `/operacoes/:identificador/historico` | Histórico de eventos (RN-21) |
| `GET` | `/` | Painel para acompanhar e operar as operações |
| `GET` | `/swagger` | Documentação interativa (Swagger UI) |
| `GET` | `/openapi.yaml` | Especificação OpenAPI 3 |

### Painel

Com a API rodando, abra o painel no navegador:

```text
http://localhost:8083/
```

É um arquivo só (`web/index.html`), embutido no binário, que funciona sem internet. Tem dashboard com os totais e as situações das operações, a lista com busca e filtro, o detalhe de cada operação (resumo, parcelas e histórico) e uma área para cada ação: pagar parcela, amortização antecipada, liquidação e registrar operação. A **data de referência** no topo vale para todas as consultas.

Para ver o serviço de um colega, abra o painel dele pelo IP da máquina (`http://IP:8083/`). O painel usa sempre a API da máquina que o serviu, então cada pessoa vê e mexe nas operações do próprio banco.

### CORS

A API aceita chamadas feitas pelo navegador a partir de outra origem, como o front de outro grupo apontando para o IP da sua máquina. Por padrão o CORS vem **liberado para qualquer origem** (`CORS_ORIGINS=*`), porque cada um sobe o serviço no próprio PC e não há autenticação. O painel em `/` não depende disso: ele é servido pela própria API.

Para restringir, liste as origens no `.env`:

```text
CORS_ORIGINS=http://192.168.0.20:3000,http://localhost:5173
```

`off` desliga o CORS. O preflight (`OPTIONS`) responde `204`, e o header `Location` das respostas `201` fica visível para o JavaScript.

### Swagger

Com a API rodando, abra:

```text
http://localhost:8083/swagger
```

A interface lista todos os endpoints, com schemas, exemplos de request e response e as mensagens de erro, e permite testar as requisições pelo botão **Try it out**. Outros grupos podem abrir pela rede com o IP da máquina deste serviço (`http://IP:8083/swagger`), e o **Try it out** usa esse mesmo endereço.

O spec fica em [`api/openapi.yaml`](api/openapi.yaml) e pode ser importado em ferramentas como Postman, Insomnia ou Bruno, ou usado para gerar clients.

O `POST /operacoes` recebe o payload do serviço de juros como ele é enviado. O exemplo completo está no Swagger (exemplos `price` e `sac` do `POST /operacoes`) e nos pedidos de criação da coleção em [`collections/`](collections).

- O `operacao.identificador` é **obrigatório** e vira o `numeroOperacao`. Reenviar o mesmo identificador responde `200`, sem gravar de novo.
- A resposta só confirma o registro (`id`, `numeroOperacao`, `status`, `criadoEm`); os dados completos ficam nos `GET`.
- O `calendario` é gravado como veio, depois de conferido: se as contas não fecharem, a resposta é `400`.
- O juros envia taxas e CET em **percentual** (`1.85`); a API devolve toda taxa em **fração** (`0.0185`).
- Data/hora sem fuso é interpretada como **GMT-03:00**; as respostas saem em UTC.

O contrato completo, com todas as regras de validação, respostas e mensagens de erro, está em [Contratos da API](docs/architecture/contracts/README.md).

---

## 8. Testes

```powershell
go test ./...
```

Os testes de integração (atraso, pagamento, amortização, liquidação, resumo e histórico) rodam contra o PostgreSQL quando a variável `FINOPS_TEST_DATABASE_URL` aponta para uma conexão de teste, por exemplo `go test ./cmd/api -v`. Cada teste cria um schema temporário exclusivo, aplica as migrations e o remove ao terminar, sem alterar as operações existentes; o usuário precisa de permissão para criar schemas. Sem a variável, só esses testes são pulados e os unitários continuam sem banco. Não publique a URL com senha.

Depois de iniciar a API, consulte `http://localhost:8083/operacoes/OP-2026-0001/parcelas?dataReferencia=2026-12-10`. Sem pagamentos no exemplo PRICE, a primeira parcela deve retornar `VENCIDA`, `diasEmAtraso: 10` e `valorEmAberto: 936.91`. Em `2026-11-30`, ela permanece `PENDENTE`, com zero dias. Os pedidos da pasta `Contrato #6` na coleção exercitam esses cenários; a operação deve estar sem movimentações para reproduzir os valores do exemplo.

Os testes ficam ao lado do código (`internal/service/*_test.go` e `cmd/api/main_test.go`) e cobrem a conferência do calendário recebido (ordem, vencimentos, soma das parcelas e saldo encadeado até zero, ao centavo), a conversão das taxas, a interpretação do fuso e todas as validações de entrada. Eles também validam o `openapi.yaml` (inclusive os exemplos contra os schemas), conferem se toda rota do router está documentada nele e se os erros de rota e de método saem em JSON. Só os testes de integração precisam de banco.

Para testar a API rodando, de ponta a ponta, use a coleção em `collections/` (seção 6).

Antes de abrir uma PR, rode também:

```powershell
gofmt -l .
go vet ./...
```

---

## 9. Execução em computadores diferentes

Os serviços da esteira podem rodar em máquinas diferentes. Exemplo:

```text
Máquina A — Juros          192.168.0.20
Máquina B — Operações      192.168.0.30:8083
```

Nesse caso, o juros deve enviar para:

```text
http://192.168.0.30:8083/operacoes
```

O IP na URL é o da máquina que **recebe** a requisição. Portanto, Juros → Operações usa o IP da máquina de **Operações**.

Na máquina deste serviço, libere a porta 8083 no firewall do Windows para conexões de entrada.

---

## 10. Problemas comuns

| Sintoma | Causa provável |
| --- | --- |
| `DATABASE_URL não configurada` | Falta o `.env`, ou a API foi rodada fora da raiz do projeto |
| `password authentication failed` | Senha errada no `DATABASE_URL` |
| `database "finops" does not exist` | Faltou o `CREATE DATABASE` da seção 3 |
| `relation "operacao" does not exist` | Migration não aplicada |
| `column "definido_em" does not exist` | Banco é de uma versão anterior: recrie (seção 3) |
| `connection refused` na porta 5432 | Serviço do PostgreSQL parado |
| Porta 8083 em uso | Outra instância da API já está rodando |

Mais detalhes e soluções em [Troubleshooting](docs/guides/troubleshooting.md).

---

## Documentação

| Documento | Conteúdo |
| --- | --- |
| [Requisitos](docs/requirements/README.md) | Features, fases e regras de negócio |
| [Regras de negócio da Fase 2](docs/requirements/regras-de-negocio-fase-2.md) | Pagamento, atraso, encargos, inadimplência, amortização, liquidação, resumo e histórico |
| [Contratos da API](docs/architecture/contracts/README.md) | Endpoints, payloads, validações e erros |
| [Guia de convenções](docs/architecture/contracts/guia-de-convencoes.md) | Regras comuns a todo endpoint e checklist da PR de contrato |
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
