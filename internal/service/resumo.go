package service

import (
	"errors"
	"time"

	"github.com/unicsul-finops/financial-operations-service/internal/domain"
)

func (s *OperacaoService) ConsultarResumo(identificador string, referencia *string) (domain.Resumo, error) {
	id, numero, err := identificarOperacao(identificador)
	if err != nil {
		return domain.Resumo{}, err
	}
	data, err := interpretarDataReferencia(referencia, s.agora())
	if err != nil {
		return domain.Resumo{}, err
	}
	c, err := s.movimentacoes.ConsultarCarteira(id, numero)
	if err != nil {
		return domain.Resumo{}, err
	}
	if c.UltimaMovimentacao != "" && data.Format(time.DateOnly) < c.UltimaMovimentacao {
		return domain.Resumo{}, errors.New("dataReferencia não pode ser anterior à data da última movimentação financeira da operação")
	}
	return calcularResumo(c, data)
}

// RN-20: tudo somado a partir dos lançamentos e das parcelas, na data de referência.
func calcularResumo(c domain.Carteira, data time.Time) (domain.Resumo, error) {
	op := c.Operacao
	r := domain.Resumo{
		NumeroOperacao:  op.NumeroOperacao,
		DataReferencia:  data.Format(time.DateOnly),
		TotalContratado: op.ValorAprovado,
		SaldoDevedor:    op.SaldoDevedor,
	}

	// o lançamento da operação (parcelas a vencer da liquidação) entra junto com os das parcelas.
	pago := append([]domain.LancamentoParcela{c.LancamentoOperacao}, c.Lancamentos...)
	for _, l := range pago {
		r.TotalPago = r.TotalPago.Add(l.Total())
		r.ValorAmortizado = r.ValorAmortizado.Add(l.Amortizacao)
		r.JurosPagos = r.JurosPagos.Add(l.Juros)
		r.EncargosPagos = r.EncargosPagos.Add(l.Multa).Add(l.JurosMora)
		r.DescontoAntecipacao = r.DescontoAntecipacao.Add(l.Desconto)
	}

	// as parcelas pagas já entram no total previsto: a repartição muda na amortização, o valor não.
	situacao := make([]domain.Parcela, 0, len(c.Parcelas))
	for _, p := range c.Parcelas {
		r.TotalPrevisto = r.TotalPrevisto.Add(p.Juros).Add(p.Amortizacao)
		if p.Status == domain.StatusParcelaPaga {
			situacao = append(situacao, p)
			continue
		}
		e, err := calcularEncargos(p, lancamentosDaParcela(c, p.Numero), data)
		if err != nil {
			return domain.Resumo{}, err
		}
		if e.valorEmAberto.IsPositive() && e.diasEmAtraso > 0 {
			p.DiasEmAtraso = e.diasEmAtraso
			r.ValorVencido = r.ValorVencido.Add(e.valorEmAberto)
			r.EncargosEmAberto = r.EncargosEmAberto.Add(e.multaEmAberto).Add(e.moraEmAberto)
		}
		situacao = append(situacao, p)
	}
	calculada := calcularSituacaoOperacao(op, situacao)
	r.Status, r.DiasEmAtraso = calculada.Status, calculada.DiasEmAtraso

	if op.Status != domain.StatusOperacaoLiquidada {
		v, err := calcularLiquidacao(c, data)
		if err != nil {
			return domain.Resumo{}, err
		}
		r.ValorParaQuitar = v.ValorParaQuitar
	}
	return r, nil
}
