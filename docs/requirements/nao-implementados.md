# Requisitos não implementados

O que ficou de fora da entrega e por quê. Separamos três grupos, porque os motivos são diferentes:

1. **Requisitos do backlog** que não foram implementados;
2. **Itens excluídos de propósito** ao definir as regras da Fase 2;
3. **Requisitos não funcionais** que um sistema em produção teria e este não tem.

> **Situação geral:** tudo o que foi planejado para as Fases 1 e 2 foi entregue (veja [Requisitos](README.md#features-e-fases)). Os únicos itens do backlog não implementados são os dois de **renegociação**, que desde o início estavam marcados como dependentes de outro grupo e não entraram no planejamento das fases.

---

## 1. Requisitos do backlog não implementados

| Requisito | Situação |
| --- | --- |
| **Renegociar operação**: criar novas condições de pagamento para uma operação inadimplente | ⏸️ Não implementado |
| **Gerar novo cronograma após renegociação**: substituir ou reorganizar as parcelas conforme as novas condições | ⏸️ Não implementado |

### Por que não foram implementados

- **Dependem de outro grupo.** Renegociar é definir uma nova taxa, um novo prazo e um novo calendário. Na esteira, quem precifica e calcula calendário é o **serviço de juros**. Desde o [ADR-009](../architecture/adr/ADR-009-operacao-a-partir-da-simulacao-de-juros.md), este serviço **não calcula calendário**: recebe o do juros e confere. Fazer a renegociação aqui duplicaria a precificação e criaria duas fontes de verdade para a taxa.
- **O contrato nunca foi alinhado.** Seria preciso combinar com o grupo de juros quem inicia a renegociação, como a operação inadimplente chega até eles, se a nova operação tem outro identificador e o que acontece com a dívida em atraso (multa e mora incorporadas ou não ao novo saldo). Sem essas respostas, qualquer implementação seria um palpite.
- **Não estava no Sprint Backlog.** O backlog sempre marcou os dois itens como "depende de alinhamento com outro grupo", e eles não entraram nas fases planejadas ([Regras da Fase 2, Fora do escopo](regras-de-negocio-fase-2.md#fora-do-escopo-desta-fase)).

### O que já está pronto para quando forem feitos

- O status `RENEGOCIADA` já existe no `CHECK` da tabela `operacao` e na constante `domain.StatusOperacaoRenegociada`.
- O modelo de eventos e lançamentos ([ADR-012](../architecture/adr/ADR-012-eventos-e-lancamentos.md)) aceita um tipo novo de evento (`RENEGOCIACAO`) com uma migration que só amplia o `CHECK` do `evento.tipo`.
- A transação de movimentação ([ADR-014](../architecture/adr/ADR-014-concorrencia-e-idempotencia-das-movimentacoes.md)) já trava a operação e grava evento, status e saldo juntos; a renegociação seria um `decidir` novo.
- A conferência de calendário (`internal/service/calendario.go`) pode ser reaproveitada para validar o calendário novo vindo do juros.

Caminho sugerido em [Integração com sistemas externos](../architecture/integracoes.md#o-que-não-existe-hoje-e-como-seria-acrescentado).

---

## 2. Itens excluídos de propósito na Fase 2

Ao escrever as [regras da Fase 2](regras-de-negocio-fase-2.md), cada decisão foi tirada de legislação e normas do Banco Central. Quando a norma permite escolher, adotamos a opção mais simples que continua correta. Ficaram de fora:

| Item | Por que ficou de fora |
| --- | --- |
| **Juros remuneratórios durante o atraso** | A Res. CMN 4.882/2020 permite, mas não obriga. Multa de 2% e mora de 1% ao mês já cobrem o atraso (RN-13), e somar um terceiro encargo complicaria o cálculo e os testes sem pedido do negócio. |
| **Reduzir o valor das parcelas na amortização** (em vez de reduzir o prazo) | Exigiria recalcular o calendário inteiro, o que é papel do juros. A amortização quita parcelas inteiras pelo valor presente (modos `ULTIMAS` e `PROXIMAS`, RN-17) e mantém os valores das que sobram. |
| **IOF sobre atraso ou devolução de IOF na quitação** | Regra tributária à parte. O IOF é calculado pelo juros na contratação e chega pronto no payload. |
| **Calendário de feriados** | Os vencimentos já chegam ajustados para dia útil pelo juros (RN-10). |
| **Estorno de pagamento** | Nenhuma história do backlog pediu. O modelo já permite: seria um evento novo, sem apagar o original. |
| **Job diário que grava atraso e inadimplência** | A situação é calculada na consulta para qualquer data (RN-11, [ADR-012](../architecture/adr/ADR-012-eventos-e-lancamentos.md)). Isso evita um banco desatualizado se o job falhar e permite testar qualquer cenário com `dataReferencia`. |

---

## 3. Requisitos não funcionais não atendidos

São limitações conhecidas. Nenhuma impede o uso acadêmico do serviço, mas todas precisariam ser resolvidas antes de um uso real.

| Requisito | Situação atual | Por que ficou assim | Impacto | Como resolver |
| --- | --- | --- | --- | --- |
| **Autenticação e autorização** | Nenhuma. Qualquer cliente que alcance a porta 8083 registra operações e pagamentos | Nenhum serviço da esteira tem autenticação e não há provedor de identidade comum aos grupos ([ADR-016](../architecture/adr/ADR-016-cors-liberado-e-sem-autenticacao.md)) | Alto em produção; nulo no laboratório | Middleware de chave de API ou JWT, antes das rotas |
| **HTTPS** | HTTP puro | A rede do laboratório é local e os demais serviços também usam HTTP | Alto em produção | Proxy reverso com TLS na frente da API |
| **Erros internos como `500`** | Uma falha inesperada do banco sai como `400`, com a mensagem original | O handler mapeia `404` para "não encontrado" e `400` para o resto. Separar exige marcar os erros de validação com um tipo próprio em todo o service ([ADR-002](../architecture/adr/ADR-002-arquitetura-em-camadas.md)) | Médio: o chamador pode tratar como erro definitivo uma falha temporária | Um tipo `ErroDeValidacao` no `domain`, `errors.As` no `responderErro`, e `500` com mensagem genérica no resto |
| **Notificação de mudanças** | Nenhum sistema é avisado quando uma operação fica inadimplente ou é liquidada | Decorrência da situação calculada na consulta ([ADR-012](../architecture/adr/ADR-012-eventos-e-lancamentos.md)) e de não haver barramento de mensagens na esteira | Baixo hoje: nenhum grupo consome esses avisos | Job diário + *outbox* publicando eventos |
| **Paginação da listagem** | `GET /operacoes` devolve todas as operações | Volume de um projeto acadêmico | Baixo hoje; cresce com o número de operações | `limite` e `cursor` na query |
| **Logs estruturados e métricas** | Só o log padrão do Gin no terminal | Sem plataforma de observabilidade no ambiente | Médio em produção: difícil investigar incidentes | Log em JSON com id de requisição e `numeroOperacao`; métricas Prometheus |
| **Migrations automáticas** | Aplicadas à mão com `psql` (ou pelo PostgreSQL no primeiro start do container) | Evitar mais uma ferramenta para a equipe aprender ([ADR-007](../architecture/adr/ADR-007-migrations-sql-versionadas.md)) | Baixo: são só duas migrations | Ferramenta como `golang-migrate`, rodando na partida ou no deploy |
| **Alta disponibilidade e backup** | Uma instância da API e um banco, sem réplica nem backup agendado | Fora do escopo de um projeto acadêmico | Alto em produção | A API não guarda estado em memória, então várias instâncias atrás de um balanceador funcionam sem mudar o código; backup e réplica ficam no PostgreSQL |

### Pequenas dívidas técnicas

- Os status `VENCIDA`, `EM_ATRASO` e `INADIMPLENTE` continuam permitidos no `CHECK` da migration `0001`, mas nunca são gravados. Removê-los exigiria uma migration nova só para isso ([ADR-012](../architecture/adr/ADR-012-eventos-e-lancamentos.md)).
- No container, uma migration nova não é aplicada num volume que já existe; é preciso `docker compose down -v` ou aplicá-la à mão ([ADR-013](../architecture/adr/ADR-013-container-para-entrega.md)).
