# API implementada — v0.1

Prefixo `/api/v1`, JSON, moeda em centavos inteiros, datas `YYYY-MM-DD`, competência `YYYY-MM`. Erros: `{ "error": "mensagem" }`. HTTP 400 validação, 401 sessão, 403 CSRF/origem, 404 inexistente/não autorizado, 409 versão/idempotência, 429 tentativas, 500 falha interna.

Autenticação: cookie `mf_session` HttpOnly, SameSite Strict, Secure sob origem HTTPS, 24 horas. `GET /me` fornece o token CSRF da sessão. Mutações exigem `Origin` igual a `MF_ORIGIN` e `X-CSRF-Token`; login exige origem, sem CSRF prévio.

| Método | Caminho | Contrato |
|---|---|---|
| GET | `/api/health` | Saúde, fora do prefixo v1 |
| GET | `/config` | `{demo:boolean}` |
| POST | `/sessions` | `{email,password}` → usuário e CSRF |
| DELETE | `/sessions/current` | Revoga a sessão atual |
| GET | `/me` | `{id,household_id,name,email,csrf}` |
| GET / POST | `/accounts` | Lista apenas contas próprias / cria `{name,kind,opening_cents,opening_date}` |
| GET / POST | `/categories` | Lista categorias da família / cria `{name,type}` |
| GET | `/transactions?month=2026-11&context=personal&offset=0&limit=100` | `{items,total,offset,limit}`; contexto `personal` ou `household` |
| POST | `/transactions` | Cria lançamento; obrigatório header `Idempotency-Key` de 16–100 caracteres |
| PATCH | `/transactions/{id}` | `{version,action,amount_cents?,paid_date?,description?}`; `pay`, `edit` ou `cancel`; apenas titular |
| GET | `/dashboard?month=2026-11&context=personal` | Totais, mês anterior, seis meses, categorias, saldos, recentes e próximos |
| GET / POST | `/recurring-rules` | Regras próprias / cria `{description,amount_cents,day,account_id,category_id,starts_on}` |
| POST | `/recurring-rules/generate` | `{month}` → `{created}`; idempotente por regra/competência |
| PATCH | `/settings` | `{name,current_password?,new_password?}` → `{ok,login_required}` |

Exemplo de criação pendente:

```json
{
  "type": "expense",
  "description": "Energia",
  "amount_cents": 28000,
  "account_id": 1,
  "destination_id": null,
  "category_id": 5,
  "date": "2026-11-05",
  "competence": "2026-11",
  "due_date": "2026-11-18",
  "status": "pending",
  "paid_date": null,
  "visibility": "private"
}
```

IDs são exemplos: use os retornados pelos cadastros. Tipos: `income`, `expense`, `transfer`. Transferências exigem destino próprio diferente, categoria nula, estado pago e visibilidade privada. Pendentes não possuem data de pagamento. O valor máximo por lançamento é 100.000.000.000 centavos; o mínimo é 1. Datas admitidas entre 2000 e 2100.

Pagamento preservando os R$ 280,00 previstos:

```json
{"version":1,"action":"pay","amount_cents":31740,"paid_date":"2026-11-05"}
```

`balance_cents` no dashboard é saldo **atual**, não saldo histórico do mês selecionado. `available_cents` é nulo fora do mês atual. No contexto da casa ambos são nulos e as contas são omitidas. Lançamentos compartilhados de outra pessoa retornam `account_id:0` e `account:"Conta privada"`.
