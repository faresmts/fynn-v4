<script lang="ts">
  import { onMount } from 'svelte';
  import InvoiceCSV from './InvoiceCSV.svelte';
  import type { Account } from './types';
  type Statement = {
    month: string; status: string; total_cents: number; due_cents: number;
    lines: {id:number; shared:boolean; date:string; description:string; amount_cents:number}[];
  };
  let {card, month, accounts, csrf, api, money, onChanged}: {
    card:number; month:string; accounts: Account[]; csrf:string;
    api:<T>(path:string,method?:string,body?:unknown)=>Promise<T>;
    money:(amount:number)=>string; onChanged:()=>Promise<void>;
  } = $props();
  let statement = $state<Statement | null>(null), selectedAccount = $state('');
  let error = $state(''), busy = $state(false), feedback = $state('');
  let sharingLine = $state<number | null>(null);
  async function refresh() {
    try {
      statement = await api<Statement>('/cards/' + card + '/invoice?month=' + month);
    } catch(e) { statement = null; error = (e as Error).message; }
  }
  onMount(() => { void refresh(); });
  async function share(lineId: number) {
    if (busy) return;
    busy = true;
    sharingLine = lineId;
    error = '';
    feedback = '';
    try {
      await api('/invoice-lines/' + lineId + '/share', 'POST');
      await refresh();
      feedback = 'Compra compartilhada. Você pode alterar o rateio em Nossa casa.';
      await onChanged();
    } catch (e) { error = (e as Error).message; }
    finally { sharingLine = null; busy = false; }
  }
  async function pay() {
    if (busy || !statement?.due_cents || !selectedAccount) return;
    busy = true;
    error = '';
    try {
      await api('/cards/' + card + '/invoice/pay', 'POST', {
        month, account_id:Number(selectedAccount), amount_cents:statement.due_cents
      });
      feedback = 'Fatura paga. A transferência foi registrada em Transações.';
      await onChanged();
    } catch(e) { error = (e as Error).message; }
    finally { await refresh(); busy = false; }
  }
</script>

<section class="panel">
  {#if error}<p class="form-error" role="alert">{error}</p>{/if}
  {#if feedback}<p class="form-success" role="status">{feedback}</p>{/if}
  {#if statement}
    <div class="panel-heading"><div>
      <h2>Fatura {month.slice(5)}/{month.slice(0,4)}</h2>
      <p>{statement.lines.length} gasto(s) · {statement.status === 'paid' ? 'Paga' : 'Em aberto'}</p>
    </div><strong>{money(statement.total_cents)}</strong></div>
    {#if statement.lines.length}
      <div class="statement-table"><table>
        <thead><tr><th>Data da compra</th><th>Estabelecimento</th><th>Valor</th><th>Nossa casa</th></tr></thead>
        <tbody>{#each statement.lines as line}<tr><td>{line.date.split('-').reverse().join('/')}</td>
          <td>{line.description}</td><td>{money(line.amount_cents)}</td><td>
            {#if line.shared}Compartilhado{:else}
              <button class="secondary small-button" disabled={busy} onclick={() => void share(line.id)}>
                {sharingLine === line.id ? 'Compartilhando…' : 'Compartilhar'}
              </button>
            {/if}
          </td></tr>{/each}</tbody>
      </table></div>
      {#if statement.due_cents > 0}
        <form onsubmit={(e) => {e.preventDefault(); void pay();}}>
          <label>Conta de saída<select bind:value={selectedAccount} required disabled={busy}>
            <option value="">Escolha a conta</option>
            {#each accounts as a}<option value={String(a.id)}>{a.name}</option>{/each}
          </select></label>
          <button class="primary" disabled={busy || !selectedAccount}>{busy && sharingLine === null ? 'Pagando…' : 'Pagar fatura'} · {money(statement.due_cents)}</button>
        </form>
      {/if}
    {:else}<p>Nenhum gasto nesta fatura.</p>{/if}
  {:else if !error}<p>Carregando fatura…</p>{/if}
</section>
{#if statement?.status === 'open'}
  <InvoiceCSV {card} {month} {csrf} {money} onImported={refresh} />
{/if}
<style>
  .statement-table { max-height: 28rem; overflow: auto; }
  form { margin-top: 1rem; }
</style>
