# Governança do Projeto

Este documento registra a organização, as responsabilidades e o fluxo de trabalho adotados no projeto **Controle Financeiro de Operações**.

## Estrutura no GitHub

- **Organization:** `unicsul-finops`
- **Repository:** `financial-operations-service`
- **GitHub Project:** `Controle Financeiro de Operações`
- **Branch principal:** `main`

O repositório concentra código, testes, requisitos, arquitetura, contratos e documentação do microsserviço.

## Papéis e responsabilidades

### Arquiteto e revisor

- Define e acompanha os padrões técnicos.
- Organiza e prioriza as Issues.
- Revisa todas as Pull Requests.
- Solicita correções quando necessário.
- Aprova e realiza o merge na `main`.
- Registra decisões arquiteturais importantes em ADRs.

### Desenvolvedores

- Trabalham a partir de uma Issue atribuída.
- Criam uma branch exclusiva para cada tarefa.
- Seguem os padrões de código e Conventional Commits.
- Testam as alterações localmente.
- Abrem Pull Requests com descrição e evidências.
- Corrigem os pontos identificados durante a revisão.

## Kanban

| Status | Significado |
| --- | --- |
| `Backlog` | Tarefa registrada, mas ainda não priorizada ou detalhada. |
| `Ready` | Tarefa definida e pronta para ser iniciada. |
| `In Progress` | Tarefa em desenvolvimento. |
| `In Review` | Pull Request aberta e aguardando revisão. |
| `Done` | Alteração aprovada e integrada à `main`. |

## Fluxo de trabalho

```text
Issue → Ready → Branch → Commits → Pull Request → Revisão → Merge → Done
```

1. O arquiteto cria, detalha e atribui a Issue.
2. A Issue é movida para `Ready`.
3. O desenvolvedor atualiza a `main` e cria uma branch.
4. Durante o desenvolvimento, a Issue fica em `In Progress`.
5. O desenvolvedor envia os commits e abre uma Pull Request.
6. A Issue e a PR passam para `In Review`.
7. O arquiteto revisa e solicita ajustes ou aprova.
8. O arquiteto realiza **Squash and merge**.
9. A Issue é encerrada e passa para `Done`.

## Padrão de branches

```text
tipo/numero-da-issue-descricao-curta
```

Exemplos:

```text
feature/12-create-operation
fix/27-operation-validation
docs/31-update-architecture
refactor/42-operation-service
```

As branches devem usar letras minúsculas, sem espaços ou acentos, com palavras separadas por hífen.

## Conventional Commits

Formato:

```text
tipo(escopo): descrição curta
```

Exemplos:

```text
feat(operations): adiciona cadastro de operação
fix(validation): corrige validação de valor negativo
docs(architecture): adiciona diagrama de contexto
refactor(service): simplifica cálculo financeiro
test(operations): adiciona testes de cadastro
chore: atualiza configurações do projeto
```

## Pull Requests

Toda alteração deve entrar na `main` por Pull Request. A descrição deve informar:

- O que foi feito.
- Qual Issue está relacionada.
- Como testar.
- Evidências, quando aplicável.

Para vincular e fechar automaticamente a Issue após o merge:

```text
Closes #12
```

Somente o arquiteto/revisor realiza o merge. Não são permitidos push direto ou force push na `main`.

## Decisões arquiteturais

Decisões técnicas relevantes devem ser registradas em `docs/architecture/adr/`, incluindo contexto, opções consideradas, decisão e consequências.

Exemplos:

- Escolha da linguagem e do framework.
- Escolha do banco de dados.
- Estratégia de autenticação.
- Modelo de comunicação e integração.

## Segurança

Nunca devem ser enviados ao repositório:

- Senhas, tokens ou chaves de API.
- Arquivos `.env` com valores reais.
- Certificados ou credenciais.
- Dependências e arquivos gerados que pertençam ao `.gitignore`.

