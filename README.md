<div align="center">

# Controle Financeiro de Operações

**Microsserviço para gerenciamento e acompanhamento de operações financeiras.**

![Status](https://img.shields.io/badge/status-em%20planejamento-7c3aed?style=for-the-badge)
![Projeto acadêmico](https://img.shields.io/badge/projeto-acad%C3%AAmico-2563eb?style=for-the-badge)
![Conventional Commits](https://img.shields.io/badge/commits-conventional-f97316?style=for-the-badge)

</div>

---

## Sobre o projeto

O **Controle Financeiro de Operações** é um projeto acadêmico desenvolvido pela equipe **UNICSUL FinOps**. A proposta é construir um microsserviço responsável por centralizar e controlar operações financeiras, aplicando boas práticas de arquitetura de software, colaboração e versionamento.

O projeto encontra-se na fase de levantamento de requisitos e definição arquitetural. Tecnologias, integrações e regras de negócio serão documentadas conforme forem decididas pela equipe.

## Objetivos

- Centralizar o registro de operações financeiras.
- Garantir organização, rastreabilidade e consistência das informações.
- Definir contratos claros para comunicação com outros sistemas.
- Aplicar práticas profissionais de arquitetura e desenvolvimento colaborativo.
- Manter decisões técnicas e documentação junto ao código.

## Status

> **Em planejamento:** requisitos, arquitetura e stack tecnológica ainda estão em definição.

| Área | Situação |
| --- | --- |
| Escopo e requisitos | Em definição |
| Arquitetura | Em definição |
| Stack tecnológica | Não definida |
| Contratos da API | Não definidos |
| Desenvolvimento | Não iniciado |

## Organização do repositório

```text
financial-operations-service/
├── .github/                   # Templates e configurações do GitHub
├── docs/
│   ├── architecture/
│   │   ├── adr/               # Registros de decisões arquiteturais
│   │   ├── contracts/         # Contratos de APIs e integrações
│   │   └── diagrams/          # Diagramas da solução
│   ├── development/           # Fluxo de trabalho e padrões da equipe
│   ├── guides/                # Guias técnicos e troubleshooting
│   └── requirements/          # Requisitos e regras de negócio
├── CONTRIBUTING.md            # Regras para contribuição
└── README.md                   # Visão geral do projeto
```

As pastas de código, testes e configurações serão adicionadas após a definição da stack.

## Fluxo de trabalho

```text
Issue → Ready → Branch → Commits → Pull Request → Revisão → Merge → Done
```

- As tarefas são registradas como **Issues** e acompanhadas pelo GitHub Project.
- Cada tarefa deve ser desenvolvida em uma branch própria.
- Os commits seguem o padrão [Conventional Commits](https://www.conventionalcommits.org/pt-br/v1.0.0/).
- Toda alteração deve passar por Pull Request e revisão antes de entrar na `main`.
- O merge é realizado pelo arquiteto responsável após a aprovação.

Exemplo de branch:

```text
feature/12-create-operation
```

Exemplo de commit:

```text
feat(operations): adiciona cadastro de operação
```

## Gestão do projeto

O trabalho é acompanhado em um Kanban com os seguintes status:

| Status | Finalidade |
| --- | --- |
| `Backlog` | Tarefas registradas, ainda não priorizadas. |
| `Ready` | Tarefas detalhadas e prontas para desenvolvimento. |
| `In Progress` | Tarefas em desenvolvimento. |
| `In Review` | Alterações aguardando revisão. |
| `Done` | Alterações aprovadas e integradas. |

## Documentação

- [Governança do projeto](docs/development/project-governance.md)
- [Fluxo Git](docs/development/git-workflow.md)
- [Requisitos](docs/requirements/README.md)
- [Decisões arquiteturais](docs/architecture/adr/README.md)
- [Diagramas](docs/architecture/diagrams/README.md)
- [Contratos](docs/architecture/contracts/README.md)
- [Guia de contribuição](CONTRIBUTING.md)

> Alguns documentos ainda estão em construção e serão atualizados durante a evolução do projeto.

## Equipe

Projeto desenvolvido por uma equipe de cinco integrantes da **Universidade Cruzeiro do Sul**, com organização centralizada na [UNICSUL FinOps](https://github.com/unicsul-finops).

## Contribuição

Antes de iniciar uma tarefa, consulte o `CONTRIBUTING.md` e confirme que existe uma Issue atribuída. Não são permitidos commits diretos na branch `main`.

---

<div align="center">

Desenvolvido pela equipe **UNICSUL FinOps**.

</div>
