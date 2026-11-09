# Diagramas de classe

Diagramas de classe UML do **financial-operations-service**, em [Mermaid](https://mermaid.js.org/), que o GitHub renderiza direto no navegador.

## Como ler um diagrama de classe num projeto Go

Go não tem classes nem herança. A correspondência usada nos diagramas é esta:

| No diagrama | No código Go |
| --- | --- |
| Classe | `struct` com seus campos e os métodos que têm esse `struct` como receptor |
| `<<interface>>` | `interface` declarada em Go |
| Realização (`..|>`) | O `struct` tem todos os métodos da interface. Em Go isso é implícito, não existe `implements` |
| Composição (`*--`) | Campo do `struct` que é outro `struct` ou uma lista dele |
| Dependência (`..>`) | Um tipo recebe ou devolve o outro, sem guardá-lo num campo |
| `<<pacote>>` | Funções soltas de um pacote, sem receptor (ex.: `handler.CORS`) |
| `+` / `-` | Exportado (começa com maiúscula) / não exportado (começa com minúscula) |
| `Decimal` | `decimal.Decimal` da biblioteca `shopspring/decimal` ([ADR-005](../adr/ADR-005-valores-monetarios-decimal.md)) |

Os diagramas mostram os campos e os métodos que importam para entender o desenho. A lista completa de campos fica no código, em `internal/domain/`.

---

## 1. Visão estrutural: camadas e dependências

Como as peças são montadas em `cmd/api/main.go` e quem depende de quem. As setas descem do HTTP para o banco e nunca sobem ([ADR-002](../adr/ADR-002-arquitetura-em-camadas.md)).

```mermaid
classDiagram
    direction TB

    class main {
        <<pacote cmd/api>>
        +main()
        -novoRouter(h OperacaoHandler, origensCORS []string) gin.Engine
    }

    class Config {
        +Porta string
        +DatabaseURL string
        +CorsOrigens []string
    }

    class config {
        <<pacote>>
        +Carregar() Config
        -origensDoCORS(valor string) []string
    }

    class database {
        <<pacote>>
        +Conectar(databaseURL string) sql.DB
    }

    class handlerFuncs {
        <<pacote handler>>
        +CORS(origens []string) gin.HandlerFunc
        +Health(c)
        +Painel(c)
        +OpenAPI(c)
        +SwaggerUI() gin.HandlerFunc
        +RotaNaoEncontrada(c)
        +MetodoNaoPermitido(c)
    }

    class OperacaoHandler {
        -service OperacaoService
        +Criar(c)
        +Listar(c)
        +Buscar(c)
        +ListarParcelas(c)
        +PagarParcela(c)
        +SimularAmortizacao(c)
        +RegistrarAmortizacao(c)
        +ConsultarLiquidacao(c)
        +Liquidar(c)
        +ConsultarResumo(c)
        +ConsultarHistorico(c)
    }

    class OperacaoService {
        -repo RepositorioOperacoes
        -movimentacoes RepositorioMovimentacoes
        -agora func() time.Time
        +CriarOperacao(nova NovaOperacao) Operacao
        +ListarOperacoes(referencia) []Operacao
        +BuscarOperacao(identificador, referencia) Operacao
        +ListarParcelas(identificador, referencia) []Parcela
        +PagarParcela(identificador, numero, pedido) Pagamento
        +SimularAmortizacao(identificador, quantidade, modo, referencia) SimulacaoAmortizacao
        +RegistrarAmortizacao(identificador, pedido) Amortizacao
        +ConsultarLiquidacao(identificador, referencia) ValorLiquidacao
        +Liquidar(identificador, pedido) Liquidacao
        +ConsultarResumo(identificador, referencia) Resumo
        +ConsultarHistorico(identificador, referencia) []ItemHistorico
    }

    class RepositorioOperacoes {
        <<interface>>
        +BuscarPorNumero(numero string) Operacao
        +CriarComParcelas(nova NovaOperacao) Operacao
        +Listar() []Operacao
        +ConsultarSituacao(id int64, numero string) Operacao
    }

    class RepositorioMovimentacoes {
        <<interface>>
        +ConsultarCarteira(id int64, numero string) Carteira
        +ConsultarHistorico(id int64, numero string) Historico
        +Movimentar(id, numero, identificadorExterno, decidir) EventoGravado
    }

    class OperacaoRepository {
        -db sql.DB
        +BuscarPorNumero(numero) Operacao
        +Buscar(id) Operacao
        +CriarComParcelas(nova) Operacao
        +Listar() []Operacao
        +ListarParcelas(operacaoID) []Parcela
        +ConsultarSituacao(id, numero) Operacao
        +ConsultarCarteira(id, numero) Carteira
        +ConsultarHistorico(id, numero) Historico
        +Movimentar(id, numero, identificadorExterno, decidir) EventoGravado
    }

    class PostgreSQL {
        <<banco externo>>
        operacao
        parcela
        evento
        lancamento
    }

    main ..> config : carrega
    main ..> database : conecta
    main ..> handlerFuncs : registra rotas
    main ..> OperacaoHandler : cria e registra rotas
    config ..> Config : devolve
    OperacaoHandler --> OperacaoService
    OperacaoService --> RepositorioOperacoes
    OperacaoService --> RepositorioMovimentacoes
    OperacaoRepository ..|> RepositorioOperacoes
    OperacaoRepository ..|> RepositorioMovimentacoes
    OperacaoRepository --> PostgreSQL : database/sql + pgx
```

Pontos importantes do desenho:

- **O service depende de interfaces, não do repositório concreto.** `RepositorioOperacoes` e `RepositorioMovimentacoes` são declaradas no pacote `service`, que é quem as usa. Nos testes unitários o service recebe dublês em memória, sem banco.
- **Um repositório só realiza as duas interfaces.** `NovoOperacaoService` recebe um `RepositorioOperacoes` e descobre, com uma conversão de tipo, se ele também é um `RepositorioMovimentacoes` (`internal/service/operacao_service.go`). O repositório de verdade é; os dublês dos testes de consulta não precisam ser.
- **Os métodos devolvem também `error`**, e os de criação devolvem ainda um `bool` que diz se o registro é novo (`201`) ou um reenvio (`200`). Os diagramas omitem esses retornos para ficarem legíveis.
- **O `decidir` do `Movimentar`** é uma função do service que o repositório chama dentro da transação, com a operação travada (veja a seção 4).

---

## 2. Modelo de domínio: operação, parcelas e movimentações

Os tipos de `internal/domain/` que as três camadas compartilham. `Operacao` e `Parcela` espelham as tabelas `operacao` e `parcela`; `Carteira` é o retrato da operação que os cálculos usam.

```mermaid
classDiagram
    direction LR

    class Operacao {
        +ID int64
        +NumeroOperacao string
        +ClienteID string
        +TipoPessoa string
        +Status string
        +DataLiberacao string
        +ProdutoCodigo string
        +ValorAprovado Decimal
        +PrazoMeses int
        +QuantidadeParcelas int
        +TaxaJuros Decimal
        +TaxaJurosAnual Decimal
        +SistemaAmortizacao string
        +SaldoDevedor Decimal
        +IOFTotal Decimal
        +TarifaCadastro Decimal
        +ValorLiberado Decimal
        +CETMensal Decimal
        +CETAnual Decimal
        +CriadoEm string
        +DiasEmAtraso int
    }

    class Parcela {
        +ID int64
        +OperacaoID int64
        +Numero int
        +Vencimento string
        +ValorParcela Decimal
        +Amortizacao Decimal
        +Juros Decimal
        +SaldoDevedor Decimal
        +Status string
        +JurosPagos Decimal
        +AmortizacaoPaga Decimal
        +DescontoConcedido Decimal
        +ValorEmAberto Decimal
        +DiasEmAtraso int
        +MultaEmAberto Decimal
        +MoraEmAberto Decimal
        +TotalParaQuitar Decimal
    }

    class NovaOperacao {
        +Identificador string
        +Estado string
        +DefinidoEm string
        +ClienteID string
        +ValorAprovado Decimal
        +PrazoMeses int
        +TaxaMensalPercentual NullDecimal
        +SistemaAmortizacao string
        +TotalJuros Decimal
        +TotalPago Decimal
        +ValorLiberado Decimal
        +TaxaJuros Decimal
        +CETMensal Decimal
    }

    class NovaParcela {
        +Numero int
        +Vencimento string
        +ValorParcela Decimal
        +Amortizacao Decimal
        +Juros Decimal
        +SaldoDevedor Decimal
    }

    class Carteira {
        +Operacao Operacao
        +Parcelas []Parcela
        +Lancamentos []LancamentoParcela
        +UltimaMovimentacao string
        +LancamentoOperacao LancamentoParcela
    }

    class Distribuicao {
        +Multa Decimal
        +JurosMora Decimal
        +Juros Decimal
        +Amortizacao Decimal
        +Total() Decimal
    }

    class LancamentoParcela {
        +NumeroParcela int
        +Data string
        +Desconto Decimal
    }

    class NovoEvento {
        +Tipo string
        +Data string
        +Valor Decimal
        +IdentificadorExterno string
        +Origem string
        +Detalhe any
        +StatusParcelas map~int, string~
        +SaldoDevedor Decimal
        +Liquidar bool
        +EventoEncerramento bool
    }

    class NovoLancamento {
        +NumeroParcela int
        +Desconto Decimal
    }

    class ParcelaRecalculada {
        +Numero int
        +Antes ReparticaoParcela
        +Depois ReparticaoParcela
    }

    class ReparticaoParcela {
        +Juros Decimal
        +Amortizacao Decimal
        +SaldoDevedor Decimal
    }

    class EventoGravado {
        +ID int64
        +Tipo string
        +NumeroParcela int
        +RegistradoEm string
        +Detalhe []byte
    }

    class Historico {
        +Carteira Carteira
        +Eventos []EventoHistorico
    }

    class EventoHistorico {
        +ID int64
        +Tipo string
        +DataEfeito string
        +RegistradoEm string
        +Valor NullDecimal
        +IdentificadorPagamento string
        +Origem string
    }

    class LancamentoEvento {
        +NumeroParcela int
        +Desconto Decimal
    }

    NovaOperacao "1" *-- "1..*" NovaParcela : Parcelas
    Carteira "1" *-- "1" Operacao
    Carteira "1" *-- "1..*" Parcela
    Carteira "1" *-- "0..*" LancamentoParcela
    Parcela "1" *-- "0..*" LancamentoParcela : Lancamentos
    LancamentoParcela *-- Distribuicao : embutida
    NovoLancamento *-- Distribuicao : embutida
    LancamentoEvento *-- Distribuicao : embutida
    NovoEvento "1" *-- "1..*" NovoLancamento : Lancamentos
    NovoEvento "1" *-- "0..*" ParcelaRecalculada
    ParcelaRecalculada *-- ReparticaoParcela
    Historico *-- Carteira
    Historico "1" *-- "0..*" EventoHistorico
    EventoHistorico "1" *-- "0..*" LancamentoEvento
    NovaOperacao ..> Operacao : CriarComParcelas grava e devolve
    NovoEvento ..> EventoGravado : Movimentar grava e devolve
```

| Tipo | Papel |
| --- | --- |
| `NovaOperacao` / `NovaParcela` | O payload do serviço de juros já convertido pelo handler. A validação preenche os campos derivados (taxas em fração, `DefinidoEmUTC`, `QuantidadeParcelas`). |
| `Operacao` / `Parcela` | O que está gravado, mais os campos **calculados na consulta** para a `dataReferencia` (`DiasEmAtraso`, `ValorEmAberto`, `MultaEmAberto`, `MoraEmAberto`, `TotalParaQuitar`), que nunca vão para o banco (RN-11 e RN-13). |
| `Carteira` | Retrato da operação: parcelas e todos os lançamentos, lido numa transação só. É a entrada de todos os cálculos do service. |
| `Distribuicao` | Para onde foi o dinheiro, na ordem de abatimento da RN-14: multa, mora, juros, amortização. Fica **embutida** (*embedding* do Go) nos lançamentos. |
| `NovoEvento` / `NovoLancamento` | O que um pagamento, uma amortização ou uma liquidação manda gravar: o evento, os lançamentos, os novos status das parcelas e o novo saldo, numa transação ([ADR-012](../adr/ADR-012-eventos-e-lancamentos.md)). |
| `EventoGravado` | O evento já gravado. No reenvio de um `identificadorPagamento`, é o que volta, sem gravar de novo. |
| `Historico` / `EventoHistorico` / `LancamentoEvento` | O que o banco devolve para montar a linha do tempo (RN-21). |

### Status e constantes

```mermaid
classDiagram
    direction LR

    class StatusOperacao {
        <<enumeration>>
        ATIVA
        EM_ATRASO
        INADIMPLENTE
        RENEGOCIADA
        LIQUIDADA
    }

    class StatusParcela {
        <<enumeration>>
        PENDENTE
        PARCIALMENTE_PAGA
        PAGA
        VENCIDA
    }

    class TipoEvento {
        <<enumeration>>
        OPERACAO_REGISTRADA
        PAGAMENTO
        AMORTIZACAO_ANTECIPADA
        LIQUIDACAO
    }

    class SistemaAmortizacao {
        <<enumeration>>
        PRICE
        SAC
    }

    class ModoAmortizacao {
        <<enumeration>>
        ULTIMAS
        PROXIMAS
    }

    class OrigemItemHistorico {
        <<enumeration>>
        REGISTRADO
        CALCULADO
    }
```

Go não tem `enum`: são constantes `string` em `internal/domain/` e o banco confere os valores com `CHECK`. **Só são gravados** `ATIVA` e `LIQUIDADA` na operação, e `PENDENTE`, `PARCIALMENTE_PAGA` e `PAGA` na parcela. `EM_ATRASO`, `INADIMPLENTE` e `VENCIDA` são calculados na consulta (RN-11). `RENEGOCIADA` está reservado para a renegociação, que não foi implementada ([Requisitos não implementados](../../requirements/nao-implementados.md)).

---

## 3. Resultados das operações financeiras

O que cada caso de uso devolve. Os tipos com `json` no código são gravados como `evento.detalhe` (JSONB) e devolvidos iguais no reenvio.

```mermaid
classDiagram
    direction LR

    class Pagamento {
        +DataPagamento string
        +Valor Decimal
        +IdentificadorPagamento string
        +Origem string
        +Distribuicao Distribuicao
        +Parcela ParcelaAposPagar
        +Operacao OperacaoAposMover
    }

    class ParcelaAposPagar {
        +Numero int
        +Status string
        +ValorEmAberto Decimal
        +EncargosEmAberto Decimal
    }

    class OperacaoAposMover {
        +NumeroOperacao string
        +Status string
        +SaldoDevedor Decimal
    }

    class ValorLiquidacao {
        +DataReferencia string
        +ValorParaQuitar Decimal
        +SaldoDevedor Decimal
        +Vencidas []ParcelaVencidaLiquidacao
        +TotalVencidas Decimal
        +AVencer []ParcelaAVencerLiquidacao
        +ValorPresente Decimal
        +Desconto Decimal
    }

    class Liquidacao {
        +DataPagamento string
        +Valor Decimal
        +IdentificadorPagamento string
        +Vencidas []LiquidacaoVencida
        +AVencer LiquidacaoAVencer
        +Operacao OperacaoAposMover
    }

    class SimulacaoAmortizacao {
        +Modo string
        +DataReferencia string
        +QuantidadeParcelas int
        +Parcelas []ParcelaAmortizada
        +Totais TotaisAmortizacao
        +SaldoDevedorAntes Decimal
        +SaldoDevedorApos Decimal
        +QuantidadeParcelasRestantes int
        +Recalculadas []ParcelaRecalculada
    }

    class Amortizacao {
        +Modo string
        +DataPagamento string
        +QuantidadeParcelas int
        +Lancamentos []ParcelaAmortizada
        +Totais TotaisAmortizacao
        +SaldoDevedorAntes Decimal
        +SaldoDevedorApos Decimal
        +ParcelasRecalculadas []ParcelaRecalculada
    }

    class ParcelaAmortizada {
        +Numero int
        +Vencimento string
        +ValorParcela Decimal
        +ValorPresente Decimal
        +Desconto Decimal
        +Amortizacao Decimal
        +Juros Decimal
    }

    class TotaisAmortizacao {
        +ValorParcelas Decimal
        +Desconto Decimal
        +ValorPresente Decimal
        +ValorAPagar Decimal
    }

    class Resumo {
        +DataReferencia string
        +Status string
        +DiasEmAtraso int
        +TotalContratado Decimal
        +TotalPago Decimal
        +ValorAmortizado Decimal
        +JurosPagos Decimal
        +EncargosPagos Decimal
        +ValorVencido Decimal
        +SaldoDevedor Decimal
        +ValorParaQuitar Decimal
    }

    class ItemHistorico {
        +ID int64
        +Tipo string
        +Origem string
        +DataEfeito string
        +Valor Decimal
        +NumeroParcela int
        +Lancamentos []LancamentoItem
        +Descricao string
    }

    class PedidoMovimentacao {
        +Valor NullDecimal
        +DataPagamento *string
        +IdentificadorPagamento *string
        +Origem *string
    }

    class PedidoAmortizacao {
        +QuantidadeParcelas NullDecimal
        +Modo *string
        +DataPagamento *string
        +IdentificadorPagamento *string
    }

    Pagamento *-- ParcelaAposPagar
    Pagamento *-- OperacaoAposMover
    Liquidacao *-- OperacaoAposMover
    SimulacaoAmortizacao "1" *-- "1..*" ParcelaAmortizada
    SimulacaoAmortizacao *-- TotaisAmortizacao
    Amortizacao "1" *-- "1..*" ParcelaAmortizada
    Amortizacao *-- TotaisAmortizacao
    PedidoMovimentacao ..> Pagamento : PagarParcela
    PedidoMovimentacao ..> Liquidacao : Liquidar
    PedidoAmortizacao ..> Amortizacao : RegistrarAmortizacao
    ValorLiquidacao ..> Liquidacao : mesma conta
    SimulacaoAmortizacao ..> Amortizacao : mesma conta
```

- **A simulação e o registro usam a mesma conta.** `calcularAmortizacao` produz a `SimulacaoAmortizacao`; `RegistrarAmortizacao` chama a mesma função dentro da transação e grava o resultado. O mesmo vale para `calcularLiquidacao` (consulta) e `Liquidar` (registro).
- **Os pedidos usam ponteiro e `NullDecimal`** para separar o campo ausente do campo vazio ou zero. Assim `"valor": 0` é recusado como inválido e um `valor` que não veio é recusado como obrigatório, com mensagens diferentes.

---

## 4. Interação entre as classes num pagamento

O diagrama de classe mostra a estrutura; este diagrama de sequência mostra como as classes acima colaboram no caso mais delicado, o pagamento de parcela (`POST /operacoes/{id}/parcelas/{n}/pagamentos`). Amortização e liquidação seguem o mesmo caminho, trocando o `decidir`.

```mermaid
sequenceDiagram
    participant C as Cliente HTTP
    participant H as OperacaoHandler
    participant S as OperacaoService
    participant R as OperacaoRepository
    participant P as PostgreSQL

    C->>H: POST .../parcelas/1/pagamentos
    H->>H: bind JSON em MovimentacaoRequest
    H->>S: PagarParcela(id, "1", PedidoMovimentacao)
    S->>S: validarPedido (valor, data, identificador)
    S->>R: Movimentar(id, numero, identificador, decidir)
    R->>P: BEGIN
    R->>P: SELECT operacao ... FOR UPDATE
    R->>P: SELECT parcelas e lançamentos (Carteira)
    alt identificadorPagamento já usado
        R-->>S: EventoGravado existente (criado = false)
    else pagamento novo
        R->>S: decidir(Carteira)
        S->>S: calcularEncargos, distribui multa → mora → juros → amortização
        S-->>R: NovoEvento
        R->>P: INSERT evento, INSERT lancamento
        R->>P: UPDATE parcela.status, operacao.saldo_devedor
        R->>P: COMMIT
        R-->>S: EventoGravado (criado = true)
    end
    S-->>H: Pagamento, criado
    H-->>C: 201 (novo) ou 200 (reenvio)
```

O `FOR UPDATE` faz dois pagamentos simultâneos da mesma operação rodarem um depois do outro, e o `decidir` roda **dentro** da transação para calcular sobre o saldo já travado. Detalhes em [Dados, transações e idempotência](../../base-de-conhecimento/06-dados-e-consistencia.md).
