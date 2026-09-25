package service

import (
	"database/sql"
	"testing"

	"github.com/shopspring/decimal"
	"github.com/unicsul-finops/financial-operations-service/internal/domain"
)

// payload de examples/finops_operacao_create.json.
func operacaoValida() domain.NovaOperacao {
	return domain.NovaOperacao{
		ClienteID:          "12345678901",
		TipoPessoa:         "PF",
		Decisao:            "APROVADO",
		DataAprovacao:      "2026-09-25",
		ValorAprovado:      dec("15000.00"),
		PrazoMeses:         24,
		QuantidadeParcelas: 24,
		TaxaJuros:          decimal.NewNullDecimal(dec("0.0199")),
		SistemaAmortizacao: "PRICE",
		PrimeiroVencimento: "2026-10-25",
		ContextoScore: domain.ContextoScore{
			ScoreFinal:           sql.NullInt32{Int32: 872, Valid: true},
			FaixaRisco:           sql.NullString{String: "EXCELENTE"},
			ProbabilidadeDefault: decimal.NewNullDecimal(dec("0.0128")),
			Modelo:               sql.NullString{String: "SCORE_PF v1.1.0"},
			Motivo:               sql.NullString{String: "Score 872 atende ao mínimo para aprovação automática."},
		},
	}
}

func TestValidarNovaOperacaoValida(t *testing.T) {
	nova := operacaoValida()
	nova.TipoPessoa = " pf "
	nova.Decisao = "aprovado"
	nova.SistemaAmortizacao = "sac"
	nova.ContextoScore.Motivo = sql.NullString{String: "   "}

	if err := validarNovaOperacao(&nova); err != nil {
		t.Fatalf("não esperava erro: %v", err)
	}

	if nova.TipoPessoa != "PF" || nova.Decisao != "APROVADO" || nova.SistemaAmortizacao != "SAC" {
		t.Errorf("campos não foram normalizados: %q %q %q", nova.TipoPessoa, nova.Decisao, nova.SistemaAmortizacao)
	}
	if !nova.ContextoScore.FaixaRisco.Valid || nova.ContextoScore.Motivo.Valid {
		t.Errorf("texto preenchido deveria ser válido e texto em branco deveria virar NULL")
	}
}

func TestValidarNovaOperacaoSemContextoScore(t *testing.T) {
	nova := operacaoValida()
	nova.ContextoScore = domain.ContextoScore{}

	if err := validarNovaOperacao(&nova); err != nil {
		t.Fatalf("contextoScore é opcional, não esperava erro: %v", err)
	}
}

func TestValidarNovaOperacaoErros(t *testing.T) {
	casos := []struct {
		nome     string
		alterar  func(*domain.NovaOperacao)
		mensagem string
	}{
		{"cliente vazio", func(n *domain.NovaOperacao) { n.ClienteID = "  " }, "clienteId não pode ser vazio"},
		{"cliente longo", func(n *domain.NovaOperacao) { n.ClienteID = "123456789012345678901" }, "clienteId deve ter no máximo 20 caracteres"},
		{"tipo pessoa", func(n *domain.NovaOperacao) { n.TipoPessoa = "PX" }, "tipoPessoa deve ser PF ou PJ"},
		{"reprovado", func(n *domain.NovaOperacao) { n.Decisao = "REPROVADO" }, "apenas operações com decisao APROVADO podem ser registradas"},
		{"data aprovacao", func(n *domain.NovaOperacao) { n.DataAprovacao = "25/09/2026" }, "dataAprovacao deve estar no formato YYYY-MM-DD"},
		{"primeiro vencimento", func(n *domain.NovaOperacao) { n.PrimeiroVencimento = "" }, "primeiroVencimento deve estar no formato YYYY-MM-DD"},
		{"vencimento antes da aprovacao", func(n *domain.NovaOperacao) { n.PrimeiroVencimento = "2026-09-25" }, "primeiroVencimento deve ser posterior à dataAprovacao"},
		{"valor zero", func(n *domain.NovaOperacao) { n.ValorAprovado = decimal.Zero }, "valorAprovado deve ser maior que zero"},
		{"valor negativo", func(n *domain.NovaOperacao) { n.ValorAprovado = dec("-1") }, "valorAprovado deve ser maior que zero"},
		{"valor fracao de centavo", func(n *domain.NovaOperacao) { n.ValorAprovado = dec("100.005") }, "valorAprovado deve ter no máximo 2 casas decimais"},
		{"parcelas zero", func(n *domain.NovaOperacao) { n.QuantidadeParcelas = 0 }, "quantidadeParcelas deve ser maior que zero"},
		{"parcelas demais", func(n *domain.NovaOperacao) { n.QuantidadeParcelas, n.PrazoMeses = 421, 421 }, "quantidadeParcelas deve ser no máximo 420"},
		{"prazo diferente", func(n *domain.NovaOperacao) { n.PrazoMeses = 12 }, "prazoMeses deve ser igual a quantidadeParcelas"},
		{"taxa ausente", func(n *domain.NovaOperacao) { n.TaxaJuros = decimal.NullDecimal{} }, "taxaJuros é obrigatória"},
		{"taxa negativa", func(n *domain.NovaOperacao) { n.TaxaJuros = decimal.NewNullDecimal(dec("-0.01")) }, "taxaJuros deve ser maior ou igual a zero"},
		{"taxa em porcentagem", func(n *domain.NovaOperacao) { n.TaxaJuros = decimal.NewNullDecimal(dec("1.99")) }, "taxaJuros deve ser a taxa mensal em decimal, ex.: 0.0199 para 1,99% ao mês"},
		{"taxa casas demais", func(n *domain.NovaOperacao) { n.TaxaJuros = decimal.NewNullDecimal(dec("0.0199001")) }, "taxaJuros deve ter no máximo 6 casas decimais"},
		{"sistema", func(n *domain.NovaOperacao) { n.SistemaAmortizacao = "SACRE" }, "sistemaAmortizacao deve ser PRICE ou SAC"},
		{"score negativo", func(n *domain.NovaOperacao) { n.ContextoScore.ScoreFinal = sql.NullInt32{Int32: -1, Valid: true} }, "contextoScore.scoreFinal deve ser maior ou igual a zero"},
		{"probabilidade maior que 1", func(n *domain.NovaOperacao) {
			n.ContextoScore.ProbabilidadeDefault = decimal.NewNullDecimal(dec("1.5"))
		}, "contextoScore.probabilidadeDefault deve estar entre 0 e 1"},
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

func TestIdentificarOperacao(t *testing.T) {
	casos := []struct {
		entrada   string
		valor     string
		porNumero bool
	}{
		{" 3f1c2b7e-8a4d-4c6e-9b1a-2d3e4f5a6b7c ", "3f1c2b7e-8a4d-4c6e-9b1a-2d3e4f5a6b7c", false},
		{"3F1C2B7E-8A4D-4C6E-9B1A-2D3E4F5A6B7C", "3F1C2B7E-8A4D-4C6E-9B1A-2D3E4F5A6B7C", false},
		{"OP-2026-000001", "OP-2026-000001", true},
		{" op-2026-000001 ", "OP-2026-000001", true},
		{"OP-2027-1000000", "OP-2027-1000000", true},
	}

	for _, c := range casos {
		valor, porNumero, err := identificarOperacao(c.entrada)
		if err != nil || valor != c.valor || porNumero != c.porNumero {
			t.Errorf("identificarOperacao(%q) = %q %v %v, esperava %q %v", c.entrada, valor, porNumero, err, c.valor, c.porNumero)
		}
	}

	invalidos := []string{"", "abc", "OP-2026-1", "OP-26-000001", "OP-2026-00000A", "3f1c2b7e-8a4d-4c6e-9b1a"}
	for _, entrada := range invalidos {
		if _, _, err := identificarOperacao(entrada); err == nil {
			t.Errorf("identificarOperacao(%q) deveria dar erro", entrada)
		}
	}
}
