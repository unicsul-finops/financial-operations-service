# imagem da API para rodar numa máquina sem Go instalado (ver ADR-013).
# o desenvolvimento do dia a dia continua nativo; o container existe para a entrega.

# ---- compilação ----
FROM golang:1.26-alpine AS build
WORKDIR /src

# as dependências primeiro, pra o cache só se perder quando o go.mod mudar.
COPY go.mod go.sum ./
RUN go mod download

# o painel (web/) e o openapi.yaml (api/) são embutidos no binário, então precisam estar na cópia.
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/finops ./cmd/api

# ---- execução ----
FROM alpine:3
RUN apk add --no-cache ca-certificates wget \
    && adduser -D -u 10001 finops
USER finops

COPY --from=build /out/finops /usr/local/bin/finops

# DATABASE_URL e CORS_ORIGINS vêm do docker-compose.yml.
ENV PORT=8083
EXPOSE 8083

HEALTHCHECK --interval=10s --timeout=3s --start-period=10s --retries=5 \
    CMD wget -qO- "http://127.0.0.1:${PORT}/health" >/dev/null || exit 1

ENTRYPOINT ["finops"]
