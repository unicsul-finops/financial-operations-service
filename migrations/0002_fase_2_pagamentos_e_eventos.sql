-- fase 2: pagamento, amortização e liquidação. as regras estão em docs/requirements/regras-de-negocio-fase-2.md.
-- só cria tabelas: nada da 0001 muda. atraso, encargos em aberto e resumo são calculados a partir daqui (RN-11 e RN-13).
-- os status VENCIDA, EM_ATRASO e INADIMPLENTE continuam no CHECK da 0001, mas não são gravados: são calculados na consulta.
BEGIN;

-- um fato da operação. é o histórico (RN-21) e o cabeçalho de cada movimentação de dinheiro.
CREATE TABLE evento (
    id                    BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    operacao_id           BIGINT        NOT NULL REFERENCES operacao(id),
    tipo                  VARCHAR(30)   NOT NULL
                          CHECK (tipo IN ('OPERACAO_REGISTRADA', 'PAGAMENTO', 'AMORTIZACAO_ANTECIPADA', 'LIQUIDACAO')),
    -- dia em que o fato vale (a dataPagamento), que pode ser anterior ao dia em que foi registrado.
    data_efeito           DATE          NOT NULL,
    -- dinheiro recebido no evento. nulo no registro da operação, que não movimenta dinheiro.
    valor                 NUMERIC(15,2) CHECK (valor > 0),
    -- identificadorPagamento de quem chamou, pra um reenvio não gravar de novo (RN-14).
    identificador_externo VARCHAR(50),
    origem                VARCHAR(30),
    -- o que não cabe em coluna, como as parcelas quitadas e a repartição antes e depois de uma amortização.
    detalhe               JSONB         NOT NULL DEFAULT '{}',
    registrado_em         TIMESTAMPTZ   NOT NULL DEFAULT now(),
    -- vários NULL não conflitam no UNIQUE, então evento sem identificador externo não é barrado.
    UNIQUE (operacao_id, identificador_externo),
    -- alvo da FK do lancamento, pra garantir que o lançamento é da mesma operação do evento.
    UNIQUE (id, operacao_id)
);

CREATE INDEX idx_evento_operacao ON evento (operacao_id, data_efeito);

-- pra onde foi o dinheiro de cada evento, por parcela e por componente (RN-13, RN-14, RN-17 e RN-18).
CREATE TABLE lancamento (
    id              BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    evento_id       BIGINT        NOT NULL,
    operacao_id     BIGINT        NOT NULL,
    -- nulo só na liquidação, pras parcelas a vencer, que entram juntas num lançamento da operação (RN-18).
    numero_parcela  INTEGER,
    multa           NUMERIC(15,2) NOT NULL DEFAULT 0 CHECK (multa >= 0),
    juros_mora      NUMERIC(15,2) NOT NULL DEFAULT 0 CHECK (juros_mora >= 0),
    juros           NUMERIC(15,2) NOT NULL DEFAULT 0 CHECK (juros >= 0),
    amortizacao     NUMERIC(15,2) NOT NULL DEFAULT 0 CHECK (amortizacao >= 0),
    -- desconto da antecipação (valor da parcela − valor presente). não é dinheiro recebido, por isso fica fora do valor.
    desconto        NUMERIC(15,2) NOT NULL DEFAULT 0 CHECK (desconto >= 0),
    valor           NUMERIC(15,2) NOT NULL CHECK (valor > 0),
    CHECK (valor = multa + juros_mora + juros + amortizacao),
    FOREIGN KEY (evento_id, operacao_id) REFERENCES evento(id, operacao_id),
    -- com numero_parcela nulo a FK não é conferida, que é o caso da liquidação.
    FOREIGN KEY (operacao_id, numero_parcela) REFERENCES parcela(operacao_id, numero)
);

CREATE INDEX idx_lancamento_parcela ON lancamento (operacao_id, numero_parcela);

COMMIT;
