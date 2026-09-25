CREATE TABLE operacao (
    id                    UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    cliente_id            VARCHAR(20)   NOT NULL,
    tipo_pessoa           VARCHAR(2)    NOT NULL CHECK (tipo_pessoa IN ('PF', 'PJ')),
    -- os demais status ja ficam previstos para as proximas fases.
    status                VARCHAR(20)   NOT NULL DEFAULT 'ATIVA'
                          CHECK (status IN ('ATIVA', 'EM_ATRASO', 'INADIMPLENTE', 'RENEGOCIADA', 'LIQUIDADA')),
    data_aprovacao        DATE          NOT NULL,
    valor_aprovado        NUMERIC(15,2) NOT NULL CHECK (valor_aprovado > 0),
    prazo_meses           INTEGER       NOT NULL,
    quantidade_parcelas   INTEGER       NOT NULL CHECK (quantidade_parcelas > 0),
    -- 6 casas pra nao perder precisao em taxas tipo 0.019875.
    taxa_juros            NUMERIC(10,6) NOT NULL CHECK (taxa_juros >= 0),
    sistema_amortizacao   VARCHAR(10)   NOT NULL CHECK (sistema_amortizacao IN ('PRICE', 'SAC')),
    primeiro_vencimento   DATE          NOT NULL,
    saldo_devedor         NUMERIC(15,2) NOT NULL,
    -- contexto do score so é guardado para consulta, nao entra em nenhum calculo.
    score_final           INTEGER,
    faixa_risco           VARCHAR(30),
    probabilidade_default NUMERIC(8,6),
    modelo_score          VARCHAR(50),
    motivo_score          TEXT,
    criado_em             TIMESTAMPTZ   NOT NULL DEFAULT now()
);

CREATE INDEX idx_operacao_cliente ON operacao (cliente_id);

CREATE TABLE parcela (
    id             UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    operacao_id    UUID          NOT NULL REFERENCES operacao(id),
    numero         INTEGER       NOT NULL,
    vencimento     DATE          NOT NULL,
    valor_parcela  NUMERIC(15,2) NOT NULL,
    amortizacao    NUMERIC(15,2) NOT NULL,
    juros          NUMERIC(15,2) NOT NULL,
    saldo_devedor  NUMERIC(15,2) NOT NULL,
    status         VARCHAR(20)   NOT NULL DEFAULT 'PENDENTE'
                   CHECK (status IN ('PENDENTE', 'PAGA', 'PARCIALMENTE_PAGA', 'VENCIDA')),
    UNIQUE (operacao_id, numero)
);
