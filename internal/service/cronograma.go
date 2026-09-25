package service

import (
	"errors"
	"time"

	"github.com/shopspring/decimal"
	"github.com/unicsul-finops/financial-operations-service/internal/domain"
)

// casas usadas nas divisões intermediárias, antes de arredondar pro centavo.
const precisaoIntermediaria = 16

// gera as parcelas mensais da operação. não acessa banco, só faz a conta.
func GerarCronograma(valor, taxa decimal.Decimal, quantidade int, primeiroVencimento time.Time, sistema string) ([]domain.NovaParcela, error) {
	if quantidade <= 0 {
		return nil, errors.New("quantidadeParcelas deve ser maior que zero")
	}

	var parcelaFixa, amortizacaoFixa decimal.Decimal

	switch sistema {
	case domain.SistemaPrice:
		pmt, err := calcularParcelaPrice(valor, taxa, quantidade)
		if err != nil {
			return nil, err
		}
		parcelaFixa = pmt
	case domain.SistemaSAC:
		amortizacaoFixa = valor.DivRound(decimal.NewFromInt(int64(quantidade)), precisaoIntermediaria).Round(2)
	default:
		return nil, errors.New("sistemaAmortizacao deve ser PRICE ou SAC")
	}

	parcelas := make([]domain.NovaParcela, 0, quantidade)
	saldo := valor

	for numero := 1; numero <= quantidade; numero++ {
		// arredonda a cada parcela, que é como o valor aparece no boleto.
		juros := saldo.Mul(taxa).Round(2)

		amortizacao := amortizacaoFixa
		if sistema == domain.SistemaPrice {
			amortizacao = parcelaFixa.Sub(juros)
		}

		// a última parcela absorve os centavos que sobraram dos arredondamentos, pra fechar o saldo em zero.
		if numero == quantidade {
			amortizacao = saldo
		}

		saldo = saldo.Sub(amortizacao)

		parcelas = append(parcelas, domain.NovaParcela{
			Numero:       numero,
			Vencimento:   adicionarMeses(primeiroVencimento, numero-1).Format("2006-01-02"),
			ValorParcela: amortizacao.Add(juros),
			Amortizacao:  amortizacao,
			Juros:        juros,
			SaldoDevedor: saldo,
		})
	}

	return parcelas, nil
}

// PMT = P * i * (1+i)^n / ((1+i)^n - 1)
func calcularParcelaPrice(valor, taxa decimal.Decimal, quantidade int) (decimal.Decimal, error) {
	n := decimal.NewFromInt(int64(quantidade))

	// sem juros a fórmula divide por zero, e a parcela é só o valor dividido igualmente.
	if taxa.IsZero() {
		return valor.DivRound(n, precisaoIntermediaria).Round(2), nil
	}

	um := decimal.NewFromInt(1)

	// expoente inteiro deixa a potência exata, sem aproximação.
	fator, err := um.Add(taxa).PowInt32(int32(quantidade))
	if err != nil {
		return decimal.Decimal{}, err
	}

	return valor.Mul(taxa).Mul(fator).DivRound(fator.Sub(um), precisaoIntermediaria).Round(2), nil
}

// soma meses sempre a partir da data base, senão um vencimento dia 31 vira dia 28 pra sempre depois de fevereiro.
// o time.AddDate não serve aqui porque 31/01 + 1 mês cai em 03/03.
func adicionarMeses(base time.Time, meses int) time.Time {
	primeiroDoMes := time.Date(base.Year(), base.Month()+time.Month(meses), 1, 0, 0, 0, 0, time.UTC)
	ultimoDia := primeiroDoMes.AddDate(0, 1, -1).Day()

	return time.Date(primeiroDoMes.Year(), primeiroDoMes.Month(), min(base.Day(), ultimoDia), 0, 0, 0, 0, time.UTC)
}
