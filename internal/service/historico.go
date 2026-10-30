package service

import (
	"errors"
	"fmt"
	"sort"
	"time"

	"github.com/unicsul-finops/financial-operations-service/internal/domain"
)

func (s *OperacaoService) ConsultarHistorico(identificador string, referencia *string) ([]domain.ItemHistorico, error) {
	id, numero, err := identificarOperacao(identificador)
	if err != nil {
		return nil, err
	}
	data, err := interpretarDataReferencia(referencia, s.agora())
	if err != nil {
		return nil, err
	}
	h, err := s.movimentacoes.ConsultarHistorico(id, numero)
	if err != nil {
		return nil, err
	}
	if h.Carteira.UltimaMovimentacao != "" && data.Format(time.DateOnly) < h.Carteira.UltimaMovimentacao {
		return nil, errors.New("dataReferencia não pode ser anterior à data da última movimentação financeira da operação")
	}
	return montarHistorico(h, data)
}

// RN-21: os eventos gravados e, na dataReferencia, o atraso e a inadimplência calculados, em ordem cronológica.
// no mesmo dia o gravado vem antes do calculado.
func montarHistorico(h domain.Historico, data time.Time) ([]domain.ItemHistorico, error) {
	itens := make([]domain.ItemHistorico, 0, len(h.Eventos))
	for _, e := range h.Eventos {
		itens = append(itens, itemGravado(e))
	}
	calculados, err := itensCalculados(h.Carteira, data)
	if err != nil {
		return nil, err
	}
	itens = append(itens, calculados...)
	sort.SliceStable(itens, func(i, j int) bool {
		if itens[i].DataEfeito != itens[j].DataEfeito {
			return itens[i].DataEfeito < itens[j].DataEfeito
		}
		return itens[i].Origem == domain.OrigemRegistrado && itens[j].Origem == domain.OrigemCalculado
	})
	return itens, nil
}

func itemGravado(e domain.EventoHistorico) domain.ItemHistorico {
	item := domain.ItemHistorico{
		ID:                     e.ID,
		Tipo:                   e.Tipo,
		Origem:                 domain.OrigemRegistrado,
		DataEfeito:             e.DataEfeito,
		RegistradoEm:           e.RegistradoEm,
		IdentificadorPagamento: e.IdentificadorPagamento,
		OrigemPagamento:        e.Origem,
		Lancamentos:            []domain.LancamentoItem{},
	}
	if e.Valor.Valid {
		valor := e.Valor.Decimal
		item.Valor = &valor
	}
	for _, l := range e.Lancamentos {
		lancamento := domain.LancamentoItem{
			Multa: l.Multa, JurosMora: l.JurosMora, Juros: l.Juros, Amortizacao: l.Amortizacao, Desconto: l.Desconto,
		}
		if l.NumeroParcela > 0 {
			numero := l.NumeroParcela
			lancamento.NumeroParcela = &numero
		}
		item.Lancamentos = append(item.Lancamentos, lancamento)
	}
	return item
}

// atraso e inadimplência não são gravados (RN-11). uma parcela esteve vencida do dia seguinte ao vencimento até o dia
// em que foi quitada, ou até a dataReferencia se continua em aberto. mais de 90 dias de atraso é inadimplência (RN-12).
func itensCalculados(c domain.Carteira, data time.Time) ([]domain.ItemHistorico, error) {
	dia := data.Format(time.DateOnly)
	itens := []domain.ItemHistorico{}
	inadimplente := ""
	for _, p := range c.Parcelas {
		vencimento, err := time.ParseInLocation(time.DateOnly, p.Vencimento, fusoJuros)
		if err != nil {
			return nil, err
		}
		fim := dia
		if p.Status == domain.StatusParcelaPaga {
			// quitada por lançamentos da parcela. na liquidação as a vencer não têm lançamento próprio e não estavam vencidas.
			lancamentos := lancamentosDaParcela(c, p.Numero)
			if len(lancamentos) == 0 {
				continue
			}
			fim = lancamentos[len(lancamentos)-1].Data
		}
		inicioAtraso := vencimento.AddDate(0, 0, 1).Format(time.DateOnly)
		if inicioAtraso > fim {
			continue
		}
		numero := p.Numero
		itens = append(itens, domain.ItemHistorico{
			Tipo: domain.ItemParcelaVencida, Origem: domain.OrigemCalculado, DataEfeito: inicioAtraso, NumeroParcela: &numero,
			Lancamentos: []domain.LancamentoItem{}, Descricao: fmt.Sprintf("parcela %d vencida em %s", p.Numero, inicioAtraso),
		})
		if inicio := vencimento.AddDate(0, 0, limiteInadimplencia+1).Format(time.DateOnly); inicio <= fim && (inadimplente == "" || inicio < inadimplente) {
			inadimplente = inicio
		}
	}
	if inadimplente != "" {
		itens = append(itens, domain.ItemHistorico{
			Tipo: domain.ItemOperacaoInadimplente, Origem: domain.OrigemCalculado, DataEfeito: inadimplente,
			Lancamentos: []domain.LancamentoItem{}, Descricao: fmt.Sprintf("operação inadimplente em %s", inadimplente),
		})
	}
	return itens, nil
}
