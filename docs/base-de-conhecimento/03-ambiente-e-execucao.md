# 03. Ambiente e execução

O passo a passo completo, com prints de saída esperada, está no [README](../../README.md#2-pré-requisitos). Aqui fica o resumo para quem já sabe o que está fazendo, e os detalhes que o README não cobre.

## Primeira vez na máquina

| O quê | Versão | Observação |
| --- | --- | --- |
| Go | 1.26.5 ou superior | Conforme o `go.mod` |
| PostgreSQL | 18 (testado) | No Windows, o `psql` normalmente não está no PATH: use `"C:\Program Files\PostgreSQL\18\bin\psql.exe"` |
| Git | qualquer recente | O repositório força fim de linha LF (`.gitattributes`) |
| Docker + Compose | opcional | Só para o caminho do container |

```powershell
git clone https://github.com/unicsul-finops/financial-operations-service.git
cd financial-operations-service
psql -U postgres -c "CREATE DATABASE finops;"
psql -U postgres -d finops -f migrations/0001_create_foundations.sql
psql -U postgres -d finops -f migrations/0002_fase_2_pagamentos_e_eventos.sql
Copy-Item .env.example .env   # ajuste a senha em DATABASE_URL
go run ./cmd/api
```

Pronto quando aparecer `financial-operations-service ouvindo na porta 8083`. Abra `http://localhost:8083/` (painel) e `http://localhost:8083/swagger`.

## Variáveis de ambiente

| Variável | Obrigatória | Padrão | Lida em |
| --- | --- | --- | --- |
| `DATABASE_URL` | sim | — | `internal/config` |
| `PORT` | não | `8083` | `internal/config` |
| `CORS_ORIGINS` | não | `*` (`off` desliga) | `internal/config` |
| `FINOPS_TEST_DATABASE_URL` | só para testes de integração | — | `cmd/api/*_integration_test.go` |
| `API_PORT`, `POSTGRES_PASSWORD` | só no container | `8083`, `finops` | `docker-compose.yml` |

O `.env` é lido da **pasta de onde o comando roda**. Rode sempre da raiz do repositório. Variáveis já definidas no ambiente têm prioridade sobre o `.env`.

## Comandos do dia a dia

| Para | Comando |
| --- | --- |
| Rodar a API | `go run ./cmd/api` |
| Gerar o executável | `go build -o bin/api.exe ./cmd/api` |
| Rodar todos os testes (os de integração são pulados sem banco) | `go test ./...` |
| Rodar os testes de integração | `$env:FINOPS_TEST_DATABASE_URL="postgres://postgres:SENHA@localhost:5432/finops?sslmode=disable"; go test ./cmd/api -v` |
| Um teste só | `go test ./internal/service -run TestEncargosNaConsulta -v` |
| Formatação (deve sair vazio) | `gofmt -l .` |
| Análise estática | `go vet ./...` |
| Ver o estado do banco | `psql -U postgres -d finops -c "SELECT numero_operacao, status, saldo_devedor FROM operacao;"` |
| Zerar os dados sem recriar o banco | `psql -U postgres -d finops -c "TRUNCATE lancamento, evento, parcela, operacao RESTART IDENTITY;"` |

## Testando cenários no tempo

Não espere 91 dias para ver uma inadimplência. Há duas regras de data, e elas são diferentes:

| Onde | Parâmetro | Pode ser futura? |
| --- | --- | --- |
| Consultas | `?dataReferencia=YYYY-MM-DD` | **Sim.** Serve para ver como a operação estará num dia qualquer |
| Pagamento, amortização, liquidação | `dataPagamento` no corpo | **Não.** É o dia em que o dinheiro entrou: no máximo hoje |

Então, para **ver** um atraso, basta consultar com uma data à frente:

```text
POST /operacoes                                                     (exemplo price do Swagger)
GET  /operacoes/OP-2026-0001/parcelas?dataReferencia=2026-11-30   → parcela 1 PENDENTE, 0 dias
GET  /operacoes/OP-2026-0001/parcelas?dataReferencia=2026-12-10   → parcela 1 VENCIDA, 10 dias, 936,91 em aberto
GET  /operacoes/OP-2026-0001?dataReferencia=2027-03-05            → INADIMPLENTE (mais de 90 dias)
```

Para **pagar** uma parcela atrasada, a operação precisa ter vencimentos no passado. A coleção faz isso registrando operações com as datas de 2025 (`dataLiberacao` 2025-10-30 e vencimentos a partir de 2025-11-30). Para montar o seu cenário, copie o exemplo `price`, troque o `identificador` e recue a `dataLiberacao` e todos os vencimentos; a conferência do calendário só exige vencimentos crescentes depois da liberação e valores que fechem.

**A data só anda para frente:** depois de um pagamento em 10/12, consultas e pagamentos com data anterior a 10/12 são recusados (RN-09). Para recomeçar um cenário, zere os dados ou use outro `identificador`.

## Coleções de testes

- **`collections/`** é a coleção **atual**, no formato do Postman, e abre também no Bruno (Import Collection → Postman Collection). Cobre todos os endpoints e os cenários de erro, organizados por história (Contrato #6, #7…). Rode inteira, na ordem.
- **`bruno/`** é a coleção nativa do Bruno da Fase 1. Cobre só registro, consultas e erros de validação.

## Container

```powershell
docker compose up -d --build     # sobe banco + API e aplica as migrations no primeiro start
docker compose logs -f api       # log da API
docker compose down              # para, mantém os dados
docker compose down -v           # para e apaga o banco
```

Detalhes no [diagrama de implantação](../architecture/diagrams/implantacao.md#cenário-2-container-entrega-para-quem-não-tem-go) e no [ADR-013](../architecture/adr/ADR-013-container-para-entrega.md). O container **não** é o ambiente de desenvolvimento: para iterar, rode nativo.

## Depurando

- **Log:** o Gin imprime uma linha por requisição (método, rota, status, tempo). Não há log de regra de negócio; para investigar um cálculo, escreva um teste unitário com a carteira do caso.
- **Debugger:** `dlv debug ./cmd/api` ou o "Run and Debug" do VS Code com a extensão Go. Lembre que o `.env` é lido da pasta de trabalho do processo.
- **Banco:** as consultas que o serviço faz estão todas em `internal/repository`, em SQL puro. Copie e rode no `psql` para ver o que o serviço vê.
- **Um `400` misterioso:** leia a mensagem no campo `erro`. Se for uma mensagem do PostgreSQL (em inglês), é uma falha interna que saiu como `400` ([limitação conhecida](../requirements/nao-implementados.md#3-requisitos-não-funcionais-não-atendidos)).

Erros comuns de ambiente (senha, banco que não existe, porta ocupada) estão no [Troubleshooting](../guides/troubleshooting.md).
