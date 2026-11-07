<script lang="ts">
  import { onMount, tick } from 'svelte';
  import Icon from './lib/Icon.svelte';
  import type {
    User,
    Account,
    Category,
    Transaction,
    Dashboard,
    Rule,
  } from './lib/types';
  let user = $state<User | null>(null),
    demo = $state(false),
    boot = $state(true),
    loading = $state(false),
    busy = $state(false),
    error = $state(''),
    notice = $state('');
  let page = $state('overview'),
    context = $state('personal'),
    hidden = $state(false),
    menu = $state(false);
  const localToday = () =>
    new Intl.DateTimeFormat('en-CA', {
      timeZone: 'America/Fortaleza',
      year: 'numeric',
      month: '2-digit',
      day: '2-digit',
    }).format(new Date());
  let dateToday = $state(localToday()),
    month = $state(localToday().slice(0, 7));
  let email = $state(''),
    password = $state(''),
    accounts = $state<Account[]>([]),
    categories = $state<Category[]>([]),
    transactions = $state<Transaction[]>([]),
    rules = $state<Rule[]>([]),
    dashboard = $state<Dashboard | null>(null);
  let total = $state(0),
    offset = $state(0),
    search = $state(''),
    typeFilter = $state('all'),
    stateFilter = $state('all'),
    categoryFilter = $state('all'),
    accountFilter = $state('all');
  let dialog: HTMLDialogElement;
  let modal = $state(''),
    selected = $state<Transaction | null>(null),
    modalError = $state('');
  let amount = $state(''),
    description = $state(''),
    kind = $state('expense'),
    account = $state(''),
    destination = $state(''),
    category = $state(''),
    date = $state(localToday()),
    due = $state(localToday()),
    competence = $state(localToday().slice(0, 7)),
    status = $state('paid'),
    visibility = $state('private'),
    day = $state(10),
    accountKind = $state('checking');
  let name = $state(''),
    currentPassword = $state(''),
    newPassword = $state('');
  let requestKey = '';
  let loadSequence = 0;
  const nav = [
    { id: 'overview', name: 'Visão geral', icon: 'grid' },
    { id: 'transactions', name: 'Transações', icon: 'arrows' },
    { id: 'accounts', name: 'Minhas contas', icon: 'wallet' },
    { id: 'recurring', name: 'Despesas fixas', icon: 'repeat' },
    { id: 'household', name: 'Nossa casa', icon: 'home' },
    { id: 'reports', name: 'Relatórios', icon: 'chart' },
  ];
  const palette = [
    '#47d7a0',
    '#5e95eb',
    '#b69af5',
    '#e9b76a',
    '#e9838c',
    '#78cbd0',
  ];
  const title = $derived(
    page === 'settings'
      ? 'Configurações'
      : nav.find((n) => n.id === page)?.name || 'Visão geral',
  );
  const filtered = $derived(
    transactions.filter(
      (t) =>
        (!search ||
          `${t.description} ${t.category}`
            .toLowerCase()
            .includes(search.toLowerCase())) &&
        (typeFilter === 'all' || t.type === typeFilter) &&
        (stateFilter === 'all' || t.status === stateFilter) &&
        (categoryFilter === 'all' ||
          String(t.category_id) === categoryFilter) &&
        (accountFilter === 'all' || String(t.account_id) === accountFilter),
    ),
  );
  const maxChart = $derived(
    Math.max(
      1,
      ...(dashboard?.history.flatMap((h) => [
        h.income_cents,
        h.expense_cents,
      ]) || []),
    ),
  );
  const money = (n: number | null | undefined) =>
    hidden
      ? '••••••'
      : n == null
        ? '—'
        : new Intl.NumberFormat('pt-BR', {
            style: 'currency',
            currency: 'BRL',
          }).format(n / 100);
  const shortMonth = (m: string) =>
    new Intl.DateTimeFormat('pt-BR', { month: 'short', timeZone: 'UTC' })
      .format(new Date(`${m}-02T12:00:00Z`))
      .replace('.', '');
  const dateLabel = (d: string) =>
    new Intl.DateTimeFormat('pt-BR', {
      day: '2-digit',
      month: 'short',
      timeZone: 'UTC',
    })
      .format(new Date(`${d}T12:00:00Z`))
      .replace('.', '');
  const monthLabel = (m: string) =>
    new Intl.DateTimeFormat('pt-BR', {
      month: 'long',
      year: 'numeric',
      timeZone: 'UTC',
    }).format(new Date(`${m}-02T12:00:00Z`));
  function cents(s: string) {
    const clean = s.trim().replace(/\s|R\$/g, '');
    if (!/^\d+(?:[,.]\d{1,2})?$/.test(clean))
      throw Error('Informe um valor como 150,90, sem separador de milhar.');
    const [a, b = ''] = clean.replace(',', '.').split('.');
    const v = Number(a) * 100 + Number(b.padEnd(2, '0'));
    if (!Number.isSafeInteger(v) || v > 100000000000)
      throw Error('Valor acima do limite.');
    return v;
  }
  async function api<T>(
    path: string,
    method = 'GET',
    body?: unknown,
    key?: string,
  ): Promise<T> {
    const headers: Record<string, string> = {
      'Content-Type': 'application/json',
    };
    if (user) headers['X-CSRF-Token'] = user.csrf;
    if (key) headers['Idempotency-Key'] = key;
    const res = await fetch('/api/v1' + path, {
      method,
      headers,
      credentials: 'same-origin',
      body: body === undefined ? undefined : JSON.stringify(body),
    });
    const data = await res.json();
    if (!res.ok) {
      if (res.status === 401 && path != '/sessions') {
        user = null;
        dashboard = null;
        transactions = [];
        accounts = [];
        categories = [];
        rules = [];
        loadSequence++;
      }
      throw Error(data.error || 'Não foi possível concluir. Tente novamente.');
    }
    return data as T;
  }
  onMount(async () => {
    try {
      const config = await api<{ demo: boolean }>('/config');
      demo = config.demo;
      try {
        user = await api<User>('/me');
        name = user.name;
        await load();
      } catch (e) {
        if (user) throw e;
      }
    } catch (e) {
      error = (e as Error).message;
    } finally {
      boot = false;
    }
  });
  async function login(isDemo = false) {
    busy = true;
    error = '';
    try {
      user = await api<User>('/sessions', 'POST', {
        email: isDemo ? 'alex@demo.local' : email,
        password: isDemo ? 'demonstracao-local-2026' : password,
      });
      password = '';
      name = user.name;
      await load();
    } catch (e) {
      error = (e as Error).message;
    } finally {
      busy = false;
    }
  }
  async function load() {
    if (!user) return;
    const seq = ++loadSequence;
    loading = true;
    error = '';
    try {
      const q = `?month=${month}&context=${context}`;
      const [d, a, c, t, rs] = await Promise.all([
        api<Dashboard>('/dashboard' + q),
        api<Account[]>('/accounts'),
        api<Category[]>('/categories'),
        api<{ items: Transaction[]; total: number }>(
          '/transactions' + q + `&offset=${offset}`,
        ),
        api<Rule[]>('/recurring-rules'),
      ]);
      if (seq !== loadSequence) return;
      dashboard = d;
      dateToday = d.today;
      accounts = a;
      categories = c;
      transactions = t.items;
      total = t.total;
      rules = rs;
    } catch (e) {
      if (seq === loadSequence) error = (e as Error).message;
    } finally {
      if (seq === loadSequence) loading = false;
    }
  }
  function navigate(id: string) {
    page = id;
    menu = false;
    offset = 0;
    search = '';
    typeFilter = 'all';
    stateFilter = 'all';
    categoryFilter = 'all';
    accountFilter = 'all';
    if (id === 'household') context = 'household';
    else if (id === 'accounts' || id === 'recurring') context = 'personal';
    void load();
  }
  function changeMonth(delta: number) {
    const d = new Date(month + '-15T12:00:00Z');
    d.setUTCMonth(d.getUTCMonth() + delta);
    month = d.toISOString().slice(0, 7);
    offset = 0;
    void load();
  }
  async function openModal(which: string, item: Transaction | null = null) {
    selected = item;
    modal = which;
    modalError = '';
    amount = item ? (item.amount_cents / 100).toFixed(2).replace('.', ',') : '';
    description = item?.description || '';
    kind = 'expense';
    account = String(accounts[0]?.id || '');
    destination = String(accounts[1]?.id || '');
    category = String(categories.find((c) => c.type === 'expense')?.id || '');
    date = dateToday;
    due = dateToday;
    competence = month;
    status = 'paid';
    visibility = 'private';
    day = 10;
    requestKey = crypto.randomUUID();
    await tick();
    dialog.showModal();
  }
  function closeModal() {
    if (busy) return;
    dialog.close();
    modal = '';
    selected = null;
    modalError = '';
  }
  async function save() {
    busy = true;
    modalError = '';
    try {
      if (modal === 'transaction') {
        const transfer = kind === 'transfer';
        await api(
          '/transactions',
          'POST',
          {
            type: kind,
            description,
            amount_cents: cents(amount),
            account_id: Number(account),
            destination_id: transfer ? Number(destination) : null,
            category_id: transfer ? null : Number(category),
            date,
            competence,
            due_date: due,
            status: transfer ? 'paid' : status,
            paid_date: transfer || status === 'paid' ? date : null,
            visibility: transfer ? 'private' : visibility,
          },
          requestKey,
        );
      } else if (modal === 'account') {
        await api('/accounts', 'POST', {
          name: description,
          kind: accountKind,
          opening_cents: amount.startsWith('-')
            ? -cents(amount.slice(1))
            : cents(amount || '0'),
          opening_date: date,
        });
      } else if (modal === 'category') {
        await api('/categories', 'POST', { name: description, type: kind });
      } else if (modal === 'rule') {
        await api('/recurring-rules', 'POST', {
          description,
          amount_cents: cents(amount),
          account_id: Number(account),
          category_id: Number(category),
          day: Number(day),
          starts_on: competence,
        });
      } else if (selected) {
        await api(`/transactions/${selected.id}`, 'PATCH', {
          version: selected.version,
          action: modal,
          amount_cents: modal === 'cancel' ? 0 : cents(amount),
          paid_date: date,
          description,
        });
      }
      notice =
        modal === 'cancel'
          ? 'Lançamento cancelado. Histórico preservado.'
          : 'Salvo com sucesso.';
      busy = false;
      closeModal();
      await load();
    } catch (e) {
      modalError = (e as Error).message;
    } finally {
      busy = false;
    }
  }
  async function generate() {
    busy = true;
    error = '';
    try {
      const x = await api<{ created: number }>(
        '/recurring-rules/generate',
        'POST',
        { month },
      );
      notice = x.created
        ? `${x.created} conta(s) gerada(s) para ${monthLabel(month)}.`
        : 'As contas deste mês já estão geradas.';
      await load();
    } catch (e) {
      error = (e as Error).message;
    } finally {
      busy = false;
    }
  }
  async function saveSettings() {
    busy = true;
    error = '';
    try {
      const result = await api<{ login_required: boolean }>(
        '/settings',
        'PATCH',
        { name, current_password: currentPassword, new_password: newPassword },
      );
      currentPassword = '';
      newPassword = '';
      if (result.login_required) {
        user = null;
        dashboard = null;
        transactions = [];
        accounts = [];
        rules = [];
        categories = [];
        notice = 'Senha alterada. Entre novamente.';
      } else {
        if (user) user.name = name;
        notice = 'Preferências salvas.';
      }
    } catch (e) {
      error = (e as Error).message;
    } finally {
      busy = false;
    }
  }
  async function logout() {
    busy = true;
    try {
      await api('/sessions/current', 'DELETE');
      loadSequence++;
      user = null;
      dashboard = null;
      accounts = [];
      categories = [];
      transactions = [];
      rules = [];
      notice = '';
      email = '';
      page = 'overview';
      context = 'personal';
    } catch (e) {
      error = (e as Error).message;
    } finally {
      busy = false;
    }
  }
  const statusText = (t: Transaction) =>
    t.status === 'cancelled'
      ? 'Cancelado'
      : t.status === 'paid'
        ? t.type === 'income'
          ? 'Recebido'
          : 'Pago'
        : t.due_date < dateToday
          ? 'Vencido'
          : 'Pendente';
</script>

<svelte:head
  ><title>{user ? title + ' · ' : ''}Meu Financeiro</title></svelte:head
>
{#if boot}<main class="login">
    <div class="brand-mark"><Icon name="leaf" size={32} /></div>
    <p>Preparando seu espaço…</p>
  </main>
{:else if !user}
  <main class="login">
    <div class="login-card">
      <a class="brand" href="/"
        ><span class="brand-mark"><Icon name="leaf" size={28} /></span><span
          >meu<span class="brand-light">financeiro</span><small
            >CLAREZA PARA SUA VIDA</small
          ></span
        ></a
      >
      <div class="eyebrow">BEM-VINDO AO SEU ESPAÇO</div>
      <h1>Mais clareza.<br />Mais tranquilidade.</h1>
      <p>Suas finanças pessoais e da casa,<br />organizadas em um só lugar.</p>
      {#if error}<div class="alert error" role="alert">
          {error}
        </div>{/if}{#if notice}<div class="alert success" role="status">
          {notice}
        </div>{/if}
      <form
        onsubmit={(e) => {
          e.preventDefault();
          void login();
        }}
      >
        <label
          >E-mail<input
            type="email"
            bind:value={email}
            autocomplete="username"
            required
            placeholder="voce@exemplo.com"
          /></label
        ><label
          >Senha<input
            type="password"
            bind:value={password}
            autocomplete="current-password"
            required
            placeholder="Sua senha"
          /></label
        ><button class="primary full" disabled={busy}
          >{busy ? 'Entrando…' : 'Entrar na minha conta'}<Icon
            name="arrow"
            size={18}
          /></button
        >
      </form>
      {#if demo}<div class="demo-entry">
          <span>Ambiente local com dados fictícios</span><button
            class="secondary full"
            onclick={() => login(true)}
            disabled={busy}
            >Explorar demonstração<Icon name="arrow" size={17} /></button
          >
        </div>{/if}
      <div class="secure">
        <Icon name="shield" size={15} /> Acesso individual. Dados privados por padrão.
      </div>
    </div>
    <div class="login-art">
      <div class="orbit one"></div>
      <div class="orbit two"></div>
      <div class="art-symbol"><Icon name="leaf" size={100} /></div>
      <p>
        Um passo de cada vez.<br /><strong>Uma vida mais equilibrada.</strong>
      </p>
    </div>
  </main>
{:else}
  <div class="app-shell">
    <aside class:mobile-open={menu}>
      <a
        class="brand"
        href="#overview"
        onclick={(e) => {
          e.preventDefault();
          navigate('overview');
        }}
        ><span class="brand-mark"><Icon name="leaf" size={25} /></span><span
          >meu<span class="brand-light">financeiro</span><small
            >SEU DINHEIRO, COM CLAREZA</small
          ></span
        ></a
      >
      <div class="workspace">
        <span class="workspace-icon"><Icon name="home" size={18} /></span>
        <div>Meu espaço<small>Finanças pessoais e familiares</small></div>
        <span class="online-dot"></span>
      </div>
      <span class="nav-label">PRINCIPAL</span>
      <nav>
        {#each nav as n}<button
            class:active={page === n.id}
            onclick={() => navigate(n.id)}
            ><Icon name={n.icon} /><span>{n.name}</span>{#if n.id === page}<span
                class="active-dot"
              ></span>{/if}</button
          >{/each}
      </nav>
      <div class="sidebar-bottom">
        <div class="privacy-note">
          <Icon name="shield" size={19} /><strong>Seu espaço é privado</strong>
          <p>Você escolhe o que compartilhar com a casa.</p>
        </div>
        <button
          class:active={page === 'settings'}
          class="settings-link"
          onclick={() => navigate('settings')}
          ><Icon name="settings" /><span>Configurações</span></button
        >
        <div class="profile">
          <span class="avatar">{user.name.slice(0, 2).toUpperCase()}</span>
          <div>
            <strong>{user.name}</strong><small
              >{demo ? 'Conta de demonstração' : 'Minha conta'}</small
            >
          </div>
          <button
            class="icon-button"
            title="Sair"
            aria-label="Sair da conta"
            onclick={logout}
            disabled={busy}><Icon name="logout" size={18} /></button
          >
        </div>
      </div>
    </aside>
    <div class="main-wrap">
      <header class="topbar">
        <div class="breadcrumb">
          <button
            class="icon-button mobile-toggle"
            aria-label="Abrir menu"
            onclick={() => (menu = !menu)}><Icon name="menu" /></button
          ><span>Meu espaço</span><Icon name="chevron" size={13} /><strong
            >{title}</strong
          >
        </div>
        <div class="top-actions">
          {#if demo}<span class="demo-badge">DEMONSTRAÇÃO</span>{/if}<button
            class="icon-button"
            onclick={() => (hidden = !hidden)}
            aria-label={hidden ? 'Mostrar valores' : 'Ocultar valores'}
            title={hidden ? 'Mostrar valores' : 'Ocultar valores'}
            ><Icon name={hidden ? 'hidden' : 'eye'} /></button
          ><span class="top-divider"></span><span class="avatar small"
            >{user.name[0].toUpperCase()}</span
          >
        </div>
      </header>
      <main class="content" aria-busy={loading}>
        <div class="page-heading">
          <div>
            <div class="eyebrow">
              {page === 'overview'
                ? 'CADA PASSO CONTA'
                : page === 'household'
                  ? 'JUNTOS, COM TRANSPARÊNCIA'
                  : 'SEU DINHEIRO ORGANIZADO'}
            </div>
            <h1>
              {page === 'overview'
                ? `Olá, ${user.name.split(' ')[0]} 👋`
                : title}
            </h1>
            <p>
              {page === 'overview'
                ? 'Vamos cuidar do que importa. Aqui está seu resumo financeiro.'
                : page === 'transactions'
                  ? 'Tudo o que entra e sai, sem perder nenhum detalhe.'
                  : page === 'accounts'
                    ? 'Cada conta no seu lugar. Seu saldo sempre à vista.'
                    : page === 'recurring'
                      ? 'Organize os compromissos que fazem parte da sua rotina.'
                      : page === 'household'
                        ? 'Acompanhe somente os lançamentos compartilhados com a família.'
                        : page === 'reports'
                          ? 'Entenda seus hábitos e acompanhe sua evolução.'
                          : 'Um espaço que funciona do seu jeito.'}
            </p>
          </div>
          {#if page !== 'settings'}<button
              class="primary"
              onclick={() =>
                openModal(
                  page === 'accounts'
                    ? 'account'
                    : page === 'recurring'
                      ? 'rule'
                      : 'transaction',
                )}
              ><Icon name="plus" size={18} />{page === 'accounts'
                ? 'Nova conta'
                : page === 'recurring'
                  ? 'Nova despesa fixa'
                  : 'Novo lançamento'}</button
            >{/if}
        </div>
        {#if notice}<div class="alert success" role="status">
            <Icon name="check" size={17} /><span>{notice}</span><button
              class="icon-button"
              aria-label="Fechar aviso"
              onclick={() => (notice = '')}
              ><Icon name="close" size={16} /></button
            >
          </div>{/if}
        {#if error}<div class="alert error" role="alert">
            <Icon name="info" size={18} /><span>{error}</span><button
              class="text-button"
              onclick={load}>Tentar novamente</button
            >
          </div>{/if}
        {#if page !== 'settings' && page !== 'accounts'}<div class="toolbar">
            {#if page !== 'recurring'}<div class="segmented">
                <button
                  class:chosen={context === 'personal'}
                  onclick={() => {
                    context = 'personal';
                    if (page === 'household') page = 'overview';
                    offset = 0;
                    void load();
                  }}><Icon name="wallet" size={16} />Minhas finanças</button
                ><button
                  class:chosen={context === 'household'}
                  onclick={() => {
                    context = 'household';
                    offset = 0;
                    void load();
                  }}><Icon name="home" size={16} />Nossa casa</button
                >
              </div>{:else}<span class="muted">Minhas despesas fixas</span>{/if}
            <div class="period">
              <button
                class="icon-button prev"
                aria-label="Mês anterior"
                onclick={() => changeMonth(-1)}
                ><Icon name="chevron" size={16} /></button
              ><label
                ><Icon name="calendar" size={16} /><span
                  >{monthLabel(month)}</span
                ><input
                  aria-label="Selecionar mês"
                  type="month"
                  min="2000-01"
                  max="2100-12"
                  bind:value={month}
                  onchange={() => {
                    offset = 0;
                    void load();
                  }}
                /></label
              ><button
                class="icon-button"
                aria-label="Próximo mês"
                onclick={() => changeMonth(1)}
                ><Icon name="chevron" size={16} /></button
              >
            </div>
          </div>{/if}
        {#if loading && !dashboard}<div class="empty panel">
            Carregando suas finanças…
          </div>{/if}
        {#if dashboard && (page === 'overview' || page === 'reports' || page === 'household')}
          {#if context === 'household'}<div class="context-note">
              <Icon name="shield" size={17} /><span
                >Somente lançamentos com visibilidade “Nossa casa”. Saldos
                pessoais permanecem privados. Rateio e acertos chegam no MVP 2.</span
              >
            </div>{/if}
          <section class="stats">
            <article class="stat balance">
              <div class="stat-label">
                <span
                  >{context === 'household'
                    ? 'Resultado compartilhado'
                    : 'Saldo das minhas contas'}</span
                ><span class="stat-icon"><Icon name="wallet" size={19} /></span>
              </div>
              <strong
                >{money(
                  context === 'household'
                    ? dashboard.totals.result_cents
                    : dashboard.balance_cents,
                )}</strong
              >
              <div class="stat-foot">
                <span class="green-dot"></span>{context === 'household'
                  ? 'Receitas menos despesas realizadas'
                  : 'Saldo atual · todas as minhas contas'}
              </div>
            </article>
            <article class="stat">
              <div class="stat-label">
                <span>Receitas do mês</span><span class="stat-icon positive"
                  ><Icon name="up" size={18} /></span
                >
              </div>
              <strong>{money(dashboard.totals.income_cents)}</strong>
              <div class="stat-foot">
                <span class="positive">↗ Recebidas</span><span
                  >na competência</span
                >
              </div>
            </article>
            <article class="stat">
              <div class="stat-label">
                <span>Despesas do mês</span><span class="stat-icon negative"
                  ><Icon name="out" size={18} /></span
                >
              </div>
              <strong>{money(dashboard.totals.expense_cents)}</strong>
              <div class="stat-foot">
                <span class="negative">↘ Pagas</span><span>na competência</span>
              </div>
            </article>
            <article class="stat">
              <div class="stat-label">
                <span>Resultado do mês</span><span class="stat-icon"
                  ><Icon name="chart" size={18} /></span
                >
              </div>
              <strong
                class:positive={dashboard.totals.result_cents >= 0}
                class:negative={dashboard.totals.result_cents < 0}
                >{money(dashboard.totals.result_cents)}</strong
              >
              <div class="stat-foot">Receitas − despesas realizadas</div>
            </article>
          </section>
          <div class="charts-grid">
            <section class="panel evolution">
              <div class="panel-heading">
                <div>
                  <h2>Seu dinheiro ao longo do tempo</h2>
                  <p>Receitas e despesas realizadas · competência</p>
                </div>
                <span class="subtle-pill">Últimos 6 meses</span>
              </div>
              <div class="chart-legend">
                <span><i class="green"></i>Receitas</span><span
                  ><i class="blue"></i>Despesas</span
                >
              </div>
              <div class="bar-chart">
                <div class="chart-axis">
                  {#each [1, 0.75, 0.5, 0.25, 0] as level}<span
                      >{hidden
                        ? '•••'
                        : money(Math.round(maxChart * level))}</span
                    >{/each}
                </div>
                <div class="chart-plot">
                  <div class="gridlines">
                    <i></i><i></i><i></i><i></i><i></i>
                  </div>
                  {#each dashboard.history as h}<div
                      class="bar-group"
                      class:current={h.month === month}
                    >
                      <div class="bars">
                        <div
                          class="bar income"
                          style:height={`${(h.income_cents / maxChart) * 100}%`}
                          title={`Receitas: ${money(h.income_cents)}`}
                        ></div>
                        <div
                          class="bar expense"
                          style:height={`${(h.expense_cents / maxChart) * 100}%`}
                          title={`Despesas: ${money(h.expense_cents)}`}
                        ></div>
                      </div>
                      <span>{shortMonth(h.month)}</span>
                    </div>{/each}
                </div>
              </div>
              <details class="chart-data">
                <summary>Ver dados do gráfico</summary>
                <table>
                  <thead
                    ><tr><th>Mês</th><th>Receitas</th><th>Despesas</th></tr
                    ></thead
                  ><tbody
                    >{#each dashboard.history as h}<tr
                        ><td
                          >{monthLabel(h.month)}{h.count === 0
                            ? ' · sem dados'
                            : ''}</td
                        ><td>{money(h.income_cents)}</td><td
                          >{money(h.expense_cents)}</td
                        ></tr
                      >{/each}</tbody
                  >
                </table>
              </details>
            </section>
            <section class="panel category-panel">
              <div class="panel-heading">
                <div>
                  <h2>Para onde foi seu dinheiro?</h2>
                  <p>Despesas pagas por categoria</p>
                </div>
                <span class="icon-muted"><Icon name="chart" size={19} /></span>
              </div>
              {#if dashboard.categories.length}<div class="category-total">
                  <span>Total no mês</span><strong
                    >{money(dashboard.totals.expense_cents)}</strong
                  >
                </div>
                <div class="category-stack" aria-hidden="true">
                  {#each dashboard.categories as c, i}<span
                      style:width={`${(c.amount_cents / Math.max(1, dashboard.totals.expense_cents)) * 100}%`}
                      style:background={palette[i % palette.length]}
                    ></span>{/each}
                </div>
                <div class="category-list">
                  {#each dashboard.categories as c, i}<div>
                      <span
                        ><i style:background={palette[i % palette.length]}
                        ></i>{c.name}</span
                      ><strong>{money(c.amount_cents)}</strong><small
                        >{hidden
                          ? '••'
                          : Math.round(
                              (c.amount_cents /
                                Math.max(1, dashboard.totals.expense_cents)) *
                                100,
                            ) + '%'}</small
                      >
                    </div>{/each}
                </div>{:else}<div class="empty">
                  <Icon name="chart" size={32} />
                  <h3>Um novo começo</h3>
                  <p>Suas despesas pagas aparecerão aqui.</p>
                </div>{/if}
            </section>
          </div>
          {#if page === 'reports'}<div class="report-cards">
              <section class="panel">
                <h2>Fluxo líquido de caixa</h2>
                <strong class="large-value"
                  >{money(dashboard.totals.cash_cents)}</strong
                >
                <p>
                  Entradas menos saídas efetivas pela data de pagamento.
                  Transferências internas são excluídas.
                </p>
              </section>
              <section class="panel">
                <h2>Comparação com o mês anterior</h2>
                <strong class="large-value"
                  >{dashboard.previous.expense_cents === 0
                    ? 'Sem base comparável'
                    : hidden
                      ? '•••'
                      : (
                          (dashboard.totals.expense_cents /
                            dashboard.previous.expense_cents -
                            1) *
                          100
                        ).toFixed(1) + '%'}</strong
                >
                <p>Variação das despesas realizadas por competência.</p>
              </section>
            </div>{/if}
          <div class="lower-grid">
            <section class="panel recent">
              <div class="panel-heading">
                <div>
                  <h2>Últimas transações</h2>
                  <p>Os movimentos mais recentes do mês</p>
                </div>
                <button
                  class="text-button"
                  onclick={() => navigate('transactions')}
                  >Ver todas<Icon name="arrow" size={15} /></button
                >
              </div>
              {#if dashboard.recent.length}<div class="table-wrap">
                  <table class="transaction-table">
                    <thead
                      ><tr
                        ><th>Descrição</th><th>Data</th><th>Status</th><th
                          class="right">Valor</th
                        ></tr
                      ></thead
                    ><tbody
                      >{#each dashboard.recent as t}<tr
                          ><td
                            ><div class="transaction-desc">
                              <span
                                class="transaction-icon"
                                class:income={t.type === 'income'}
                                ><Icon
                                  name={t.type === 'income'
                                    ? 'up'
                                    : t.type === 'transfer'
                                      ? 'arrows'
                                      : t.category === 'Moradia'
                                        ? 'home'
                                        : t.category === 'Mercado'
                                          ? 'wallet'
                                          : 'out'}
                                  size={18}
                                /></span
                              >
                              <div>
                                <strong>{t.description}</strong><small
                                  >{t.category} · {t.account}</small
                                >
                              </div>
                            </div></td
                          ><td class="muted">{dateLabel(t.date)}</td><td
                            ><span
                              class="status"
                              class:paid={t.status === 'paid'}
                              class:overdue={t.status === 'pending' &&
                                t.due_date < dateToday}>{statusText(t)}</span
                            ></td
                          ><td
                            class="right amount"
                            class:positive={t.type === 'income'}
                            >{t.type === 'income'
                              ? '+'
                              : t.type === 'expense'
                                ? '−'
                                : ''}
                            {money(t.amount_cents)}</td
                          ></tr
                        >{/each}</tbody
                    >
                  </table>
                </div>{:else}<div class="empty">
                  <Icon name="arrows" size={30} />
                  <h3>Seu mês começa aqui</h3>
                  <p>Registre seu primeiro lançamento.</p>
                  <button
                    class="secondary"
                    onclick={() => openModal('transaction')}
                    >Adicionar lançamento</button
                  >
                </div>{/if}
            </section>
            <section class="panel upcoming">
              <div class="panel-heading">
                <div>
                  <h2>No seu radar</h2>
                  <p>Compromissos até o fim do mês</p>
                </div>
                <span class="icon-muted"
                  ><Icon name="calendar" size={19} /></span
                >
              </div>
              {#each dashboard.upcoming as t}<div class="upcoming-item">
                  <div class="date-tile" class:late={t.due_date < dateToday}>
                    <strong>{t.due_date.slice(8)}</strong><small
                      >{shortMonth(t.due_date.slice(0, 7))}</small
                    >
                  </div>
                  <div>
                    <strong>{t.description}</strong><small
                      >{t.due_date < dateToday ? 'Vencido' : t.category}</small
                    >
                  </div>
                  <span>{money(t.amount_cents)}</span>
                </div>{:else}<div class="empty compact">
                  <span class="done-circle"
                    ><Icon name="check" size={24} /></span
                  >
                  <h3>Tudo em dia por aqui</h3>
                  <p>Nenhum compromisso pendente.</p>
                </div>{/each}{#if dashboard.available_cents !== null}<div
                  class="available"
                >
                  <span
                    >Disponível planejado<small
                      >Saldo atual − despesas em aberto até este mês</small
                    ></span
                  ><strong>{money(dashboard.available_cents)}</strong>
                </div>{/if}
            </section>
          </div>
        {/if}
        {#if page === 'transactions'}<section class="panel transactions-panel">
            <div class="filters">
              <label class="search"
                ><Icon name="search" size={18} /><input
                  aria-label="Buscar transações"
                  placeholder="Buscar descrição ou categoria…"
                  bind:value={search}
                /></label
              ><select aria-label="Tipo" bind:value={typeFilter}
                ><option value="all">Todos os tipos</option><option
                  value="income">Receitas</option
                ><option value="expense">Despesas</option><option
                  value="transfer">Transferências</option
                ></select
              ><select aria-label="Estado" bind:value={stateFilter}
                ><option value="all">Todos os status</option><option
                  value="paid">Pagos / recebidos</option
                ><option value="pending">Pendentes</option><option
                  value="cancelled">Cancelados</option
                ></select
              ><select aria-label="Categoria" bind:value={categoryFilter}
                ><option value="all">Todas as categorias</option
                >{#each categories as c}<option value={String(c.id)}
                    >{c.name}</option
                  >{/each}</select
              >{#if context === 'personal'}<select
                  aria-label="Conta"
                  bind:value={accountFilter}
                  ><option value="all">Todas as contas</option
                  >{#each accounts as a}<option value={String(a.id)}
                      >{a.name}</option
                    >{/each}</select
                >{/if}
            </div>
            <div class="table-wrap">
              <table class="transaction-table">
                <thead
                  ><tr
                    ><th>Descrição / categoria</th><th>Competência / data</th
                    ><th>Status</th><th class="right">Valor</th><th
                      class="right">Ações</th
                    ></tr
                  ></thead
                ><tbody
                  >{#each filtered as t}<tr
                      ><td
                        ><div class="transaction-desc">
                          <span
                            class="transaction-icon"
                            class:income={t.type === 'income'}
                            ><Icon
                              name={t.type === 'transfer'
                                ? 'arrows'
                                : t.type === 'income'
                                  ? 'up'
                                  : 'out'}
                              size={18}
                            /></span
                          >
                          <div>
                            <strong>{t.description}</strong><small
                              >{t.category} · {t.account}{t.visibility ===
                              'household'
                                ? ' · Casa'
                                : ''}</small
                            >
                          </div>
                        </div></td
                      ><td class="muted"
                        >{dateLabel(t.date)}<small
                          >Vence {dateLabel(t.due_date)}</small
                        ></td
                      ><td
                        ><span
                          class="status"
                          class:paid={t.status === 'paid'}
                          class:overdue={t.status === 'pending' &&
                            t.due_date < dateToday}>{statusText(t)}</span
                        ></td
                      ><td
                        class="right amount"
                        class:positive={t.type === 'income'}
                        >{t.type === 'income'
                          ? '+'
                          : t.type === 'expense'
                            ? '−'
                            : ''}
                        {money(
                          t.amount_cents,
                        )}{#if t.expected_cents !== t.amount_cents}<small
                            >Previsto: {money(t.expected_cents)}</small
                          >{/if}</td
                      ><td
                        ><div class="row-actions">
                          {#if t.owner_id === user.id && t.status !== 'cancelled'}{#if t.status === 'pending'}<button
                                class="icon-button"
                                aria-label={`Pagar ${t.description}`}
                                title="Confirmar pagamento"
                                onclick={() => openModal('pay', t)}
                                ><Icon name="check" size={17} /></button
                              ><button
                                class="icon-button"
                                aria-label={`Editar ${t.description}`}
                                title="Editar valor e descrição"
                                onclick={() => openModal('edit', t)}
                                ><Icon name="edit" size={16} /></button
                              >{/if}<button
                              class="icon-button"
                              aria-label={`Cancelar ${t.description}`}
                              title="Cancelar lançamento"
                              onclick={() => openModal('cancel', t)}
                              ><Icon name="trash" size={16} /></button
                            >{/if}
                        </div></td
                      ></tr
                    >{:else}<tr
                      ><td colspan="5"
                        ><div class="empty">
                          <Icon name="search" size={28} />
                          <h3>Nenhum lançamento encontrado</h3>
                          <p>
                            Experimente outro filtro ou adicione um lançamento.
                          </p>
                        </div></td
                      ></tr
                    >{/each}</tbody
                >
              </table>
            </div>
            <div class="table-footer">
              <span
                >{filtered.length} nesta página · {total} lançamento(s) no mês. Filtros
                aplicados à página.</span
              >
              <div>
                <button
                  class="secondary small-button"
                  disabled={offset === 0 || loading}
                  onclick={() => {
                    offset = Math.max(0, offset - 100);
                    void load();
                  }}>Anterior</button
                ><button
                  class="secondary small-button"
                  disabled={offset + 100 >= total || loading}
                  onclick={() => {
                    offset += 100;
                    void load();
                  }}>Próxima</button
                >
              </div>
            </div>
          </section>{/if}
        {#if page === 'accounts'}<div class="account-grid">
            {#each accounts as a, i}<article class="panel account-card">
                <div class="account-top">
                  <span
                    class="bank-icon"
                    style:background={`${palette[i % palette.length]}18`}
                    style:color={palette[i % palette.length]}
                    ><Icon name="bank" size={26} /></span
                  ><span class="subtle-pill"
                    >{a.kind === 'savings'
                      ? 'Reserva'
                      : a.kind === 'cash'
                        ? 'Dinheiro'
                        : 'Conta corrente'}</span
                  >
                </div>
                <h2>{a.name}</h2>
                <p>Saldo atual</p>
                <strong class="large-value">{money(a.balance_cents)}</strong>
                <div class="account-bottom">
                  <Icon name="shield" size={14} /> Conta pessoal · privada
                </div>
                <small
                  >Saldo inicial: {money(a.opening_cents)} em {dateLabel(
                    a.opening_date,
                  )}</small
                >
              </article>{:else}<section class="panel empty">
                <Icon name="wallet" size={36} />
                <h3>Adicione sua primeira conta</h3>
                <p>
                  Informe o saldo inicial para começar com os números certos.
                </p>
                <button class="primary" onclick={() => openModal('account')}
                  >Criar conta</button
                >
              </section>{/each}
          </div>
          <div class="context-note">
            <Icon name="info" size={18} /><span
              >O saldo inicial não entra como receita. Transferências entre suas
              contas não alteram o resultado do mês. Cartões fazem parte da
              v1.1.</span
            >
          </div>{/if}
        {#if page === 'recurring'}<section class="panel">
            <div class="panel-heading">
              <div>
                <h2>Sua rotina financeira</h2>
                <p>Uma ocorrência por competência, mesmo ao gerar novamente.</p>
              </div>
              <button
                class="secondary"
                disabled={busy || !rules.length}
                onclick={generate}
                ><Icon name="repeat" size={16} />{busy
                  ? 'Gerando…'
                  : 'Gerar contas do mês'}</button
              >
            </div>
            {#each rules as r}<div class="rule-row">
                <span class="date-tile"
                  ><strong>{r.day}</strong><small>todo mês</small></span
                >
                <div>
                  <strong>{r.description}</strong><small
                    >Desde {monthLabel(r.starts_on)} · {accounts.find(
                      (a) => a.id === r.account_id,
                    )?.name}</small
                  >
                </div>
                <span>{money(r.amount_cents)}<small>valor previsto</small></span
                >
              </div>{:else}<div class="empty">
                <Icon name="repeat" size={32} />
                <h3>Menos coisas para lembrar</h3>
                <p>
                  Cadastre aluguel, energia, internet e outras despesas mensais.
                </p>
                <button class="secondary" onclick={() => openModal('rule')}
                  >Cadastrar despesa fixa</button
                >
              </div>{/each}
          </section>
          <div class="context-note">
            <Icon name="info" size={18} /><span
              >Em meses curtos, vencimentos nos dias 29–31 usam o último dia
              válido. Confirme o valor real ao pagar a ocorrência em Transações.</span
            >
          </div>{/if}
        {#if page === 'settings'}<div class="settings-grid">
            <section class="panel">
              <div class="panel-heading">
                <div>
                  <h2>Meu perfil</h2>
                  <p>Informações da sua conta individual</p>
                </div>
                <Icon name="settings" />
              </div>
              <form
                onsubmit={(e) => {
                  e.preventDefault();
                  void saveSettings();
                }}
              >
                <label
                  >Como você quer ser chamado?<input
                    bind:value={name}
                    required
                    maxlength="80"
                  /></label
                ><label>E-mail<input value={user.email} disabled /></label>
                <div class="form-divider"></div>
                <h3>Alterar senha</h3>
                <p class="muted">
                  Deixe em branco para manter sua senha atual.
                </p>
                <label
                  >Senha atual<input
                    type="password"
                    bind:value={currentPassword}
                    autocomplete="current-password"
                  /></label
                ><label
                  >Nova senha<input
                    type="password"
                    bind:value={newPassword}
                    autocomplete="new-password"
                    minlength="12"
                    placeholder="Pelo menos 12 caracteres"
                  /></label
                ><button class="primary" disabled={busy}
                  >Salvar alterações</button
                >
              </form>
            </section>
            <div>
              <section class="panel">
                <div class="panel-heading">
                  <div>
                    <h2>Suas categorias</h2>
                    <p>Organize os lançamentos do seu jeito</p>
                  </div>
                  <button
                    class="icon-button"
                    aria-label="Adicionar categoria"
                    onclick={() => openModal('category')}
                    ><Icon name="plus" /></button
                  >
                </div>
                <div class="category-chips">
                  {#each categories as c}<span
                      ><i class:income={c.type === 'income'}></i>{c.name}</span
                    >{/each}
                </div>
              </section>
              <section class="panel preferences">
                <h2>Preferências da família</h2>
                <div>
                  <span>Moeda</span><strong>Real brasileiro · BRL</strong>
                </div>
                <div>
                  <span>Fuso financeiro</span><strong>America/Fortaleza</strong>
                </div>
                <div>
                  <span>Visibilidade inicial</span><strong>Privada</strong>
                </div>
                <p class="muted">
                  Versão inicial · Controle manual. O roadmap completo está
                  versionado no projeto.
                </p>
              </section>
            </div>
          </div>{/if}
        <footer>
          <span
            ><span class="green-dot"></span>Um pouco mais de clareza, todos os
            dias.</span
          ><span>Meu Financeiro <span class="footer-dot">·</span> v0.1</span>
        </footer>
      </main>
    </div>
  </div>
{/if}

<dialog
  bind:this={dialog}
  oncancel={(e) => {
    e.preventDefault();
    closeModal();
  }}
>
  {#if modal}<div class="modal-heading">
      <div>
        <span class="eyebrow">SEU CONTROLE FINANCEIRO</span>
        <h2>
          {(
            {
              transaction: 'Novo lançamento',
              account: 'Nova conta',
              category: 'Nova categoria',
              rule: 'Nova despesa fixa',
              pay: 'Confirmar pagamento',
              edit: 'Editar lançamento',
              cancel: 'Cancelar lançamento',
            } as Record<string, string>
          )[modal]}
        </h2>
      </div>
      <button
        class="icon-button"
        aria-label="Fechar formulário"
        onclick={closeModal}><Icon name="close" /></button
      >
    </div>
    <form
      onsubmit={(e) => {
        e.preventDefault();
        void save();
      }}
    >
      {#if modalError}<div class="alert error" role="alert">
          {modalError}
        </div>{/if}
      {#if modal === 'cancel'}<p>
          Cancelar <strong>{selected?.description}</strong> ({money(
            selected?.amount_cents,
          )})? O efeito no saldo será revertido e o histórico permanecerá
          registrado.
        </p>
      {:else}
        {#if modal === 'transaction' || modal === 'category'}<label
            >Tipo<select
              bind:value={kind}
              onchange={() => {
                category = String(
                  categories.find((c) => c.type === kind)?.id || '',
                );
                if (kind === 'transfer') {
                  status = 'paid';
                  visibility = 'private';
                }
              }}
              ><option value="expense">Despesa</option><option value="income"
                >Receita</option
              >{#if modal === 'transaction'}<option value="transfer"
                  >Transferência entre minhas contas</option
                >{/if}</select
            ></label
          >{/if}
        {#if modal !== 'category'}<label class="amount-input"
            >{modal === 'account'
              ? 'Saldo inicial (R$)'
              : modal === 'rule'
                ? 'Valor previsto (R$)'
                : 'Valor (R$)'}<input
              bind:value={amount}
              inputmode="decimal"
              placeholder="0,00"
              required={modal !== 'account'}
              maxlength="16"
            /></label
          >{/if}
        {#if modal !== 'pay'}<label
            >{modal === 'account' || modal === 'category'
              ? 'Nome'
              : 'Descrição'}<input
              bind:value={description}
              required
              maxlength={modal === 'account'
                ? 80
                : modal === 'category'
                  ? 60
                  : 180}
              placeholder={modal === 'account'
                ? 'Ex.: Minha conta corrente'
                : modal === 'category'
                  ? 'Ex.: Educação'
                  : 'Ex.: Supermercado'}
            /></label
          >{:else}<p class="muted">
            {selected?.description} · Previsto: {money(
              selected?.expected_cents,
            )}. Ajuste o valor efetivamente pago.
          </p>{/if}
        {#if modal === 'account'}<label
            >Tipo de conta<select bind:value={accountKind}
              ><option value="checking">Conta corrente</option><option
                value="savings">Reserva / poupança</option
              ><option value="cash">Dinheiro em espécie</option></select
            ></label
          >{/if}
        {#if modal === 'transaction' || modal === 'rule'}<div class="form-row">
            <label
              >Conta<select bind:value={account} required
                ><option value="" disabled>Selecione uma conta</option
                >{#each accounts as a}<option value={String(a.id)}
                    >{a.name}</option
                  >{/each}</select
              ></label
            >{#if kind !== 'transfer'}<label
                >Categoria<select bind:value={category} required
                  ><option value="" disabled>Selecione</option
                  >{#each categories.filter((c) => c.type === (modal === 'rule' ? 'expense' : kind)) as c}<option
                      value={String(c.id)}>{c.name}</option
                    >{/each}</select
                ></label
              >{:else}<label
                >Conta de destino<select bind:value={destination} required
                  ><option value="" disabled>Selecione</option
                  >{#each accounts.filter((a) => String(a.id) !== account) as a}<option
                      value={String(a.id)}>{a.name}</option
                    >{/each}</select
                ></label
              >{/if}
          </div>
          {#if !accounts.length}<p class="negative">
              Crie uma conta antes de registrar lançamentos.
            </p>{/if}{/if}
        {#if modal === 'transaction' || modal === 'account' || modal === 'pay'}<div
            class="form-row"
          >
            <label
              >{modal === 'pay'
                ? 'Data do pagamento'
                : modal === 'account'
                  ? 'Saldo inicial em'
                  : 'Data do evento / pagamento'}<input
                type="date"
                bind:value={date}
                required
                min="2000-01-01"
                max={modal === 'account' || modal === 'pay' || status === 'paid'
                  ? dateToday
                  : '2100-12-31'}
              /></label
            >{#if modal === 'transaction'}<label
                >Competência<input
                  type="month"
                  bind:value={competence}
                  min="2000-01"
                  max="2100-12"
                  required
                /></label
              >{/if}
          </div>{/if}
        {#if modal === 'transaction' && kind !== 'transfer'}<div
            class="form-row"
          >
            <label
              >Status<select bind:value={status}
                ><option value="paid"
                  >{kind === 'income' ? 'Recebido' : 'Pago'}</option
                ><option value="pending">Pendente / previsto</option></select
              ></label
            ><label
              >Vencimento<input
                type="date"
                bind:value={due}
                required
                min="2000-01-01"
                max="2100-12-31"
              /></label
            >
          </div>
          <label
            >Quem pode ver?<select bind:value={visibility}
              ><option value="private">Somente eu · privado</option><option
                value="household">Nossa casa · compartilhar lançamento</option
              ></select
            ></label
          >
          <p class="form-hint">
            <Icon name="shield" size={14} /> Compartilhar permite visualizar este
            lançamento. Não aplica rateio.
          </p>{/if}
        {#if modal === 'rule'}<div class="form-row">
            <label
              >Dia do vencimento<input
                type="number"
                bind:value={day}
                min="1"
                max="31"
                required
              /></label
            ><label
              >A partir de<input
                type="month"
                bind:value={competence}
                required
                min="2000-01"
                max="2100-12"
              /></label
            >
          </div>
          <p class="form-hint">
            Depois de salvar, use “Gerar contas do mês”. A recorrência não
            registra pagamento automático.
          </p>{/if}
      {/if}
      <div class="modal-actions">
        <button
          class="secondary"
          type="button"
          onclick={closeModal}
          disabled={busy}>Voltar</button
        ><button
          class="primary"
          class:danger={modal === 'cancel'}
          disabled={busy ||
            ((modal === 'transaction' || modal === 'rule') && !accounts.length)}
          >{busy
            ? 'Salvando…'
            : modal === 'cancel'
              ? 'Confirmar cancelamento'
              : modal === 'pay'
                ? 'Confirmar pagamento'
                : 'Salvar'}<Icon name="check" size={17} /></button
        >
      </div>
    </form>{/if}
</dialog>
