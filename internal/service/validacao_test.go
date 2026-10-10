package service

import (
	"strings"
	"testing"
	"time"

	"github.com/shopspring/decimal"
	"github.com/unicsul-finops/financial-operations-service/internal/domain"
)

// payload de exemplo do serviço de juros, o mesmo do exemplo "price" do POST no openapi.yaml.
func operacaoValida() domain.NovaOperacao {
	return domain.NovaOperacao{
		Identificador:        "OP-2026-0001",
		Estado:               "DEFINIDA",
		DefinidoEm:           "2026-10-08 15:12:40",
		ClienteID:            "52998224725",
		TipoPessoa:           "PF",
		ProdutoCodigo:        "CONSIGNADO_INSS",
		ProdutoNome:          "Crédito pessoal consignado INSS",
		ModalidadeBcb:        "Crédito pessoal consignado INSS - Prefixado",
		ValorAprovado:        dec("10000"),
		PrazoMeses:           12,
		DataLiberacao:        "2026-10-30",
		TaxaMensalPercentual: decimal.NewNullDecimal(dec("1.85")),
		TaxaAnualPercentual:  decimal.NewNullDecimal(dec("24.6")),
		FaixaRisco:           "B",
		TetoAplicado:         true,
		SistemaAmortizacao:   "PRICE",
		TotalJuros:           dec("1242.88"),
		TotalPago:            dec("11242.88"),
		IOFDiario:            dec("167.65"),
		IOFAdicional:         dec("38"),
		IOFTotal:             dec("205.65"),
		TarifaCadastro:       dec("50"),
		EncargosTotal:        dec("255.65"),
		ValorLiberado:        dec("9744.35"),
		CETMensalPercentual:  decimal.NewNullDecimal(dec("2.27")),
		CETAnualPercentual:   decimal.NewNullDecimal(dec("30.89")),
		Parcelas:             calendarioExemplo(),
	}
}

func TestValidarNovaOperacaoValida(t *testing.T) {
	nova := operacaoValida()
	nova.Identificador = " op-2026-0001 "
	nova.Estado = "definida"
	nova.TipoPessoa = " pf "
	nova.SistemaAmortizacao = "price"

	if err := validarNovaOperacao(&nova); err != nil {
		t.Fatalf("não esperava erro: %v", err)
	}

	if nova.Identificador != "OP-2026-0001" || nova.Estado != "DEFINIDA" || nova.TipoPessoa != "PF" || nova.SistemaAmortizacao != "PRICE" {
		t.Errorf("campos não foram normalizados: %q %q %q %q", nova.Identificador, nova.Estado, nova.TipoPessoa, nova.SistemaAmortizacao)
	}
}

// o juros manda percentual, o banco e a resposta guardam fração.
func TestValidarNovaOperacaoDerivados(t *testing.T) {
	nova := operacaoValida()

	if err := validarNovaOperacao(&nova); err != nil {
		t.Fatalf("não esperava erro: %v", err)
	}

	taxas := []struct {
		nome           string
		veio, esperado decimal.Decimal
	}{
		{"taxaJuros", nova.TaxaJuros, dec("0.0185")},
		{"taxaJurosAnual", nova.TaxaJurosAnual, dec("0.246")},
		{"cetMensal", nova.CETMensal, dec("0.0227")},
		{"cetAnual", nova.CETAnual, dec("0.3089")},
	}
	for _, taxa := range taxas {
		if !taxa.veio.Equal(taxa.esperado) {
			t.Errorf("%s = %s, esperava %s", taxa.nome, taxa.veio, taxa.esperado)
		}
	}

	if nova.QuantidadeParcelas != 12 || nova.PrimeiroVencimento != "2026-11-30" {
		t.Errorf("quantidade %d e primeiro vencimento %q, esperava 12 e 2026-11-30", nova.QuantidadeParcelas, nova.PrimeiroVencimento)
	}

	// 15:12:40 em GMT-03:00 é 18:12:40 em UTC.
	if esperado := time.Date(2026, 10, 8, 18, 12, 40, 0, time.UTC); !nova.DefinidoEmUTC.Equal(esperado) {
		t.Errorf("definidoEm %s, esperava %s", nova.DefinidoEmUTC, esperado)
	}
}

func TestValidarNovaOperacaoPJ(t *testing.T) {
	nova := operacaoValida()
	nova.TipoPessoa = "PJ"
	nova.ClienteID = "11222333000181"

	if err := validarNovaOperacao(&nova); err != nil {
		t.Fatalf("não esperava erro: %v", err)
	}
}

func TestValidarNovaOperacaoErros(t *testing.T) {
	casos := []struct {
		nome     string
		alterar  func(*domain.NovaOperacao)
		mensagem string
	}{
		{"identificador vazio", func(n *domain.NovaOperacao) { n.Identificador = "  " }, "operacao.identificador não pode ser vazio"},
		{"identificador formato", func(n *domain.NovaOperacao) { n.Identificador = "OP-26-1" }, "operacao.identificador deve estar no formato OP-AAAA-NNNN"},
		{"estado", func(n *domain.NovaOperacao) { n.Estado = "SIMULADA" }, "apenas operações com operacao.estado DEFINIDA podem ser registradas"},
		{"definido em", func(n *domain.NovaOperacao) { n.DefinidoEm = "08/10/2026 15:12" }, "operacao.definidoEm deve estar no formato YYYY-MM-DD HH:MM:SS ou ISO-8601 com fuso"},
		{"tipo pessoa", func(n *domain.NovaOperacao) { n.TipoPessoa = "PX" }, "cliente.tipoPessoa deve ser PF ou PJ"},
		{"documento com mascara", func(n *domain.NovaOperacao) { n.ClienteID = "529.982.247-25" }, "cliente.documento deve ter apenas dígitos"},
		{"documento vazio", func(n *domain.NovaOperacao) { n.ClienteID = "" }, "cliente.documento deve ter apenas dígitos"},
		{"pf com cnpj", func(n *domain.NovaOperacao) { n.ClienteID = "11222333000181" }, "cliente.documento deve ter 11 dígitos para PF e 14 para PJ"},
		{"produto vazio", func(n *domain.NovaOperacao) { n.ProdutoCodigo = "" }, "produto.codigo não pode ser vazio"},
		{"produto longo", func(n *domain.NovaOperacao) { n.ProdutoNome = strings.Repeat("a", 101) }, "produto.nome deve ter no máximo 100 caracteres"},
		{"modalidade vazia", func(n *domain.NovaOperacao) { n.ModalidadeBcb = " " }, "produto.modalidadeBcb não pode ser vazio"},
		{"valor zero", func(n *domain.NovaOperacao) { n.ValorAprovado = decimal.Zero }, "condicoes.valor deve ser maior que zero"},
		{"valor fracao de centavo", func(n *domain.NovaOperacao) { n.ValorAprovado = dec("10000.001") }, "condicoes.valor deve ter no máximo 2 casas decimais"},
		{"prazo zero", func(n *domain.NovaOperacao) { n.PrazoMeses = 0 }, "condicoes.prazoMeses deve estar entre 1 e 420"},
		{"prazo demais", func(n *domain.NovaOperacao) { n.PrazoMeses = 421 }, "condicoes.prazoMeses deve estar entre 1 e 420"},
		{"data liberacao", func(n *domain.NovaOperacao) { n.DataLiberacao = "30/10/2026" }, "condicoes.dataLiberacao deve estar no formato YYYY-MM-DD"},
		{"taxa ausente", func(n *domain.NovaOperacao) { n.TaxaMensalPercentual = decimal.NullDecimal{} }, "taxa.taxaFinalMes é obrigatório"},
		{"taxa negativa", func(n *domain.NovaOperacao) { n.TaxaMensalPercentual = decimal.NewNullDecimal(dec("-1")) }, "taxa.taxaFinalMes deve ser maior ou igual a zero"},
		{"taxa casas demais", func(n *domain.NovaOperacao) { n.TaxaMensalPercentual = decimal.NewNullDecimal(dec("1.85001")) }, "taxa.taxaFinalMes deve ter no máximo 4 casas decimais"},
		{"taxa mensal absurda", func(n *domain.NovaOperacao) { n.TaxaMensalPercentual = decimal.NewNullDecimal(dec("185")) }, "taxa.taxaFinalMes deve ser o percentual mensal, ex.: 1.85 para 1,85% ao mês"},
		{"taxa anual ausente", func(n *domain.NovaOperacao) { n.TaxaAnualPercentual = decimal.NullDecimal{} }, "taxa.taxaFinalAno é obrigatório"},
		{"cet ausente", func(n *domain.NovaOperacao) { n.CETAnualPercentual = decimal.NullDecimal{} }, "cet.anualPercentual é obrigatório"},
		{"faixa vazia", func(n *domain.NovaOperacao) { n.FaixaRisco = "" }, "taxa.faixaRisco não pode ser vazio"},
		{"sistema", func(n *domain.NovaOperacao) { n.SistemaAmortizacao = "SACRE" }, "sistema deve ser PRICE ou SAC"},
		{"iof negativo", func(n *domain.NovaOperacao) { n.IOFDiario = dec("-1") }, "encargos.iof.diario deve ser maior ou igual a zero"},
		{"tarifa fracao", func(n *domain.NovaOperacao) { n.TarifaCadastro = dec("50.001") }, "encargos.tarifaCadastro deve ter no máximo 2 casas decimais"},
		{"iof total", func(n *domain.NovaOperacao) { n.IOFTotal = dec("205.66") }, "encargos.iof.total deve ser diario + adicional"},
		{"encargos total", func(n *domain.NovaOperacao) { n.EncargosTotal = dec("255.66") }, "encargos.total deve ser iof.total + tarifaCadastro"},
		{"valor liberado", func(n *domain.NovaOperacao) { n.ValorLiberado = dec("10000") }, "valorLiberado deve ser condicoes.valor - encargos.total"},
		{"calendario", func(n *domain.NovaOperacao) { n.Parcelas[4].SaldoDevedor = dec("6098.70") }, "calendario: parcela 5 com saldoDevedor 6098.7, esperado 6098.75"},
		{"total juros", func(n *domain.NovaOperacao) { n.TotalJuros = dec("1242.87") }, "resumo.totalJuros deve ser a soma dos juros do calendario"},
		{"total pago", func(n *domain.NovaOperacao) { n.TotalPago = dec("11242.89") }, "resumo.totalPago deve ser a soma do valor das parcelas do calendario"},
	}

	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			nova := operacaoValida()
			c.alterar(&nova)

			err := validarNovaOperacao(&nova)
			if err == nil {
				t.Fatalf("esperava erro %q, não veio erro", c.mensagem)
			}
			if err.Error() != c.mensagem {
				t.Errorf("mensagem %q, esperava %q", err.Error(), c.mensagem)
			}
		})
	}
}

func TestInterpretarDataHora(t *testing.T) {
	casos := []struct {
		entrada  string
		esperado time.Time
	}{
		// sem fuso é GMT-03:00.
		{"2026-10-08 15:12:40", time.Date(2026, 10, 8, 18, 12, 40, 0, time.UTC)},
		// perto da meia-noite muda o dia em UTC.
		{"2026-12-31 22:00:00", time.Date(2027, 1, 1, 1, 0, 0, 0, time.UTC)},
		// com fuso explícito, o fuso enviado vale.
		{"2026-10-08T15:12:40-03:00", time.Date(2026, 10, 8, 18, 12, 40, 0, time.UTC)},
		{"2026-10-08T15:12:40Z", time.Date(2026, 10, 8, 15, 12, 40, 0, time.UTC)},
	}

	for _, c := range casos {
		momento, err := interpretarDataHora(c.entrada)
		if err != nil || !momento.Equal(c.esperado) {
			t.Errorf("interpretarDataHora(%q) = %s %v, esperava %s", c.entrada, momento.UTC(), err, c.esperado)
		}
	}

	for _, entrada := range []string{"", "2026-10-08", "08/10/2026 15:12:40", "2026-10-08T15:12:40"} {
		if _, err := interpretarDataHora(entrada); err == nil {
			t.Errorf("interpretarDataHora(%q) deveria dar erro", entrada)
		}
	}
}
