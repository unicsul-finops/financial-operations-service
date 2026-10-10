# Diagramas

Os diagramas estão em [Mermaid](https://mermaid.js.org/), que o GitHub renderiza direto no navegador.

## Contexto: Esteira de Crédito

```mermaid
flowchart LR
    Cliente([Dados do cliente]) --> Score
    Score["Score Engine<br/>:8080"] -->|"POST resultado do score"| Decisao
    Decisao["Decisão<br/>:8081"] -->|"aprovada"| Juros
    Juros["Juros"] -->|"POST /operacoes<br/>(somente DEFINIDA,<br/>com o calendário)"| FinOps
    FinOps["Operações Financeiras<br/>:8083"] --> DB[(PostgreSQL<br/>finops)]
```

Este serviço é a última etapa: recebe apenas operações definidas no juros, já com o calendário, e passa a ser dono do ciclo financeiro delas.

## Camadas internas

```mermaid
flowchart TB
    HTTP([Requisição HTTP]) --> Handler
    subgraph api["cmd/api"]
        Handler["handler<br/>request/response, status HTTP"]
        Service["service<br/>validação, regras, conferência do calendário"]
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
    participant J as Juros
    participant H as handler
    participant S as service
    participant R as repository
    participant P as PostgreSQL

    J->>H: POST /operacoes (operação definida + calendário)
    H->>H: bind do JSON
    H->>S: CriarOperacao(NovaOperacao)
    S->>S: valida, confere o calendário, converte taxas e fuso
    alt payload inválido
        S-->>H: erro de validação
        H-->>J: 400 {"erro": "..."}
    else identificador já registrado
        S->>R: BuscarPorNumero
        R-->>S: operação existente
        H-->>J: 200 {id, numeroOperacao, status, criadoEm}
    else operação nova
        S->>R: CriarComParcelas
        R->>P: BEGIN
        R->>P: INSERT operacao RETURNING
        loop cada parcela
            R->>P: INSERT parcela
        end
        R->>P: COMMIT
        R-->>S: operação
        Note over S,P: envio simultâneo do mesmo identificador cai na UNIQUE<br/>e o service devolve a operação já gravada (200)
        H-->>J: 201 {id, numeroOperacao, status, criadoEm} + Location
    end
```

## Modelo de dados

```mermaid
erDiagram
    operacao ||--|{ parcela : "possui"
    operacao ||--o{ evento : "histórico"
    evento ||--|{ lancamento : "distribui o dinheiro"
    parcela ||--o{ lancamento : "recebe"

    operacao {
        BIGINT id PK "sequencial"
        VARCHAR numero_operacao UK "identificador do juros"
        VARCHAR cliente_id "CPF ou CNPJ"
        VARCHAR tipo_pessoa "PF | PJ"
        VARCHAR status "ATIVA | EM_ATRASO | INADIMPLENTE | RENEGOCIADA | LIQUIDADA"
        TIMESTAMPTZ definido_em
        DATE data_liberacao
        VARCHAR produto_codigo
        VARCHAR produto_nome
        VARCHAR modalidade_bcb
        NUMERIC valor_aprovado "15,2"
        INTEGER prazo_meses
        INTEGER quantidade_parcelas
        NUMERIC taxa_juros "10,6 fração mensal"
        NUMERIC taxa_juros_anual "10,6 fração"
        VARCHAR faixa_risco "faixa de preço do juros"
        BOOLEAN teto_aplicado
        VARCHAR sistema_amortizacao "PRICE | SAC"
        DATE primeiro_vencimento
        NUMERIC saldo_devedor "15,2"
        NUMERIC iof_diario "15,2"
        NUMERIC iof_adicional "15,2"
        NUMERIC iof_total "15,2"
        NUMERIC tarifa_cadastro "15,2"
        NUMERIC encargos_total "15,2"
        NUMERIC valor_liberado "15,2"
        NUMERIC cet_mensal "10,6 fração"
        NUMERIC cet_anual "10,6 fração"
        TIMESTAMPTZ criado_em
    }

    parcela {
        BIGINT id PK "sequencial"
        BIGINT operacao_id FK
        INTEGER numero "único por operação"
        DATE vencimento
        NUMERIC valor_parcela "15,2"
        NUMERIC amortizacao "15,2"
        NUMERIC juros "15,2"
        NUMERIC saldo_devedor "15,2"
        VARCHAR status "PENDENTE | PARCIALMENTE_PAGA | PAGA (VENCIDA é calculada)"
    }

    evento {
        BIGINT id PK "sequencial"
        BIGINT operacao_id FK
        VARCHAR tipo "OPERACAO_REGISTRADA | PAGAMENTO | AMORTIZACAO_ANTECIPADA | LIQUIDACAO"
        DATE data_efeito "dataPagamento"
        NUMERIC valor "15,2, nulo no registro"
        VARCHAR identificador_externo "único por operação, opcional"
        VARCHAR origem "opcional"
        JSONB detalhe
        TIMESTAMPTZ registrado_em
    }

    lancamento {
        BIGINT id PK "sequencial"
        BIGINT evento_id FK
        BIGINT operacao_id FK
        INTEGER numero_parcela FK "nulo só na liquidação"
        NUMERIC multa "15,2"
        NUMERIC juros_mora "15,2"
        NUMERIC juros "15,2"
        NUMERIC amortizacao "15,2"
        NUMERIC desconto "15,2, fora do valor"
        NUMERIC valor "multa + juros_mora + juros + amortizacao"
    }
```

Schema completo em [`migrations/`](../../../migrations).

## Ciclo de vida da operação

```mermaid
stateDiagram-v2
    [*] --> ATIVA: POST /operacoes
    ATIVA --> EM_ATRASO: parcela vencida e não paga (1 dia)
    EM_ATRASO --> INADIMPLENTE: alguma parcela com mais de 90 dias de atraso
    EM_ATRASO --> ATIVA: parcelas vencidas pagas
    INADIMPLENTE --> EM_ATRASO: atraso volta a 90 dias ou menos
    INADIMPLENTE --> ATIVA: parcelas vencidas pagas
    ATIVA --> LIQUIDADA: liquidação ou última parcela paga
    EM_ATRASO --> LIQUIDADA: liquidação (vencidas com encargos)
    INADIMPLENTE --> LIQUIDADA: liquidação (vencidas com encargos)
    LIQUIDADA --> [*]
```

Regras em [RN-11, RN-12, RN-18 e RN-19](../../requirements/regras-de-negocio-fase-2.md). Só `ATIVA` e `LIQUIDADA` são gravados: `EM_ATRASO` e `INADIMPLENTE` são calculados na consulta, para a data de referência, então as transições entre eles acontecem sozinhas com o passar do tempo e com os pagamentos ([ADR-012](../adr/ADR-012-eventos-e-lancamentos.md)). `RENEGOCIADA` continua prevista no banco, mas depende de alinhamento com outro grupo.
