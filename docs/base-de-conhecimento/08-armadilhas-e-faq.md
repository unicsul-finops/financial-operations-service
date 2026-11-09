# 08. Armadilhas e FAQ

Problemas de ambiente (senha do banco, porta ocupada, `psql` fora do PATH) estão no [Troubleshooting](../guides/troubleshooting.md). Aqui ficam as armadilhas do **código e das regras**.

## Armadilhas

### "Mudei o `index.html` / o `openapi.yaml` e nada mudou"

Os dois são **embutidos no binário** com `go:embed`. Salvar o arquivo não basta: pare e rode `go run ./cmd/api` de novo. No container, `docker compose up -d --build`.

### "`go run .` não funciona"

O pacote `main` está em `cmd/api`. Use `go run ./cmd/api`, **da raiz**, que é onde o `.env` é procurado.

### "O pagamento diz `dataPagamento não pode ser futura`"

`dataPagamento` é o dia em que o dinheiro entrou, então no máximo hoje. Para testar pagamento em atraso, a operação precisa ter vencimentos no passado (a coleção usa operações de 2025). Para só **ver** o atraso, use `?dataReferencia=` com uma data futura numa consulta. Veja [Testando cenários no tempo](03-ambiente-e-execucao.md#testando-cenários-no-tempo).

### "A consulta diz `dataReferencia não pode ser anterior à data da última movimentação`"

A data só anda para frente (RN-09): depois de um pagamento em 10/12, nem consultas nem pagamentos aceitam datas anteriores. Para ver o passado, use o histórico. Para recomeçar, registre outra operação.

### "Pedi a operação e o status gravado é `ATIVA`, mas a API diz `INADIMPLENTE`"

Correto. `EM_ATRASO` e `INADIMPLENTE` **não são gravados**: são calculados para a `dataReferencia` (RN-11). Olhar o banco direto mostra só `ATIVA` ou `LIQUIDADA`. O mesmo vale para `VENCIDA` nas parcelas.

### "O `CHECK` permite `VENCIDA` / `EM_ATRASO`; posso gravar?"

Não. Os valores ficaram no `CHECK` da `0001` por histórico e não são usados ([ADR-012](../architecture/adr/ADR-012-eventos-e-lancamentos.md)). Gravar um deles faria a situação calculada e a gravada divergirem.

### "Taxa de 1,85% chega como `1.85` e sai como `0.0185`"

De propósito: o juros manda em **percentual** e esta API trabalha em **fração**. A conversão acontece só em `validarTaxas`. Se for comparar com o payload do juros, multiplique por 100.

### "Mandei `100.005` e recebi `400`"

Dinheiro com mais de 2 casas é **recusado**, não arredondado ([ADR-005](../architecture/adr/ADR-005-valores-monetarios-decimal.md)).

### "O JSON mostra `185` em vez de `185.00`"

O decimal sai como número JSON, e número não guarda zero à direita. Quem consome deve formatar na exibição.

### "Usei `==` para comparar dois `decimal.Decimal` e deu falso"

`decimal.Decimal` é uma struct com expoente: `1.50` e `1.5` são iguais em valor e diferentes em `==`. Use `.Equal()`, `.IsZero()`, `.IsPositive()`, `.Cmp()`.

### "Criei uma migration e o teste de integração não enxerga a coluna"

A lista de migrations dos testes é explícita, em `bancoDeIntegracao` (`cmd/api/atraso_integration_test.go`). Acrescente o arquivo lá.

### "Criei uma migration e o container não aplicou"

O PostgreSQL do container só roda `migrations/` com o volume **vazio**. Use `docker compose down -v` (apaga os dados) ou aplique à mão.

### "Erro em inglês com `400`"

É uma falha interna (do banco, por exemplo) saindo como `400`. É uma [limitação conhecida](../requirements/nao-implementados.md#3-requisitos-não-funcionais-não-atendidos): leia a mensagem, ela costuma dizer o problema (coluna inexistente, conexão recusada).

### "Paguei R$ 500 e o saldo devedor caiu menos que isso"

O saldo devedor é só o **principal** (RN-15). Do pagamento, primeiro sai a multa, depois a mora, depois os juros; só o resto amortiza. No exemplo da RN-14, de 500,00 só 293,14 viram amortização.

### "Pedi a amortização e recebi `400` falando de parcela vencida"

A amortização antecipada exige que não haja parcela vencida em aberto nem parcialmente paga (RN-17). Pague o atraso primeiro. E se a quantidade cobrir todas as parcelas em aberto, use a liquidação.

### "Reenviei o pagamento com o mesmo `identificadorPagamento` e mudei o valor, mas voltou o valor antigo"

É a idempotência funcionando: o mesmo identificador devolve o que foi gravado na primeira vez, ignorando o corpo novo. Um pagamento novo precisa de um identificador novo ([ADR-014](../architecture/adr/ADR-014-concorrencia-e-idempotencia-das-movimentacoes.md)).

### "A pasta `bruno/` não tem os pagamentos"

`bruno/` é a coleção nativa da Fase 1. A coleção completa está em `collections/` e também abre no Bruno.

## Perguntas frequentes

**Por que o serviço não calcula o calendário de parcelas?**
Porque o juros já calcula e é o dono da precificação. Calcular de novo aqui criaria duas fontes de verdade. O serviço confere se o calendário fecha e grava como veio ([ADR-009](../architecture/adr/ADR-009-operacao-a-partir-da-simulacao-de-juros.md)). Antes era diferente ([ADR-004](../architecture/adr/ADR-004-cronograma-gerado-internamente.md), substituído).

**Por que não tem ORM?**
Para o SQL ficar explícito e a equipe não precisar aprender mais uma ferramenta ([ADR-001](../architecture/adr/ADR-001-stack-go-gin-postgres.md)). As consultas são poucas e estão todas em `internal/repository`.

**Por que o `id` é sequencial e não UUID?**
Era UUID ([ADR-003](../architecture/adr/ADR-003-id-gerado-pelo-servico.md)). Com o `numeroOperacao` do juros como identificador de negócio, o `id` virou só chave técnica, e um inteiro é mais simples ([ADR-011](../architecture/adr/ADR-011-id-sequencial.md)).

**Por que não há um job que marca as operações em atraso?**
Porque um job que falha deixa o banco errado, e porque com a situação calculada dá para simular qualquer data ([ADR-012](../architecture/adr/ADR-012-eventos-e-lancamentos.md)). O lado ruim é que ninguém é avisado na hora em que a operação fica inadimplente; veja [Integrações](../architecture/integracoes.md#o-que-não-existe-hoje-e-como-seria-acrescentado).

**Como outro grupo consome a API do navegador?**
O CORS vem liberado por padrão (`CORS_ORIGINS=*`). Basta apontar para `http://IP-DA-MAQUINA:8083` e liberar a porta no firewall ([ADR-016](../architecture/adr/ADR-016-cors-liberado-e-sem-autenticacao.md)).

**Posso rodar duas instâncias da API no mesmo banco?**
Pode. A API não guarda estado em memória, e a concorrência entre movimentações é resolvida com `FOR UPDATE` no banco ([ADR-014](../architecture/adr/ADR-014-concorrencia-e-idempotencia-das-movimentacoes.md)). Só mude a `PORT` de uma delas.

**Onde está a renegociação?**
Não foi implementada: depende de alinhamento com o grupo de juros. O que já está pronto para ela e o caminho sugerido estão em [Requisitos não implementados](../requirements/nao-implementados.md).

**As regras financeiras são reais?**
Foram tiradas de legislação e normas do Banco Central, com as fontes citadas em [Regras da Fase 2](../requirements/regras-de-negocio-fase-2.md#fontes). Mas é um projeto acadêmico: não use como parecer jurídico.

**Quem aprova uma PR?**
O arquiteto do projeto, dono do `CODEOWNERS`. Papéis e fluxo em [Governança](../development/project-governance.md).
