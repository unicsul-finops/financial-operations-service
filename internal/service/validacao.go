package service

import (
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/shopspring/decimal"
	"github.com/unicsul-finops/financial-operations-service/internal/domain"
)

// regras do payload do serviço de juros. a conferência do calendário fica em calendario.go.

var somenteDigitos = regexp.MustCompile(`^\d+$`)

// combinado com o serviço de juros: data/hora sem fuso é GMT-03:00 fixo, não "horário de Brasília" (ADR-009).
var fusoJuros = time.FixedZone("GMT-03:00", -3*60*60)

var cem = decimal.NewFromInt(100)

// o juros manda sem fuso ("2026-10-08 15:12:40"), mas se um dia mandar no ISO com fuso, o fuso enviado vale.
func interpretarDataHora(texto string) (time.Time, error) {
	if momento, err := time.Parse(time.RFC3339, texto); err == nil {
		return momento, nil
	}

	return time.ParseInLocation("2006-01-02 15:04:05", texto, fusoJuros)
}

func validarNovaOperacao(nova *domain.NovaOperacao) error {
	nova.Identificador = strings.ToUpper(strings.TrimSpace(nova.Identificador))
	nova.Estado = strings.ToUpper(strings.TrimSpace(nova.Estado))
	nova.DefinidoEm = strings.TrimSpace(nova.DefinidoEm)
	nova.ClienteID = strings.TrimSpace(nova.ClienteID)
	nova.TipoPessoa = strings.ToUpper(strings.TrimSpace(nova.TipoPessoa))
	nova.ProdutoCodigo = strings.ToUpper(strings.TrimSpace(nova.ProdutoCodigo))
	nova.ProdutoNome = strings.TrimSpace(nova.ProdutoNome)
	nova.ModalidadeBcb = strings.TrimSpace(nova.ModalidadeBcb)
	nova.DataLiberacao = strings.TrimSpace(nova.DataLiberacao)
	nova.FaixaRisco = strings.ToUpper(strings.TrimSpace(nova.FaixaRisco))
	nova.SistemaAmortizacao = strings.ToUpper(strings.TrimSpace(nova.SistemaAmortizacao))

	if nova.Identificador == "" {
		return errors.New("operacao.identificador não pode ser vazio")
	}
	if !formatoNumeroOperacao.MatchString(nova.Identificador) {
		return errors.New("operacao.identificador deve estar no formato OP-AAAA-NNNN")
	}
	// este serviço só registra o que o cliente já fechou no juros.
	if nova.Estado != domain.EstadoJurosDefinida {
		return errors.New("apenas operações com operacao.estado DEFINIDA podem ser registradas")
	}

	definidoEm, err := interpretarDataHora(nova.DefinidoEm)
	if err != nil {
		return errors.New("operacao.definidoEm deve estar no formato YYYY-MM-DD HH:MM:SS ou ISO-8601 com fuso")
	}
	nova.DefinidoEmUTC = definidoEm.UTC()

	if err := validarCliente(nova.ClienteID, nova.TipoPessoa); err != nil {
		return err
	}

	if err := validarTexto("produto.codigo", nova.ProdutoCodigo, 50); err != nil {
		return err
	}
	if err := validarTexto("produto.nome", nova.ProdutoNome, 100); err != nil {
		return err
	}
	if err := validarTexto("produto.modalidadeBcb", nova.ModalidadeBcb, 150); err != nil {
		return err
	}

	if !nova.ValorAprovado.IsPositive() {
		return errors.New("condicoes.valor deve ser maior que zero")
	}
	// fração de centavo não existe em dinheiro, melhor recusar do que arredondar sem avisar.
	if !nova.ValorAprovado.Equal(nova.ValorAprovado.Round(2)) {
		return errors.New("condicoes.valor deve ter no máximo 2 casas decimais")
	}
	if nova.PrazoMeses <= 0 || nova.PrazoMeses > maximoParcelas {
		return errors.New("condicoes.prazoMeses deve estar entre 1 e 420")
	}

	dataLiberacao, err := time.Parse("2006-01-02", nova.DataLiberacao)
	if err != nil {
		return errors.New("condicoes.dataLiberacao deve estar no formato YYYY-MM-DD")
	}

	if err := validarTaxas(nova); err != nil {
		return err
	}

	if nova.SistemaAmortizacao != domain.SistemaPrice && nova.SistemaAmortizacao != domain.SistemaSAC {
		return errors.New("sistema deve ser PRICE ou SAC")
	}

	if err := validarEncargos(nova); err != nil {
		return err
	}

	if err := validarCalendario(nova.Parcelas, nova.ValorAprovado, nova.PrazoMeses, dataLiberacao); err != nil {
		return err
	}

	if err := validarResumo(nova); err != nil {
		return err
	}

	nova.QuantidadeParcelas = len(nova.Parcelas)
	nova.PrimeiroVencimento = nova.Parcelas[0].Vencimento

	return nil
}

// o formato do cpf/cnpj já foi validado antes na esteira, aqui só garante que o tamanho bate com o tipo de pessoa.
func validarCliente(documento, tipoPessoa string) error {
	if tipoPessoa != "PF" && tipoPessoa != "PJ" {
		return errors.New("cliente.tipoPessoa deve ser PF ou PJ")
	}
	if !somenteDigitos.MatchString(documento) {
		return errors.New("cliente.documento deve ter apenas dígitos")
	}
	if (tipoPessoa == "PF" && len(documento) != 11) || (tipoPessoa == "PJ" && len(documento) != 14) {
		return errors.New("cliente.documento deve ter 11 dígitos para PF e 14 para PJ")
	}

	return nil
}

// o juros manda as taxas em percentual (1.85 = 1,85%). aqui elas viram fração, que é como ficam no banco e na resposta.
func validarTaxas(nova *domain.NovaOperacao) error {
	taxas := []struct {
		campo   string
		valor   decimal.NullDecimal
		destino *decimal.Decimal
	}{
		{"taxa.taxaFinalMes", nova.TaxaMensalPercentual, &nova.TaxaJuros},
		{"taxa.taxaFinalAno", nova.TaxaAnualPercentual, &nova.TaxaJurosAnual},
		{"cet.mensalPercentual", nova.CETMensalPercentual, &nova.CETMensal},
		{"cet.anualPercentual", nova.CETAnualPercentual, &nova.CETAnual},
	}

	for _, taxa := range taxas {
		if !taxa.valor.Valid {
			return fmt.Errorf("%s é obrigatório", taxa.campo)
		}
		if taxa.valor.Decimal.IsNegative() {
			return fmt.Errorf("%s deve ser maior ou igual a zero", taxa.campo)
		}
		// 4 casas em percentual viram 6 em fração, que é o que a coluna guarda.
		if !taxa.valor.Decimal.Equal(taxa.valor.Decimal.Round(4)) {
			return fmt.Errorf("%s deve ter no máximo 4 casas decimais", taxa.campo)
		}

		*taxa.destino = taxa.valor.Decimal.Div(cem)
	}

	// taxa mensal de 100% ou mais é erro de unidade, não uma taxa de verdade.
	if nova.TaxaMensalPercentual.Decimal.GreaterThanOrEqual(cem) {
		return errors.New("taxa.taxaFinalMes deve ser o percentual mensal, ex.: 1.85 para 1,85% ao mês")
	}

	return validarTexto("taxa.faixaRisco", nova.FaixaRisco, 10)
}

func validarEncargos(nova *domain.NovaOperacao) error {
	valores := []struct {
		campo string
		valor decimal.Decimal
	}{
		{"encargos.iof.diario", nova.IOFDiario},
		{"encargos.iof.adicional", nova.IOFAdicional},
		{"encargos.iof.total", nova.IOFTotal},
		{"encargos.tarifaCadastro", nova.TarifaCadastro},
		{"encargos.total", nova.EncargosTotal},
		{"valorLiberado", nova.ValorLiberado},
	}

	for _, v := range valores {
		if err := validarDinheiro(v.campo, v.valor); err != nil {
			return err
		}
	}

	if !nova.IOFTotal.Equal(nova.IOFDiario.Add(nova.IOFAdicional)) {
		return errors.New("encargos.iof.total deve ser diario + adicional")
	}
	if !nova.EncargosTotal.Equal(nova.IOFTotal.Add(nova.TarifaCadastro)) {
		return errors.New("encargos.total deve ser iof.total + tarifaCadastro")
	}
	// os encargos são descontados na liberação, não financiados. por isso o saldo devedor começa no valor cheio.
	if !nova.ValorLiberado.Equal(nova.ValorAprovado.Sub(nova.EncargosTotal)) {
		return errors.New("valorLiberado deve ser condicoes.valor - encargos.total")
	}
	if !nova.ValorLiberado.IsPositive() {
		return errors.New("valorLiberado deve ser maior que zero")
	}

	return nil
}

// o resumo não é gravado, mas se não bater com o calendário algo se perdeu no caminho.
func validarResumo(nova *domain.NovaOperacao) error {
	if err := validarDinheiro("resumo.totalJuros", nova.TotalJuros); err != nil {
		return err
	}
	if err := validarDinheiro("resumo.totalPago", nova.TotalPago); err != nil {
		return err
	}

	totalJuros := decimal.Zero
	totalPago := decimal.Zero
	for _, parcela := range nova.Parcelas {
		totalJuros = totalJuros.Add(parcela.Juros)
		totalPago = totalPago.Add(parcela.ValorParcela)
	}

	if !nova.TotalJuros.Equal(totalJuros) {
		return errors.New("resumo.totalJuros deve ser a soma dos juros do calendario")
	}
	if !nova.TotalPago.Equal(totalPago) {
		return errors.New("resumo.totalPago deve ser a soma do valor das parcelas do calendario")
	}

	return nil
}

func validarDinheiro(campo string, valor decimal.Decimal) error {
	if valor.IsNegative() {
		return fmt.Errorf("%s deve ser maior ou igual a zero", campo)
	}
	if !valor.Equal(valor.Round(2)) {
		return fmt.Errorf("%s deve ter no máximo 2 casas decimais", campo)
	}

	return nil
}

// o limite é o tamanho da coluna no banco.
func validarTexto(campo, valor string, maximo int) error {
	if valor == "" {
		return fmt.Errorf("%s não pode ser vazio", campo)
	}
	if len([]rune(valor)) > maximo {
		return fmt.Errorf("%s deve ter no máximo %d caracteres", campo, maximo)
	}

	return nil
}
