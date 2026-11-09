# 04. Como fazer (receitas)

Passo a passo das mudanças mais comuns. Todas começam do mesmo jeito: **Issue → branch → PR** ([Fluxo Git](../development/git-workflow.md)). Nada vai direto para a `main`.

## Criar um endpoint novo

O projeto trabalha **com o contrato primeiro** ([guia de convenções](../architecture/contracts/guia-de-convencoes.md#como-trabalhar-contrato-primeiro)): a primeira PR só tem o contrato, a segunda tem o código.

**PR 1: contrato**

1. Descreva a rota em `api/openapi.yaml`: `summary`, `description` citando as RNs, `operationId`, schemas com `required`, um exemplo de sucesso e **um exemplo nomeado para cada mensagem de erro**. Reaproveite os parâmetros que já existem (`IdentificadorOperacao`, `dataReferencia`).
2. Como o código ainda não existe, acrescente a rota em `rotasSoNoContrato` (`cmd/api/main_test.go`), senão `TestOpenAPICobreTodasAsRotas` falha.
3. Acrescente os pedidos na coleção `collections/financial-operations-service.postman_collection.json`, com testes do status e dos valores principais.
4. Atualize a tabela de endpoints e a de erros em [Contratos da API](../architecture/contracts/README.md).
5. `go test ./...` valida o YAML e os exemplos contra os schemas.

**PR 2: implementação**

1. **domain:** os tipos novos (resultado, pedido) em `internal/domain/`. Se o resultado for gravado em `evento.detalhe`, use tags `json`.
2. **service:** o caso de uso como método de `OperacaoService` e a conta como **função pura** separada (`calcularX(carteira, data)`). Validação de entrada antes de abrir transação; regras que dependem da operação dentro do `decidir`.
3. **repository:** só se precisar de SQL novo. Para gravar dinheiro, **não crie outra transação**: use `Movimentar` com um `decidir` novo (veja abaixo).
4. **handler:** struct de request e de response, método em `OperacaoHandler`, `responderErro` no erro. `201` + `Location` quando grava, `200` no reenvio.
5. **router:** a linha em `novoRouter` (`cmd/api/main.go`) e **remova** a rota de `rotasSoNoContrato`.
6. **testes:** unitário da função pura com os números da RN, ao centavo; integração em `cmd/api/` se houver SQL novo.
7. **painel**, se a ação deve aparecer na interface: `web/index.html`.

## Gravar uma movimentação de dinheiro nova (ex.: estorno)

Toda escrita de dinheiro passa por `OperacaoRepository.Movimentar` ([ADR-014](../architecture/adr/ADR-014-concorrencia-e-idempotencia-das-movimentacoes.md)). Você só escreve o `decidir`:

```go
func (s *OperacaoService) Estornar(identificador string, pedido domain.PedidoMovimentacao) (domain.Estorno, bool, error) {
    id, numero, err := identificarOperacao(identificador)
    if err != nil { return domain.Estorno{}, false, err }
    m, err := s.validarPedido(pedido) // valida corpo antes da transação
    if err != nil { return domain.Estorno{}, false, err }

    var estorno domain.Estorno
    gravado, novo, err := s.movimentacoes.Movimentar(id, numero, m.identificador,
        func(c domain.Carteira) (domain.NovoEvento, error) {
            // c já está travada (FOR UPDATE). calcule aqui, sem I/O.
            var evento domain.NovoEvento
            estorno, evento, err = decidirEstorno(c, m)
            return evento, err
        })
    if err != nil { return domain.Estorno{}, false, err }
    if !novo { /* reenvio: confira o tipo e faça json.Unmarshal(gravado.Detalhe, &estorno) */ }
    return estorno, novo, nil
}
```

E ainda:

- Uma **migration nova** acrescentando o tipo ao `CHECK` de `evento.tipo` (veja a próxima receita).
- A constante do tipo em `internal/domain/movimentacao.go`.
- O tratamento do tipo em `itemGravado` (`internal/service/historico.go`), para aparecer no histórico.
- Conferir se `calcularResumo` deve somar o novo tipo.
- Usar como modelo `PagarParcela` e `decidirPagamento` em `internal/service/movimentacao.go`.

## Mudar o banco (migration)

As migrations **nunca são editadas** depois de entrarem na `main` ([ADR-007](../architecture/adr/ADR-007-migrations-sql-versionadas.md)).

1. Crie `migrations/000N_descricao_curta.sql` com o próximo número.
2. Envolva em `BEGIN; ... COMMIT;`, como as existentes, e comente o **porquê** de cada coluna que não for óbvia.
3. Prefira mudanças que só **acrescentam** (tabela, coluna com `DEFAULT`, índice). Remover ou renomear quebra quem já tem o banco.
4. **Acrescente o arquivo na lista de `bancoDeIntegracao`** em `cmd/api/atraso_integration_test.go`. Os testes de integração aplicam as migrations a partir dessa lista, não da pasta.
5. Atualize o `SELECT` em `colunasOperacao` / `colunasParcela` e as funções `scanOperacao` / `scanParcela` (`internal/repository/operacao_repository.go`) se a coluna precisar ser lida.
6. Documente: tabela de migrations no [README](../../README.md#3-preparar-o-banco), [modelo de dados](../architecture/diagrams/README.md#modelo-de-dados) e, se mudar um tipo, o [diagrama de classes](../architecture/diagrams/classes.md).
7. Avise o time no PR: cada um aplica a migration nova no próprio banco com `psql -f`. No container, é preciso `docker compose down -v`.

## Mudar uma regra de cálculo

1. Encontre a função em [Regras de negócio no código](05-regras-e-calculos.md).
2. **Escreva primeiro o teste** com o exemplo numérico da RN, ao centavo, sem tolerância.
3. Mude a função pura. Se a regra mudar de verdade (não é correção de bug), atualize a RN em [regras-de-negocio-fase-2.md](../requirements/regras-de-negocio-fase-2.md), com a fonte legal.
4. Rode `go test ./...` e a coleção inteira: várias regras se apoiam umas nas outras (encargos → pagamento → resumo → liquidação).
5. Se o JSON mudar, é mudança de contrato: `openapi.yaml`, exemplos e coleção.

> **Atenção com dados já gravados:** a distribuição dos pagamentos antigos está gravada nos lançamentos e **não é recalculada**. Uma regra nova vale dali para frente. Se ela mudar a interpretação do que já foi gravado (por exemplo, como os encargos em aberto são calculados), discuta com o arquiteto antes.

## Mudar o contrato com o serviço de juros

O payload do `POST /operacoes` é **do juros**, não nosso. Qualquer mudança é combinada com o grupo deles antes.

1. Combine o novo formato com o grupo de juros e registre o combinado na Issue.
2. Atualize `CriarOperacaoRequest` e as structs aninhadas (`internal/handler/operacao_handler.go`) e `paraNovaOperacao`.
3. Valide em `validarNovaOperacao` (`internal/service/validacao.go`). Campo novo **opcional** primeiro, para não quebrar o juros enquanto ele não muda o lado dele.
4. Se for gravado: migration, `NovaOperacao`, `Operacao`, `CriarComParcelas`, `scanOperacao` e a resposta do `GET`.
5. Atualize os exemplos `price` e `sac` do `openapi.yaml`, o `examples/juros_inbound.json` e a coleção.
6. Mudança que altera uma decisão (ex.: passar a recalcular o calendário) vira um ADR novo que substitui o [ADR-009](../architecture/adr/ADR-009-operacao-a-partir-da-simulacao-de-juros.md).

## Acrescentar uma variável de ambiente

1. Campo em `Config` e leitura em `Carregar` (`internal/config/config.go`), com padrão quando fizer sentido.
2. Teste em `internal/config/config_test.go`.
3. `.env.example`, a tabela do [README](../../README.md#4-configurar-o-ambiente) e o `environment` do `docker-compose.yml`.
4. **Nunca** coloque segredo com valor real no `.env.example` nem no compose.

## Mudar o painel

1. Edite `web/index.html`. É JavaScript puro, sem build ([ADR-015](../architecture/adr/ADR-015-painel-web-embutido.md)).
2. Reinicie a API (`go run ./cmd/api`): o HTML vai embutido no binário, então salvar o arquivo não basta.
3. Chame a API sempre pela função `api('/operacoes/...')`, com caminho relativo.
4. Não coloque regra de negócio no painel: se precisar de um número, ele deve vir da API.
5. Atualize os prints em `docs/assets/` se a tela mudar de forma visível.

## Escrever um ADR

Quando a decisão afetar mais de uma camada ou o contrato com outro grupo, for difícil de reverter, ou tiver alternativa que alguém vai questionar depois. Modelo e regras em [ADRs](../architecture/adr/README.md#como-escrever). Um ADR aceito não é editado para mudar a decisão: crie outro e marque o antigo como substituído.
