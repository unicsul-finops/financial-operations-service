# ADR-015: Painel web num arquivo HTML embutido no binário

## Status

Aceita

## Data

2026-10-30

## Contexto

Até a Fase 2, a única forma de usar o serviço era pelo Swagger ou pela coleção do Postman/Bruno. Isso basta para testar o contrato, mas não para acompanhar a carteira: ver quais operações estão em atraso, quanto falta para quitar, o histórico de uma operação. Para a apresentação, era preciso mostrar isso de forma visual, inclusive na máquina do professor e na rede do laboratório, que pode não ter internet.

Restrições:

- ninguém do grupo é dono de um front separado, e criar um repositório e um processo de build a mais aumentaria o que precisa ser mantido;
- o painel tem que abrir em qualquer máquina que rode a API, sem passos extras;
- os dados precisam ser os da API que serviu a página, para que cada integrante veja o próprio banco.

## Opções consideradas

- Projeto de front separado (React/Vite), com build e servidor próprios.
- Pasta `static/` servida pela API, com HTML, CSS e JS separados.
- **Um único `web/index.html`, com CSS, JavaScript, ícones e fonte dentro dele, embutido no binário com `go:embed` e servido em `GET /`.**

## Decisão

- O painel é **um arquivo só**: `web/index.html`, com JavaScript puro (sem framework e sem etapa de build), a fonte em base64 e os ícones em SVG inline. Não carrega nada da internet.
- O pacote `web` expõe o conteúdo com `//go:embed index.html`, e o handler `Painel` responde em `GET /`.
- O painel chama a API **pelo caminho relativo** (`fetch('/operacoes...')`), ou seja, sempre a mesma origem que o serviu. Não depende de CORS.
- Navegação por hash (`#/operacoes/OP-2026-0001`), sem rotas no servidor além de `/`.
- A **data de referência** fica no topo e vale para todas as consultas, como na API (RN-09).

## Consequências

- Implantar continua sendo copiar um binário: o painel vai junto, como o `openapi.yaml` ([ADR-008](ADR-008-openapi-escrito-a-mao.md)).
- Funciona sem internet, na rede do laboratório e no container ([ADR-013](ADR-013-container-para-entrega.md)).
- O painel não tem regra de negócio: tudo o que ele mostra vem da API. Uma regra nova aparece no painel só se o JSON mudar.
- **Mudar o painel exige recompilar a API**, porque o HTML está no binário. Para iterar no visual, rode `go run ./cmd/api` de novo.
- Um arquivo de quase mil linhas é mais difícil de revisar que componentes separados. Se o painel crescer muito, a saída é um front separado consumindo a mesma API, que já está pronta para isso com o CORS ([ADR-016](ADR-016-cors-liberado-e-sem-autenticacao.md)).
- O teste `TestPainelResponde` garante que `GET /` devolve o HTML.
