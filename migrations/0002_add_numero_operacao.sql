-- tudo ou nada: se o backfill falhar, a coluna não fica pela metade.
BEGIN;

CREATE SEQUENCE operacao_numero_seq AS BIGINT;

-- o número entra por parâmetro pra o nextval ser chamado uma vez só.
-- o lpad sozinho trunca o texto passando de 6 dígitos (1000000 viraria 100000), o greatest evita número repetido.
CREATE FUNCTION formatar_numero_operacao(numero BIGINT, data TIMESTAMPTZ) RETURNS VARCHAR(20) AS $$
    SELECT 'OP-' || to_char(data, 'YYYY') || '-' || lpad(numero::text, greatest(6, length(numero::text)), '0')
$$ LANGUAGE sql STABLE;

ALTER TABLE operacao ADD COLUMN numero_operacao VARCHAR(20);

-- operações que já existem ganham número na ordem em que foram criadas, com o ano da criação.
UPDATE operacao o
SET numero_operacao = formatar_numero_operacao(ordenadas.numero, o.criado_em)
FROM (
    SELECT id, nextval('operacao_numero_seq') AS numero
    FROM (SELECT id FROM operacao ORDER BY criado_em, id) AS por_criacao
) AS ordenadas
WHERE ordenadas.id = o.id;

ALTER TABLE operacao
    ALTER COLUMN numero_operacao SET DEFAULT formatar_numero_operacao(nextval('operacao_numero_seq'), now()),
    ALTER COLUMN numero_operacao SET NOT NULL,
    ADD CONSTRAINT operacao_numero_operacao_key UNIQUE (numero_operacao);

-- se a coluna for removida, a sequence vai junto.
ALTER SEQUENCE operacao_numero_seq OWNED BY operacao.numero_operacao;

COMMIT;
