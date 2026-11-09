# 07. Testes e qualidade

## As três camadas de teste

| Tipo | Onde | Precisa de banco? | Cobre |
| --- | --- | --- | --- |
| **Unitário** | `internal/service/*_test.go`, `internal/config/config_test.go` | Não | Toda regra de cálculo e validação, com carteiras montadas em memória e o relógio fixo |
| **Router e contrato** | `cmd/api/main_test.go` | Não | `openapi.yaml` válido, toda rota documentada, exemplos válidos contra os schemas, painel e Swagger respondendo, `404`/`405` em JSON, os três modos de CORS |
| **Integração** | `cmd/api/*_integration_test.go` | **Sim** | O fluxo HTTP completo contra um PostgreSQL de verdade: atraso, pagamento, amortização, liquidação, resumo, histórico e regressões |
| **Ponta a ponta manual** | `collections/` (Postman/Bruno) | Sim, com a API rodando | O contrato do jeito que outro grupo o usa, inclusive todos os erros |

```powershell
go test ./...                       # unitários e de contrato; os de integração aparecem como SKIP
$env:FINOPS_TEST_DATABASE_URL = "postgres://postgres:SENHA@localhost:5432/finops?sslmode=disable"
go test ./cmd/api -v                # agora com os de integração
```

## Como os testes de integração isolam os dados

`bancoDeIntegracao` (em `cmd/api/atraso_integration_test.go`) não toca nas suas operações de desenvolvimento:

1. conecta em `FINOPS_TEST_DATABASE_URL`;
2. cria um **schema temporário** com nome único (`finops_test_<nanossegundos>`);
3. aplica as migrations listadas **nesse schema** (via `search_path`);
4. no fim do teste, `DROP SCHEMA ... CASCADE`.

O usuário do banco precisa de permissão para criar schemas. Pode usar o mesmo banco `finops` do desenvolvimento. **Ao criar uma migration nova, acrescente-a na lista** dentro de `bancoDeIntegracao`; senão os testes rodam com o schema antigo.

Nunca publique a URL de teste com a senha (em PR, Issue ou log de CI).

## Como escrever um bom teste aqui

- **Use os números das RNs.** As regras têm exemplos calculados à mão (o pagamento de 500,00 em 10/12/2026, a parcela de 936,91). O teste deve reproduzi-los.
- **Compare ao centavo, sem tolerância**: `if !got.Equal(decimal.RequireFromString("458.77"))`. Uma diferença de 0,01 é um bug.
- **Teste a função pura**, não a casca: `decidirPagamento(carteira, 1, movimento)` em vez de subir o router.
- **Fixe a data.** Passe a data como parâmetro; no service, use o `agora` injetado. Teste que depende do dia em que roda vai quebrar em algum dia.
- **Teste o caminho de erro com a mensagem exata**, porque a mensagem faz parte do contrato.
- Para um bug encontrado, escreva primeiro o teste que falha, depois corrija. Os casos de regressão vão em `cmd/api/regressao_integration_test.go` quando envolvem o banco.

## Guardiões do contrato

O `openapi.yaml` é a fonte oficial do contrato ([ADR-008](../architecture/adr/ADR-008-openapi-escrito-a-mao.md)), e os testes impedem que ele fique para trás:

| Teste | Falha quando |
| --- | --- |
| `TestOpenAPIValido` | O YAML é inválido ou um exemplo não bate com o schema |
| `TestOpenAPICobreTodasAsRotas` | Uma rota existe no router e não no contrato, ou o contrário (exceto as listadas em `rotasSoNoContrato`) |
| `TestErrosDeRotaEmJSON` | Uma rota inexistente ou um método errado não devolve `{"erro": ...}` |
| `TestCORSLiberado` / `Restrito` / `Desligado` | O comportamento de `CORS_ORIGINS` muda |

## Antes de abrir a PR

```powershell
gofmt -l .          # não deve listar nada
go vet ./...
go test ./...
go test ./cmd/api -v   # com FINOPS_TEST_DATABASE_URL definida
```

E também:

- [ ] Rodou a coleção em `collections/` inteira contra a API local, na ordem, e tudo passou?
- [ ] Mudou o contrato? `openapi.yaml`, exemplos, coleção e [Contratos](../architecture/contracts/README.md) atualizados.
- [ ] Mudou o banco? Migration **nova**, lista do `bancoDeIntegracao`, README e diagrama de dados.
- [ ] Mudou uma regra? RN atualizada, com a fonte.
- [ ] Decisão difícil de reverter? ADR.
- [ ] Mudou como alguém trabalha no projeto? Artigo desta base atualizado.
- [ ] Commit no padrão [Conventional Commits](https://www.conventionalcommits.org/pt-br/v1.0.0/), em português (`feat(pagamentos): ...`), citando a Issue.

O template da PR está em `.github/pull_request_template.md`, e a revisão é do arquiteto (`.github/CODEOWNERS`). Detalhes em [Fluxo Git](../development/git-workflow.md).
