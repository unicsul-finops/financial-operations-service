# ADR-013: Container para rodar a entrega em máquinas sem Go nem PostgreSQL

## Status

Proposta

## Data

2026-10-30

## Contexto

O serviço é desenvolvido e testado com Go e PostgreSQL instalados direto na máquina de cada integrante, e é assim que continua. O professor, porém, precisa rodar tudo no computador dele, onde não dá para instalar o Go. Na sala também não dá para subir container, então a execução na sala continua sendo a nativa, com o serviço de cada integrante acessado pela rede.

Precisamos de um jeito de empacotar o serviço e o banco para quem só tem um runtime de container.

## Opções consideradas

- Entregar o binário compilado (`go build`) e pedir para instalar o PostgreSQL.
- **Imagem da API e um `docker-compose.yml` que sobe a API e o PostgreSQL juntos.**
- Imagem só da API, com o banco por fora.

## Decisão

- **`Dockerfile` em dois estágios:** compila com `golang:1.26-alpine` (`CGO_ENABLED=0`) e copia só o binário para uma imagem `alpine` enxuta, rodando com usuário comum. O painel (`web/`) e o `openapi.yaml` já vão embutidos no binário, então nada mais precisa ser copiado.
- **`docker-compose.yml` com dois serviços:** `db` (`postgres:18-alpine`, a mesma versão usada no desenvolvimento) e `api`, que só sobe depois de o banco responder ao `pg_isready`.
- **Migrations pelo próprio PostgreSQL:** a pasta `migrations/` é montada em `/docker-entrypoint-initdb.d`, e o PostgreSQL roda os arquivos em ordem na primeira vez que o volume está vazio. Continua valendo o [ADR-007](ADR-007-migrations-sql-versionadas.md): as migrations são as mesmas, versionadas e numeradas, sem ferramenta nova.
- **Configuração por variável de ambiente**, como já é: `PORT`, `DATABASE_URL` e `CORS_ORIGINS` (padrão `*`). A senha do banco e a porta publicada podem ser trocadas com `POSTGRES_PASSWORD` e `API_PORT`.
- Os dados ficam num volume nomeado (`pgdata`), então sobrevivem a `docker compose down`. Para recomeçar do zero e reaplicar as migrations, `docker compose down -v`.
- **Não muda o fluxo de desenvolvimento.** O README continua descrevendo a execução nativa como o caminho normal; o container é a alternativa para quem não tem Go.

## Consequências

- O professor roda o projeto com `docker compose up -d --build`, sem instalar Go nem PostgreSQL. Quem não tiver internet na máquina pode levar as imagens prontas com `docker save` e `docker load`.
- As migrations só rodam com o volume vazio. Uma migration nova (`0003`…) não é aplicada num volume antigo: é preciso `down -v` (perde os dados) ou aplicá-la à mão. Para o uso previsto, uma demonstração, isso é aceitável.
- A senha padrão do banco (`finops`) serve só para demonstração: o banco não é publicado na máquina, só a API é, mas quem for expor o serviço deve definir `POSTGRES_PASSWORD`.
- Passa a existir mais uma forma de executar o serviço para manter funcionando. O `Dockerfile` e o compose são pequenos e não duplicam regra nenhuma.
