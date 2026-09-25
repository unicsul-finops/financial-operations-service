# ADR-006: Número de operação legível e busca por ambos os identificadores

## Status

Aceita

## Data

2026-09-24

## Contexto

O UUID ([ADR-003](ADR-003-id-gerado-pelo-servico.md)) é adequado como chave técnica, mas ruim para pessoas: difícil de ler, ditar ou procurar numa tela. Precisávamos de um identificador de exibição, sem alterar o contrato já combinado com a Decisão.

## Opções consideradas

Para gerar o número:

- Gerar o número no Go (exige coordenação entre instâncias).
- **`SEQUENCE` do PostgreSQL com formatação no banco.**

Para buscar pelo número:

- Filtro na listagem (`GET /operacoes?numeroOperacao=...`).
- Rota separada (`GET /operacoes/numero/:numero`).
- **A mesma rota `:id` aceita o UUID ou o número.**

## Decisão

- A coluna `numero_operacao` tem o formato **`OP-AAAA-NNNNNN`** (ex.: `OP-2026-000001`), é `NOT NULL` e `UNIQUE`, e é preenchida por `DEFAULT` a partir da sequence `operacao_numero_seq`, pela função `formatar_numero_operacao`. Foi criada na migration `0002`.
- O **UUID continua sendo a chave primária**. O `numeroOperacao` é devolvido a mais nas respostas.
- `GET /operacoes/:id` e `GET /operacoes/:id/parcelas` aceitam os dois formatos. O service identifica o formato pela regex; o número é aceito em maiúsculas ou minúsculas.
- O número pode ter mais de 6 dígitos depois de `999999` (a função evita que o `lpad` trunque e gere números repetidos).

## Consequências

- O contrato com a Decisão não mudou: quem usa UUID continua funcionando igual.
- A busca pelo número usa o índice da constraint `UNIQUE`.
- **A numeração não reinicia a cada ano:** a sequence é única e o ano vem da data de criação. Em 2027 a numeração continua de onde parou (ex.: `OP-2027-000153`).
- **Pode haver buracos:** um número consumido por um POST que falhou não é reutilizado. Para exibição isso é aceitável, mas não se trata de uma numeração fiscal contínua.
