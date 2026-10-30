package service

import (
	"errors"
	"reflect"
	"testing"
	"time"

	"github.com/unicsul-finops/financial-operations-service/internal/domain"
)

func TestDataReferencia(t *testing.T) {
	for _, c := range []struct{ instante, dia string }{
		{"2026-12-01T02:59:59Z", "2026-11-30"},
		{"2026-12-01T03:00:00Z", "2026-12-01"},
	} {
		agora, _ := time.Parse(time.RFC3339, c.instante)
		data, err := interpretarDataReferencia(nil, agora)
		if err != nil || data.Format(time.DateOnly) != c.dia {
			t.Fatalf("hoje em GMT-03: %v %v", data, err)
		}
	}
	for _, valor := range []string{"", "2026-02-30", "2026-2-03", "10/12/2026", "2026-12-10T00:00:00Z", " 2026-12-10", "0000-01-01"} {
		if _, err := interpretarDataReferencia(&valor, time.Now()); err == nil || err.Error() != "dataReferencia deve estar no formato YYYY-MM-DD" {
			t.Errorf("aceitou data inválida %q: %v", valor, err)
		}
	}
	for _, valor := range []string{"2028-02-29", "2026-12-10", "9999-12-31"} {
		if _, err := interpretarDataReferencia(&valor, time.Now()); err != nil {
			t.Errorf("recusou %s: %v", valor, err)
		}
	}
}

func parcelaExemplo() domain.Parcela {
	return domain.Parcela{Numero: 1, Vencimento: "2026-11-30", Status: domain.StatusParcelaPendente,
		ValorParcela: dec("936.91"), Juros: dec("185"), Amortizacao: dec("751.91")}
}

func TestSituacaoParcela(t *testing.T) {
	for _, c := range []struct {
		nome, data, gravado, jurosPagos, amortizacaoPaga, desconto, status, aberto string
		dias                                                                       int
	}{
		{"antes", "2026-11-29", "PENDENTE", "0", "0", "0", "PENDENTE", "936.91", 0},
		{"vencimento", "2026-11-30", "PENDENTE", "0", "0", "0", "PENDENTE", "936.91", 0},
		{"primeiro dia", "2026-12-01", "PENDENTE", "0", "0", "0", "VENCIDA", "936.91", 1},
		{"dez dias", "2026-12-10", "PENDENTE", "0", "0", "0", "VENCIDA", "936.91", 10},
		{"parcial RN-14", "2026-12-10", "PARCIALMENTE_PAGA", "185", "293.14", "0", "VENCIDA", "458.77", 10},
		{"parcial no vencimento", "2026-11-30", "PARCIALMENTE_PAGA", "185", "293.14", "0", "PARCIALMENTE_PAGA", "458.77", 0},
		{"desconto", "2026-12-10", "PARCIALMENTE_PAGA", "0", "500", "100", "VENCIDA", "336.91", 10},
		{"paga no vencimento", "2026-11-30", "PAGA", "185", "751.91", "0", "PAGA", "0", 0},
		{"paga com desconto", "2027-12-01", "PAGA", "0", "751.91", "185", "PAGA", "0", 0},
		{"liquidação sem lançamento por parcela", "2027-12-01", "PAGA", "0", "0", "0", "PAGA", "0", 0},
	} {
		t.Run(c.nome, func(t *testing.T) {
			p := parcelaExemplo()
			p.Status = c.gravado
			p.JurosPagos = dec(c.jurosPagos)
			p.AmortizacaoPaga = dec(c.amortizacaoPaga)
			p.DescontoConcedido = dec(c.desconto)
			original := p
			data, _ := interpretarDataReferencia(&c.data, time.Now())
			got, err := calcularSituacaoParcela(p, data)
			if err != nil {
				t.Fatal(err)
			}
			if got.Status != c.status || got.DiasEmAtraso != c.dias || !got.ValorEmAberto.Equal(dec(c.aberto)) {
				t.Fatalf("situação: %s, %d dias, %s em aberto", got.Status, got.DiasEmAtraso, got.ValorEmAberto)
			}
			if !reflect.DeepEqual(p, original) {
				t.Fatal("alterou a parcela original")
			}
		})
	}
	p := parcelaExemplo()
	p.Vencimento = "2028-02-28"
	dataTexto := "2028-03-01"
	data, _ := interpretarDataReferencia(&dataTexto, time.Now())
	got, _ := calcularSituacaoParcela(p, data)
	if got.DiasEmAtraso != 2 {
		t.Fatalf("ano bissexto: %d", got.DiasEmAtraso)
	}
	p.Vencimento = "inválido"
	if _, err := calcularSituacaoParcela(p, data); err == nil {
		t.Fatal("aceitou vencimento inválido")
	}
}

type repositorioConsultaTeste struct {
	operacao   domain.Operacao
	parcelas   []domain.Parcela
	ultimaData string
	err        error
	id         int64
	numero     string
	chamadas   int
}

func (r *repositorioConsultaTeste) BuscarPorNumero(string) (domain.Operacao, error) {
	panic("não esperado")
}
func (r *repositorioConsultaTeste) CriarComParcelas(domain.NovaOperacao) (domain.Operacao, error) {
	panic("não esperado")
}
func (r *repositorioConsultaTeste) Listar() ([]domain.Operacao, error) { panic("não esperado") }
func (r *repositorioConsultaTeste) ConsultarSituacao(id int64, numero string) (domain.Operacao, []domain.Parcela, string, error) {
	r.id = id
	r.numero = numero
	r.chamadas++
	return r.operacao, r.parcelas, r.ultimaData, r.err
}

func TestSituacaoOperacao(t *testing.T) {
	for _, c := range []struct {
		nome, gravado, status string
		dias, esperado        int
	}{
		{"em dia", "ATIVA", "ATIVA", 0, 0},
		{"primeiro dia", "ATIVA", "EM_ATRASO", 1, 1},
		{"90 dias", "ATIVA", "EM_ATRASO", 90, 90},
		{"91 dias", "ATIVA", "INADIMPLENTE", 91, 91},
		{"liquidada", "LIQUIDADA", "LIQUIDADA", 0, 0},
	} {
		t.Run(c.nome, func(t *testing.T) {
			parcelas := []domain.Parcela{{DiasEmAtraso: 0}, {DiasEmAtraso: c.dias}, {DiasEmAtraso: c.dias / 2}}
			op := calcularSituacaoOperacao(domain.Operacao{Status: c.gravado}, parcelas)
			if op.Status != c.status || op.DiasEmAtraso != c.esperado {
				t.Fatalf("%s com %d dias", op.Status, op.DiasEmAtraso)
			}
		})
	}
}

func TestConsultasSituacao(t *testing.T) {
	r := &repositorioConsultaTeste{operacao: domain.Operacao{Status: "ATIVA"}, parcelas: []domain.Parcela{parcelaExemplo()}}
	s := NovoOperacaoService(r)
	for _, c := range []struct{ data, status string }{{"2026-11-30", "ATIVA"}, {"2026-12-10", "EM_ATRASO"}} {
		op, err := s.BuscarOperacao("op-2026-0001", &c.data)
		if err != nil || op.Status != c.status || r.numero != "OP-2026-0001" {
			t.Fatalf("operação %v %v", op, err)
		}
	}
	data := "2026-12-10"
	parcelas, err := s.ListarParcelas("1", &data)
	if err != nil || r.id != 1 || parcelas[0].DiasEmAtraso != 10 {
		t.Fatalf("parcelas: %v %v", parcelas, err)
	}
	if r.operacao.Status != "ATIVA" || r.parcelas[0].Status != "PENDENTE" {
		t.Fatal("alterou estado gravado")
	}
	r.operacao.Status = "LIQUIDADA"
	op, err := s.BuscarOperacao("1", &data)
	if err != nil || op.Status != "LIQUIDADA" {
		t.Fatal("alterou LIQUIDADA")
	}
	r.ultimaData = data
	if _, err := s.BuscarOperacao("1", &data); err != nil {
		t.Fatal("recusou mesma data", err)
	}
	anterior := "2026-12-09"
	for _, consulta := range []func() error{
		func() error { _, e := s.BuscarOperacao("1", &anterior); return e },
		func() error { _, e := s.ListarParcelas("1", &anterior); return e },
	} {
		if e := consulta(); e == nil || e.Error() != "dataReferencia não pode ser anterior à data da última movimentação financeira da operação" {
			t.Fatalf("data anterior: %v", e)
		}
	}
	r.ultimaData = ""
	r.operacao.Status = "ATIVA"
	s.agora = func() time.Time { v, _ := time.Parse(time.RFC3339, "2026-12-01T02:59:59Z"); return v }
	op, err = s.BuscarOperacao("1", nil)
	if err != nil || op.Status != "ATIVA" {
		t.Fatal("não usou hoje GMT-03", err)
	}
	chamadas := r.chamadas
	invalida := ""
	if _, err := s.BuscarOperacao("1", &invalida); err == nil || r.chamadas != chamadas {
		t.Fatal("data inválida chegou ao banco")
	}
	if _, err := s.BuscarOperacao("abc", nil); err == nil || r.chamadas != chamadas {
		t.Fatal("identificador inválido chegou ao banco")
	}
	r.err = domain.ErrOperacaoNaoEncontrada
	if _, err := s.ListarParcelas("99", nil); !errors.Is(err, domain.ErrOperacaoNaoEncontrada) {
		t.Fatal(err)
	}
	r.err = errors.New("falha de leitura")
	if _, err := s.BuscarOperacao("1", nil); !errors.Is(err, r.err) {
		t.Fatal(err)
	}
	r.err = nil
	r.parcelas = nil
	parcelas, err = s.ListarParcelas("1", nil)
	if err != nil || parcelas == nil || len(parcelas) != 0 {
		t.Fatal("lista vazia deve ser []")
	}
}
