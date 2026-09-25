# ADR-002: Arquitetura em camadas com validação no service

## Status

Aceita

## Data

2026-09-24

## Contexto

O protótipo de referência (`crud-go`) usa um único pacote `main` com todos os arquivos juntos. Para um serviço que vai crescer por fases (pagamentos, atraso, amortização, renegociação), precisávamos de uma separação que deixasse claro onde cada regra mora.

## Opções consideradas

- Pacote único (`main`), como no protótipo.
- **Camadas em `internal/`: handler, service, repository, domain.**
- Arquitetura hexagonal completa, com portas e adaptadores por interface.

## Decisão

Adotamos camadas dentro de `internal/`, com o ponto de entrada em `cmd/api`:

| Camada | Responsabilidade |
| --- | --- |
| `cmd/api` | Ligação das dependências e das rotas. Não contém regra de negócio. |
| `internal/config` | Leitura da configuração (`PORT`, `DATABASE_URL`). |
| `internal/database` | Abertura e verificação da conexão com o PostgreSQL. |
| `internal/handler` | Traduz HTTP ↔ domínio: structs de request/response, bind do JSON, status HTTP. **Não valida regras.** |
| `internal/service` | **Toda a validação de entrada e as regras de negócio**, incluindo o cálculo do cronograma. |
| `internal/repository` | SQL. Não valida nada; só persiste e lê. |
| `internal/domain` | Tipos do domínio, constantes de status e erros de negócio compartilhados. |

Convenções:

- Erros de negócio são criados com `errors.New`, com mensagem em português direcionada a quem consome a API.
- O handler responde `404` para `domain.ErrOperacaoNaoEncontrada` (verificado com `errors.Is`) e `400` para os demais erros.
- As structs de request/response ficam no handler quando o formato do JSON difere do domínio (ex.: o payload aninhado da Decisão vira um `NovaOperacao` sem aninhamento).
- Os comentários no código explicam o **porquê**, não o quê.
- Não há interfaces entre as camadas enquanto houver uma única implementação de cada uma.

## Consequências

- Cada regra tem um único lugar óbvio: a validação fica no service, o SQL no repository e o HTTP no handler.
- O cálculo do cronograma é uma função pura no service e pode ser testado sem banco.
- Sem interfaces, os testes de service que precisam de banco dependem de um PostgreSQL real. Hoje os testes cobrem as funções puras (cálculo e validação). Interfaces podem ser introduzidas quando houver necessidade de mocks.
- Como o handler responde `400` para tudo o que não é "não encontrado", uma falha inesperada do banco também sai como `400`, com a mensagem original do erro. Separar erros de validação de erros internos (`500`) é uma melhoria prevista.
