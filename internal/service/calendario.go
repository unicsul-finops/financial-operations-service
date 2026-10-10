package service

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/shopspring/decimal"
	"github.com/unicsul-finops/financial-operations-service/internal/domain"
)

// limite só pra barrar payload absurdo, 35 anos é o prazo de um financiamento imobiliário longo.
const maximoParcelas = 420

// o calendário vem pronto do serviço de juros e é gravado como veio (ADR-009).
// aqui só confere se as contas fecham, sem refazer o cálculo dos juros, que é regra deles.
func validarCalendario(parcelas []domain.NovaParcela, valor decimal.Decimal, prazo int, dataLiberacao time.Time) error {
	if len(parcelas) == 0 {
		return errors.New("calendario não pode ser vazio")
	}
	if len(parcelas) > maximoParcelas {
		return errors.New("calendario deve ter no máximo 420 parcelas")
	}
	// o calendário é mensal, então prazo diferente da quantidade de parcelas é contradição no payload.
	if len(parcelas) != prazo {
		return errors.New("calendario deve ter uma parcela por mês de condicoes.prazoMeses")
	}

	saldo := valor
	somaAmortizacao := decimal.Zero
	vencimentoAnterior := dataLiberacao

	for i := range parcelas {
		parcela := &parcelas[i]
		parcela.Vencimento = strings.TrimSpace(parcela.Vencimento)

		if parcela.Numero != i+1 {
			return fmt.Errorf("calendario: parcela na posição %d deveria ter numero %d", i+1, i+1)
		}

		vencimento, err := time.Parse("2006-01-02", parcela.Vencimento)
		if err != nil {
			return fmt.Errorf("calendario: parcela %d com vencimento fora do formato YYYY-MM-DD", parcela.Numero)
		}
		// a primeira compara com a liberação, as demais com a parcela anterior.
		if !vencimento.After(vencimentoAnterior) {
			return fmt.Errorf("calendario: parcela %d deve vencer depois da anterior e da dataLiberacao", parcela.Numero)
		}
		vencimentoAnterior = vencimento

		for _, campo := range []decimal.Decimal{parcela.ValorParcela, parcela.Amortizacao, parcela.Juros, parcela.SaldoDevedor} {
			if campo.IsNegative() || !campo.Equal(campo.Round(2)) {
				return fmt.Errorf("calendario: parcela %d com valor negativo ou com mais de 2 casas decimais", parcela.Numero)
			}
		}
		if !parcela.ValorParcela.IsPositive() {
			return fmt.Errorf("calendario: parcela %d com valor zerado", parcela.Numero)
		}
		if !parcela.ValorParcela.Equal(parcela.Amortizacao.Add(parcela.Juros)) {
			return fmt.Errorf("calendario: parcela %d com valor diferente de amortizacao + juros", parcela.Numero)
		}

		saldo = saldo.Sub(parcela.Amortizacao)
		if !parcela.SaldoDevedor.Equal(saldo) {
			return fmt.Errorf("calendario: parcela %d com saldoDevedor %s, esperado %s", parcela.Numero, parcela.SaldoDevedor, saldo)
		}

		somaAmortizacao = somaAmortizacao.Add(parcela.Amortizacao)
	}

	// com o saldo encadeado, saldo final zero já garante que a soma das amortizações é o valor.
	if !saldo.IsZero() {
		return fmt.Errorf("calendario: soma das amortizações %s diferente de condicoes.valor %s", somaAmortizacao, valor)
	}

	return nil
}
