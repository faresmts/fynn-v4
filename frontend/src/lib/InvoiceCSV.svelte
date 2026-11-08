<script lang="ts">
  type Review = {
    token: string; total_cents: number; new_cents: number; new_count: number; duplicate_count: number;
    lines: { date: string; description: string; amount_cents: number; duplicate: boolean }[];
  };
  let { card, month, csrf, money, onImported }: {
    card: number; month: string;
    csrf: string; money: (amount: number) => string; onImported: () => Promise<void>;
  } = $props();
  let file = $state<File | null>(null);
  let review = $state<Review | null>(null), busy = $state(false);
  let error = $state(''), status = $state('');
  let input: HTMLInputElement;

  function resetReview() {
    review = null;
    error = '';
    status = '';
  }
  async function readInvoice(confirm = false) {
    if (!file || !card || busy || (confirm && !review)) return;
    busy = true;
    error = '';
    status = '';
    try {
      const form = new FormData();
      form.append('file', file);
      form.append('card_id', String(card));
      form.append('month', month);
      if (confirm && review) form.append('token', review.token);
      const response = await fetch('/api/v1/imports/csv' + (confirm ? '/confirm' : ''), {
        method: 'POST', headers: { 'X-CSRF-Token': csrf }, body: form,
      });
      const result = await response.json();
      if (!response.ok) {
        if (response.status === 409) review = null;
        throw Error(result.error || 'Falha ao processar a fatura.');
      }
      if (confirm) {
        status = result.inserted + ' gasto(s) importado(s). Já existentes: ' + result.skipped + '.';
        review = null;
        file = null;
        input.value = '';
        await onImported();
      } else {
        review = result;
      }
    } catch (e) {
      error = (e as Error).message;
    } finally {
      busy = false;
    }
  }
</script>

<section class="panel card-registration">
  <div class="panel-heading"><div><h2>Importar fatura CSV</h2><p>Revise os gastos antes de incluí-los nesta fatura.</p></div></div>
  <form onsubmit={(e) => { e.preventDefault(); void readInvoice(); }}>
    <label>Arquivo CSV<input type="file" accept=".csv,text/csv" bind:this={input} disabled={busy}
      onchange={(e) => { resetReview(); file = e.currentTarget.files?.[0] ?? null; }} /></label>

    <button class="secondary" disabled={busy || !file || !card}>{busy && !review ? 'Lendo…' : 'Revisar fatura'}</button>
  </form>
  {#if error}<p role="alert" class="form-error">{error}</p>{/if}
  {#if status}<p role="status" class="form-success">{status}</p>{/if}
  {#if review}
    <h3>Gastos da fatura</h3>
    <p>{review.lines.length} gasto(s) · Total da fatura <strong>{money(review.total_cents)}</strong></p>
    <div class="review-table"><table>
      <thead><tr><th>Data</th><th>Estabelecimento</th><th>Valor</th><th>Importação</th></tr></thead>
      <tbody>{#each review.lines as line}<tr>
        <td>{line.date.split('-').reverse().join('/')}</td><td>{line.description}</td><td>{money(line.amount_cents)}</td>
        <td>{line.duplicate ? 'Já importado' : 'Novo'}</td>
      </tr>{/each}</tbody>
    </table></div>
    <p>{review.new_count} novo(s) · {money(review.new_cents)} a importar · {review.duplicate_count} já existente(s)</p>
    <button class="primary" disabled={busy || !review.new_count} onclick={() => void readInvoice(true)}>
      {busy ? 'Processando…' : 'Confirmar importação'}
    </button>
    <button class="secondary" disabled={busy} onclick={resetReview}>Cancelar</button>
  {/if}
</section>

<style>
  form { display: grid; grid-template-columns: minmax(0, 1fr) auto; gap: 12px; align-items: end; }
  @media (max-width: 600px) { form { grid-template-columns: minmax(0, 1fr); } }
  .review-table { overflow: auto; max-height: 26rem; }
  h3 { margin-top: 1.5rem; }
  th, td { text-align: left; padding: .6rem .8rem; }
</style>
