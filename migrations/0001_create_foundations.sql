-- schema reiniciado na integração com o serviço de juros, ver ADR-010.
CREATE TABLE operacao (
    -- sequencial, como o id do serviço de juros. o número que identifica a operação na esteira é o numero_operacao.
    id                    BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    -- vem do serviço de juros e identifica a operação em toda a esteira, por isso é UNIQUE.
    numero_operacao       VARCHAR(20)   NOT NULL UNIQUE,
    cliente_id            VARCHAR(14)   NOT NULL,
    tipo_pessoa           VARCHAR(2)    NOT NULL CHECK (tipo_pessoa IN ('PF', 'PJ')),
    -- os demais status ja ficam previstos para as proximas fases.
    status                VARCHAR(20)   NOT NULL DEFAULT 'ATIVA'
                          CHECK (status IN ('ATIVA', 'EM_ATRASO', 'INADIMPLENTE', 'RENEGOCIADA', 'LIQUIDADA')),
    definido_em           TIMESTAMPTZ   NOT NULL,
    data_liberacao        DATE          NOT NULL,
    produto_codigo        VARCHAR(50)   NOT NULL,
    produto_nome          VARCHAR(100)  NOT NULL,
    modalidade_bcb        VARCHAR(150)  NOT NULL,
    valor_aprovado        NUMERIC(15,2) NOT NULL CHECK (valor_aprovado > 0),
    prazo_meses           INTEGER       NOT NULL,
    quantidade_parcelas   INTEGER       NOT NULL CHECK (quantidade_parcelas > 0),
    -- as taxas ficam em fração (0.0185 = 1,85%), o juros manda em percentual e a conversão é feita na entrada.
    taxa_juros            NUMERIC(10,6) NOT NULL CHECK (taxa_juros >= 0),
    taxa_juros_anual      NUMERIC(10,6) NOT NULL CHECK (taxa_juros_anual >= 0),
    -- faixa de preço do juros, não é a faixa de risco do score.
    faixa_risco           VARCHAR(10)   NOT NULL,
    teto_aplicado         BOOLEAN       NOT NULL,
    sistema_amortizacao   VARCHAR(10)   NOT NULL CHECK (sistema_amortizacao IN ('PRICE', 'SAC')),
    primeiro_vencimento   DATE          NOT NULL,
    saldo_devedor         NUMERIC(15,2) NOT NULL,
    -- encargos são descontados na liberação, não entram no saldo devedor.
    iof_diario            NUMERIC(15,2) NOT NULL CHECK (iof_diario >= 0),
    iof_adicional         NUMERIC(15,2) NOT NULL CHECK (iof_adicional >= 0),
    iof_total             NUMERIC(15,2) NOT NULL CHECK (iof_total >= 0),
    tarifa_cadastro       NUMERIC(15,2) NOT NULL CHECK (tarifa_cadastro >= 0),
    encargos_total        NUMERIC(15,2) NOT NULL CHECK (encargos_total >= 0),
    valor_liberado        NUMERIC(15,2) NOT NULL CHECK (valor_liberado > 0),
    cet_mensal            NUMERIC(10,6) NOT NULL CHECK (cet_mensal >= 0),
    cet_anual             NUMERIC(10,6) NOT NULL CHECK (cet_anual >= 0),
    criado_em             TIMESTAMPTZ   NOT NULL DEFAULT now()
);

CREATE INDEX idx_operacao_cliente ON operacao (cliente_id);

CREATE TABLE parcela (
    id             BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    operacao_id    BIGINT        NOT NULL REFERENCES operacao(id),
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
