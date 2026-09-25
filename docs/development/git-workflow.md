# Fluxo Git

Passo a passo do dia a dia com Git neste repositório. Os papéis, o Kanban e as regras gerais estão em [project-governance.md](project-governance.md).

## Resumo

```text
Issue → Ready → Branch → Commits → Pull Request → Revisão → Squash and merge → Done
```

- Nenhum commit direto na `main`.
- Uma branch por Issue.
- Commits no padrão [Conventional Commits](https://www.conventionalcommits.org/pt-br/v1.0.0/).
- O merge é feito pelo arquiteto, com **Squash and merge**.

## 1. Atualizar a `main`

```bash
git checkout main
git pull origin main
```

## 2. Criar a branch da Issue

```text
tipo/numero-da-issue-descricao-curta
```

```bash
git checkout -b feature/12-registrar-pagamento
```

| Tipo | Uso |
| --- | --- |
| `feature/` | Nova funcionalidade |
| `fix/` | Correção de bug |
| `docs/` | Somente documentação |
| `refactor/` | Mudança interna sem alterar comportamento |
| `test/` | Somente testes |
| `chore/` | Configuração, dependências, tarefas de manutenção |

Use letras minúsculas, sem acentos nem espaços, com palavras separadas por hífen.

## 3. Desenvolver e verificar

Antes de cada commit, rode na raiz do projeto:

```bash
gofmt -l .        # não deve listar nenhum arquivo
go vet ./...
go test ./...
```

Se `gofmt -l .` listar arquivos, formate com `gofmt -w .`.

Se a tarefa alterar o banco, crie uma **nova** migration em `migrations/` com o próximo número. Nunca edite uma migration existente ([ADR-007](../architecture/adr/ADR-007-migrations-sql-versionadas.md)).

Se a tarefa envolver uma decisão técnica relevante, registre um ADR em [`docs/architecture/adr`](../architecture/adr/README.md).

## 4. Commitar

```text
tipo(escopo): descrição curta no imperativo
```

```bash
git add internal/service/pagamento_service.go internal/service/pagamento_service_test.go
git commit -m "feat(pagamentos): registra pagamento total de parcela"
```

Escopos sugeridos: `operations`, `cronograma`, `pagamentos`, `database`, `api`, `docs`, `config`.

Evite `git add .` sem conferir o `git status` antes: arquivos como o `.env` nunca devem ir para o repositório.

## 5. Enviar e abrir a Pull Request

```bash
git push -u origin feature/12-registrar-pagamento
```

Abra a PR para a `main` pelo GitHub e inclua na descrição:

- o que foi feito;
- `Closes #12`, para fechar a Issue no merge;
- como testar (requisições de exemplo, migrations a aplicar);
- evidências, quando fizer sentido.

## 6. Revisão

- Responda aos comentários com novos commits na mesma branch. Não reescreva o histórico de uma PR em revisão.
- Se a `main` avançar e gerar conflito, atualize a branch:

```bash
git checkout main
git pull origin main
git checkout feature/12-registrar-pagamento
git merge main
```

## 7. Depois do merge

```bash
git checkout main
git pull origin main
git branch -d feature/12-registrar-pagamento
```

## Proibido

- Push direto ou `--force` na `main`.
- Commitar `.env`, senhas, tokens ou arquivos gerados.
- Editar migrations já aplicadas.
