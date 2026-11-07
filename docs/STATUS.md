# Estado da implementação — 5 de novembro de 2026

Este incremento inaugura o repositório. O plano completo permanece em `PLANO_IMPLEMENTACAO.md`.

| Fase | Estado |
|---|---|
| Fundação | Base local implementada; implantação, backup externo e validações operacionais pendentes |
| MVP 1 | Controle manual e relatórios iniciais funcionais; critérios completos ainda pendentes |
| MVP 2 | Não iniciado: rateio, acertos, metas, aportes e fechamento |
| v1.1 | Não iniciado: cartões, parcelas e CSV |
| v1.2 | Não iniciado: combustível e financiamentos |
| v1.3 | Não iniciado: PDF com revisão |
| v2.0 | Não iniciado: Telegram e automações |

## Validação executada

- 10 testes de aplicação com SQLite real e handlers HTTP: centavo exato; idempotência e conflito de payload; transferência sem consumo/caixa duplicado; pagamento previsto/real e conflito de versão; privacidade em listas, agregados e mutações; recorrência idempotente em fevereiro; cancelamento e auditoria; sessão/CSRF/logout; validações financeiras; competência versus caixa; limite de login. (Alguns casos pertencem ao mesmo teste.)
- 2 testes de domínio: vencimentos e limites de valores.
- 1 teste de persistência: reinício, migração, WAL, FK e integridade.
- 1 teste Python de snapshot com banco ativo em WAL, restauração e preservação de valor.
- Snapshot e restauração isolada do banco de demonstração real: 2 usuários, 3 contas, 55 lançamentos, integridade/FKs válidas e saldo consolidado preservado em 3.953.280 centavos.
- Build Go e frontend; `go vet`; Svelte/TypeScript sem erros ou avisos.
- Navegador: login, dashboard, lançamento de R$ 0,01, cancelamento, busca, ocultação de valores; layout verificado em 1440 e 360 px, sem overflow horizontal da página em 360 px.
- Detector de corrida configurado no CI, não executado localmente por ausência de CGO/compilador C. O workflow ainda não foi executado no GitHub.

## Pendências para fechar Fundação e MVP 1

1. Definir domínio, VPS, armazenamento externo e responsável pelas chaves; configurar HTTPS e backup diário; ensaiar recuperação integral a partir da cópia externa.
2. Calibrar Argon2id, carga e limites no servidor alvo; medir p95/memória com massa de 50 mil lançamentos; testar disco cheio e reinício sob carga.
3. Refinar ciclo de vida de contas/categorias/recorrências: edição e arquivamento seguros; receitas recorrentes e job de geração.
4. Implementar edição completa de pendentes e fluxo explícito de correção de pagos, fechamento/reabertura auditados na fase correspondente.
5. Filtros e paginação no SQL; hoje a API limita a resposta a 100 itens, mas carrega a competência em memória e os filtros da UI se aplicam apenas à página.
6. Refinar relatórios para reconhecimento independente de pagamento, orçamento previsto/real agregado, patrimônio e exportação aberta. O realizado atual é explicitamente limitado aos pagos/recebidos.
7. Ampliar testes ponta a ponta para provisão, contas, recorrências e configuração; auditoria formal de acessibilidade. O teste visual de 360 px não equivale a certificação WCAG.
8. Configuração de fuso, política de privacidade familiar, recuperação administrativa de acesso e nomes dos participantes reais.

Não são oferecidos pagamentos parciais, contas de múltiplos proprietários, reembolso vinculado à despesa, razão imutável de movimentos nem restauração de cancelamento nesta entrega. A tabela de transações é a fonte canônica; modificações permitidas são atômicas e auditadas. Essa simplificação deverá ser evoluída antes de rateio/faturas/pagamentos parciais.
