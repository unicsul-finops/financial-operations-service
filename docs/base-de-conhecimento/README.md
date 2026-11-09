# Base de Conhecimento para desenvolvedores

Documentação para quem vai **manter, corrigir ou evoluir** o financial-operations-service. O [README](../../README.md) explica como rodar; esta base explica **como o projeto pensa**: onde fica cada coisa, por que foi feito assim, como mexer sem quebrar e o que costuma confundir quem chega.

> Se você só tem 15 minutos, leia nesta ordem: [Visão geral](01-visao-geral-e-glossario.md) → [Arquitetura e código](02-arquitetura-e-codigo.md) → [Armadilhas e FAQ](08-armadilhas-e-faq.md).

## Artigos

| # | Artigo | Responde a |
| --- | --- | --- |
| 01 | [Visão geral e glossário](01-visao-geral-e-glossario.md) | O que o serviço faz, onde fica na esteira, o que significa cada termo financeiro |
| 02 | [Arquitetura e mapa do código](02-arquitetura-e-codigo.md) | Como as camadas se dividem, o caminho de uma requisição, em que arquivo está cada coisa |
| 03 | [Ambiente e execução](03-ambiente-e-execucao.md) | Como montar a máquina, rodar, depurar e usar o container |
| 04 | [Como fazer (receitas)](04-como-fazer.md) | Passo a passo para criar endpoint, migration, regra, tipo de evento, mudar o contrato com o juros |
| 05 | [Regras de negócio no código](05-regras-e-calculos.md) | Onde cada RN está implementada e como funcionam os cálculos (atraso, encargos, valor presente) |
| 06 | [Dados, transações e idempotência](06-dados-e-consistencia.md) | Modelo de dados, o que é gravado e o que é calculado, travas, reenvios, decimal |
| 07 | [Testes e qualidade](07-testes-e-qualidade.md) | Que testes existem, como rodar os de integração, o que a PR precisa ter |
| 08 | [Armadilhas e FAQ](08-armadilhas-e-faq.md) | Os erros que todo mundo comete uma vez, e as perguntas que sempre aparecem |

## Documentos de referência

A base aponta para estes documentos em vez de repeti-los:

| Documento | Conteúdo |
| --- | --- |
| [Diagramas de classe](../architecture/diagrams/classes.md) | Estrutura das camadas, modelo de domínio e resultados das operações (UML em Mermaid) |
| [Diagrama de implantação](../architecture/diagrams/implantacao.md) | Execução nativa, container e a esteira na rede do laboratório |
| [Outros diagramas](../architecture/diagrams/README.md) | Contexto, camadas, sequência do `POST /operacoes`, modelo de dados (ER) e ciclo de vida da operação |
| [Decisões arquiteturais (ADRs)](../architecture/adr/README.md) | ADR-001 a ADR-016: por que o projeto é como é |
| [Integração com sistemas externos](../architecture/integracoes.md) | Quem chama o serviço, com que contrato, e como integrar um sistema novo |
| [Contratos da API](../architecture/contracts/README.md) e [`openapi.yaml`](../../api/openapi.yaml) | Todos os endpoints, campos, exemplos e mensagens de erro |
| [Guia de convenções](../architecture/contracts/guia-de-convencoes.md) | Regras comuns a todo endpoint e checklist da PR de contrato |
| [Requisitos](../requirements/README.md) e [Regras da Fase 2](../requirements/regras-de-negocio-fase-2.md) | Features, fases e as regras RN-01 a RN-21, com as fontes legais |
| [Requisitos não implementados](../requirements/nao-implementados.md) | O que ficou de fora e por quê |
| [Troubleshooting](../guides/troubleshooting.md) | Erros comuns ao rodar e como resolver |
| [Fluxo Git](../development/git-workflow.md) e [Governança](../development/project-governance.md) | Branches, commits, PRs, papéis e Kanban |

## Roteiro da entrega (Fase 3)

Onde está cada item pedido para esta entrega:

| Pedido | Onde está |
| --- | --- |
| Diagramas de classe | [diagrams/classes.md](../architecture/diagrams/classes.md) |
| Diagrama de implantação | [diagrams/implantacao.md](../architecture/diagrams/implantacao.md) |
| ADRs que construíram o projeto | [adr/README.md](../architecture/adr/README.md) (16 registros, com os substituídos) |
| Requisitos não implementados e os motivos | [requirements/nao-implementados.md](../requirements/nao-implementados.md) |
| Como o projeto interage com sistemas externos | [architecture/integracoes.md](../architecture/integracoes.md) |
| Base de Conhecimento para futuros desenvolvedores | Esta pasta |

## Como manter esta base

- **Mudou o comportamento, mude o artigo na mesma PR.** Uma base desatualizada é pior que nenhuma.
- **Decisão nova e difícil de reverter vira ADR**, não parágrafo aqui ([como escrever um ADR](../architecture/adr/README.md#como-escrever)). O artigo só aponta para ele.
- **Contrato da API mora no `openapi.yaml`.** A base explica o porquê; o detalhe de campo fica lá.
- **Resolveu um problema que custou mais de meia hora?** Acrescente em [Armadilhas e FAQ](08-armadilhas-e-faq.md) ou no [Troubleshooting](../guides/troubleshooting.md).
- Escreva para quem não estava na reunião: diga o que é, onde está e por quê.
