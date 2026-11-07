# Meu Financeiro

Primeiro incremento executável do [plano de implementação](PLANO_IMPLEMENTACAO.md), com Go, SQLite, Svelte 5, TypeScript, Vite e Tailwind CSS. Interface dark fintech em português.

**Estado: fundação e controle manual inicial. Não representa a conclusão de todas as fases do plano nem um release de produção.**

## Já funciona

- Login individual, até duas pessoas provisionadas por comando administrativo, Argon2id, sessão opaca revogável, CSRF e limite de tentativas.
- SQLite com migração embarcada, WAL, integridade referencial e banco persistente.
- Contas pessoais, saldo inicial com data e categorias personalizadas.
- Receitas e despesas em centavos, previstas ou pagas; transferências entre contas próprias sem inflar consumo.
- Pagamento integral com valor real diferente do previsto; edição de pendentes; cancelamento auditado e controle otimista de versão.
- Criação idempotente de lançamentos via `Idempotency-Key`.
- Regras mensais e geração explícita de ocorrências, sem duplicação e com ajuste de vencimento ao último dia do mês.
- Dashboard, evolução mensal, categorias, compromissos, saldo atual e fluxo de caixa.
- Visão pessoal e visão da casa. Lançamentos privados ficam fora das consultas compartilhadas; nomes/IDs de contas de outra pessoa são ocultados.
- Busca e filtros na página de transações, até 100 resultados por página; ocultação visual de valores.
- Nome e senha editáveis; alteração da senha revoga todas as sessões.
- Scripts de snapshot consistente, criptografia com age, configuração de Nginx/systemd e CI.

## Executar

Neste workspace, os artefatos já foram compilados. Para iniciar sem reinstalar ferramentas, execute `./scripts/demo.sh`.

Requisitos: Go 1.27+, Node 24+, pnpm 11.25 e Python 3.12+ para os scripts operacionais.

```sh
make install
make demo
```

Abra **http://127.0.0.1:8080** e clique em **Explorar demonstração**. A demonstração usa `data/demo.db`, dados fictícios e credenciais públicas próprias desse modo. Não misture esse banco com dados reais. Os nomes Alex e Marina são fictícios.

A aplicação compilada é servida pelo Go. Node é necessário apenas para desenvolvimento/build.

### Banco real vazio e provisionamento

O cadastro público é desabilitado. Execute o comando para cada pessoa (máximo de duas). Insira a senha sem registrá-la no histórico do shell:

```sh
cd backend
export MF_DATABASE=../data/financeiro.db
export MF_NAME='Seu nome'
export MF_EMAIL='voce@exemplo.com'
read -r -s -p 'Senha (mínimo 12 caracteres): ' MF_PASSWORD
export MF_PASSWORD
./server --provision
unset MF_PASSWORD
./server
```

A primeira sessão começa sem contas nem lançamentos. Crie a conta com saldo inicial na data de início do acompanhamento. As variáveis são lidas do ambiente; `.env` **não** é carregado implicitamente. Consulte [.env.example](.env.example).

### Desenvolvimento com atualização automática

Terminal 1:

```sh
cd backend
MF_DATABASE=../data/demo.db MF_ORIGIN=http://127.0.0.1:5173 go run ./cmd/server --demo
```

Terminal 2:

```sh
cd frontend
pnpm run dev
```

Abra `http://127.0.0.1:5173` exatamente: a proteção de origem compara protocolo, host e porta. Vite encaminha `/api` para Go. Em produção `MF_ORIGIN` deve ser a origem HTTPS pública.

### Verificar

```sh
make test
make build
cd backend && go test -race ./...
```

`-race` exige compilador C e CGO; o CI Linux executa essa verificação. Dependências estão fixadas em `go.mod`/`go.sum` e `package.json`/`pnpm-lock.yaml`.

## Regras desta versão

- **Realizado por competência:** apenas receitas recebidas e despesas integralmente pagas, agrupadas pela competência informada. Pendentes são previsão e não compõem esse realizado.
- **Fluxo de caixa:** receita/despesa paga pela data do pagamento. Transferências próprias são excluídas.
- **Saldo de conta:** saldo inicial + entradas − saídas pagas até hoje. Saldo inicial não é receita.
- **Disponível planejado:** somente no mês atual e contexto pessoal, saldo atual menos despesas pendentes vencidas ou a vencer até o fim desse mês. Ainda não há reservas/metas.
- **Pagamento:** integral nesta versão. A data de pagamento não pode estar no futuro nem anteceder a abertura da conta. O pagamento pode diferir do valor previsto, preservado no registro.
- **Visibilidade:** `private` por padrão. `household` compartilha o lançamento e seu valor; não compartilha automaticamente saldo ou outros lançamentos. Compartilhar não aplica rateio.
- **Correções:** pendentes permitem alterar valor e descrição. Pagos podem ser cancelados e substituídos; o evento de cancelamento permanece auditado. Fechamento mensal ainda não existe.
- **Recorrências:** geração acionada pelo usuário para o mês selecionado, sem job automático e sem edição retroativa.
- **Fuso:** America/Fortaleza nesta versão. Preferências de moeda/fuso são informativas até a etapa de configuração ampliada.

## Próximas entregas e limitações

Veja [docs/STATUS.md](docs/STATUS.md) para o escopo implementado e pendências. Rateio, acertos, metas, cartões, CSV/PDF, combustível, financiamentos e Telegram **ainda não estão implementados**. Edição/arquivamento de contas e categorias, edição/desativação de recorrências, exportação e filtros de servidor também permanecem pendentes.

Antes de uso real compartilhado, concluir o endurecimento operacional, configurar backup externo criptografado, executar restauração completa e validar privacidade com as duas pessoas. Não há dependência de Open Banking ou serviços pagos.

Implantação e recuperação: [docs/OPERACAO.md](docs/OPERACAO.md). Contrato implementado: [docs/API.md](docs/API.md).
