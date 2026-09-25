# Diagramas

Os diagramas estão em [Mermaid](https://mermaid.js.org/), que o GitHub renderiza direto no navegador.

## Contexto: Esteira de Crédito

```mermaid
flowchart LR
    Cliente([Dados do cliente]) --> Score
    Score["Score Engine<br/>:8080"] -->|"POST resultado do score"| Decisao
    Decisao["Decisão<br/>:8081"] -->|"POST /operacoes<br/>(somente APROVADO)"| FinOps
    FinOps["Operações Financeiras<br/>:8083"] --> DB[(PostgreSQL<br/>finops)]
```

Este serviço é a última etapa: recebe apenas operações aprovadas e passa a ser dono do ciclo financeiro delas.

## Camadas internas

```mermaid
flowchart TB
    HTTP([Requisição HTTP]) --> Handler
    subgraph api["cmd/api"]
        Handler["handler<br/>request/response, status HTTP"]
        Service["service<br/>validação, regras, cronograma"]
        Repository["repository<br/>SQL"]
        Domain["domain<br/>tipos, status, erros"]
    end
    Handler --> Service --> Repository --> DB[(PostgreSQL)]
    Handler -.-> Domain
    Service -.-> Domain
    Repository -.-> Domain
```

Detalhes em [ADR-002](../adr/ADR-002-arquitetura-em-camadas.md).

## Fluxo de `POST /operacoes`

```mermaid
sequenceDiagram
    participant D as Decisão
    participant H as handler
    participant S as service
    participant R as repository
    participant P as PostgreSQL

    D->>H: POST /operacoes (payload aprovado)
    H->>H: bind do JSON
    H->>S: CriarOperacao(NovaOperacao)
    S->>S: valida e normaliza
    alt payload inválido
        S-->>H: erro de validação
        H-->>D: 400 {"erro": "..."}
    else payload válido
        S->>S: GerarCronograma (PRICE ou SAC)
        S->>R: CriarComParcelas
        R->>P: BEGIN
        R->>P: INSERT operacao RETURNING (id, numero_operacao)
        loop cada parcela
            R->>P: INSERT parcela RETURNING
        end
        R->>P: COMMIT
        R-->>S: operação + parcelas
        S-->>H: operação + parcelas
        H-->>D: 201 operação com cronograma
    end
```

## Modelo de dados

```mermaid
erDiagram
    operacao ||--|{ parcela : "possui"

    operacao {
        UUID id PK
        VARCHAR numero_operacao UK "OP-AAAA-NNNNNN"
        VARCHAR cliente_id
        VARCHAR tipo_pessoa "PF | PJ"
        VARCHAR status "ATIVA | EM_ATRASO | INADIMPLENTE | RENEGOCIADA | LIQUIDADA"
        DATE data_aprovacao
        NUMERIC valor_aprovado "15,2"
        INTEGER prazo_meses
        INTEGER quantidade_parcelas
        NUMERIC taxa_juros "10,6"
        VARCHAR sistema_amortizacao "PRICE | SAC"
        DATE primeiro_vencimento
        NUMERIC saldo_devedor "15,2"
        INTEGER score_final "opcional"
        VARCHAR faixa_risco "opcional"
        NUMERIC probabilidade_default "opcional"
        VARCHAR modelo_score "opcional"
        TEXT motivo_score "opcional"
        TIMESTAMPTZ criado_em
    }

    parcela {
        UUID id PK
        UUID operacao_id FK
        INTEGER numero "único por operação"
        DATE vencimento
        NUMERIC valor_parcela "15,2"
        NUMERIC amortizacao "15,2"
        NUMERIC juros "15,2"
        NUMERIC saldo_devedor "15,2"
        VARCHAR status "PENDENTE | PAGA | PARCIALMENTE_PAGA | VENCIDA"
    }
```

Schema completo em [`migrations/`](../../../migrations).

## Ciclo de vida da operação

```mermaid
stateDiagram-v2
    [*] --> ATIVA: POST /operacoes
    ATIVA --> EM_ATRASO: parcela vencida (futuro)
    EM_ATRASO --> ATIVA: pagamento em dia (futuro)
    EM_ATRASO --> INADIMPLENTE: atraso acima do limite (futuro)
    INADIMPLENTE --> RENEGOCIADA: renegociação (futuro)
    RENEGOCIADA --> ATIVA: novo cronograma (futuro)
    ATIVA --> LIQUIDADA: saldo zerado (futuro)
    LIQUIDADA --> [*]
```

Na Fase 1 só existe a transição inicial para `ATIVA`. As demais são planejadas; as regras exatas de cada transição serão definidas nas próximas fases.
