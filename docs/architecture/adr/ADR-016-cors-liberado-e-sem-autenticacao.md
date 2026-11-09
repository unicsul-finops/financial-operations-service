# ADR-016: CORS liberado por padrão e API sem autenticação

## Status

Aceita

## Data

2026-10-30

## Contexto

Na apresentação, cada grupo roda o próprio serviço no próprio PC, e os serviços e as telas se falam pela rede local (Juros chama `http://IP:8083/operacoes`). Outros grupos quiseram montar fronts próprios que consultam este serviço direto do navegador, de outra origem (outro IP ou outra porta). O navegador bloqueia essas chamadas sem os cabeçalhos de CORS.

Ao mesmo tempo, nenhum serviço da esteira tem autenticação, e não existe um provedor de identidade comum aos grupos.

## Opções consideradas

Para o CORS:

- Sem CORS: só o painel embutido e chamadas servidor a servidor funcionam.
- Lista fixa de origens no código.
- **Middleware configurável por `CORS_ORIGINS`: `*` (padrão) libera tudo, uma lista separada por vírgula libera só aquelas origens, `off` desliga.**

Para a autenticação:

- Chave de API compartilhada (`X-API-Key`).
- JWT emitido por um serviço de identidade.
- **Sem autenticação nesta fase, como os demais serviços da esteira.**

## Decisão

- `handler.CORS(origens)` roda **antes do roteamento**, para valer também no preflight (`OPTIONS`, respondido com `204`) e nos erros `404` e `405`.
- Com `*`, responde `Access-Control-Allow-Origin: *`. Com uma lista, devolve a própria origem quando ela está na lista, com `Vary: Origin`.
- Libera os métodos `GET, POST, OPTIONS` e expõe o cabeçalho `Location`, para que o JavaScript leia o endereço do recurso criado nas respostas `201`.
- O padrão é `*` porque o uso previsto é a rede do laboratório. A variável fica no `.env` e no `docker-compose.yml`.
- **Não há autenticação nem autorização.** Qualquer cliente que alcance a porta 8083 pode registrar operações e pagamentos.

## Consequências

- Fronts de outros grupos chamam a API sem proxy. O painel embutido continua independente disso, porque usa a mesma origem ([ADR-015](ADR-015-painel-web-embutido.md)).
- Os testes `TestCORSLiberado`, `TestCORSRestrito` e `TestCORSDesligado` cobrem os três modos.
- **Isto não serve para produção.** Uma API financeira exposta exige, no mínimo, HTTPS, autenticação dos chamadores (o serviço de juros e o canal de pagamento) e uma lista fechada de origens. Esse trabalho está listado em [Requisitos não implementados](../../requirements/nao-implementados.md).
- Como não há usuário autenticado, o campo `origem` dos pagamentos é informado pelo chamador e não é conferido.
