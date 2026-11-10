<script lang="ts">
  import { onMount } from 'svelte';
  type Payment = {id:number; paid_on:string; amount_cents:number};
  type Contract = {id:number; name:string; total_cents:number; paid_cents:number; installments:number; installment_cents:number; payments:Payment[]};
  const today = () => new Intl.DateTimeFormat('en-CA',{timeZone:'America/Fortaleza',year:'numeric',month:'2-digit',day:'2-digit'}).format(new Date());
  let dates=$state<Record<number,string>>({});
  let {api,money,cents}: {
    api:<T>(path:string,method?:string,body?:unknown)=>Promise<T>;
    money:(n:number)=>string; cents:(s:string)=>number;
  }=$props();
  let items=$state<Contract[]>([]), busy=$state(false), loaded=$state(false);
  let name=$state(''), total=$state(''), paid=$state('0'), amount=$state(''), count=$state<number>();
  let error=$state(''), notice=$state('');
  async function load(){
    try {items=await api<Contract[]>('/financings');loaded=true;}
    catch(e){error=(e as Error).message;}
  }
  onMount(()=>{void load();});
  async function create(){
    if(busy)return;
    busy=true;error='';notice='';
    try{
      const contract=await api<Contract>('/financings','POST',{name,total_cents:cents(total),paid_cents:cents(paid),installments:count,installment_cents:cents(amount)});
      items=[contract,...items];name='';total='';paid='0';amount='';count=undefined;
      notice='Financiamento cadastrado.';
    }catch(e){error=(e as Error).message;}finally{busy=false;}
  }
  async function pay(contract:Contract){
    if(busy)return;
    busy=true;error='';notice='';
    try{
      const result=await api<{paid_cents:number}>('/financings/'+contract.id+'/pay','POST',{paid_cents:contract.paid_cents,paid_on:dates[contract.id] ?? today()});
      notice='Pagamento de '+money(result.paid_cents-contract.paid_cents)+' registrado em '+contract.name+'.';
      delete dates[contract.id];
      await load();
    }catch(e){error=(e as Error).message;await load();}finally{busy=false;}
  }
  async function remove(contract:Contract,payment:Payment){
    if(busy || !window.confirm('Excluir o pagamento de '+money(payment.amount_cents)+' de '+payment.paid_on.split('-').reverse().join('/')+'? Esse valor voltará ao saldo a pagar.'))return;
    busy=true;error='';notice='';
    try{
      await api('/financings/'+contract.id+'/payments/'+payment.id,'DELETE');
      notice='Pagamento excluído.';
    }catch(e){error=(e as Error).message;}finally{await load();busy=false;}
  }
</script>
{#if error}<p class="alert error" role="alert">{error}</p>{/if}
{#if notice}<p class="alert success" role="status">{notice}</p>{/if}
<section class="panel">
  <h2>Cadastrar financiamento</h2>
  <form onsubmit={(e)=>{e.preventDefault();void create();}}>
    <label>Nome do contrato<input bind:value={name} maxlength="100" required placeholder="Ex.: Carro" /></label>
    <label>Valor total do contrato (R$)<input bind:value={total} required inputmode="decimal" placeholder="30000,00" /></label>
    <label>Valor já pago (R$)<input bind:value={paid} required inputmode="decimal" /></label>
    <label>Quantidade de parcelas<input type="number" min="1" step="1" bind:value={count} required /></label>
    <label>Parcela atual (R$)<input bind:value={amount} required inputmode="decimal" placeholder="625,00" /></label>
    <button class="primary" disabled={busy || !loaded}>Cadastrar</button>
  </form>
</section>
<section class="panel contracts">
  <h2>Meus financiamentos</h2>
  {#each items as contract (contract.id)}
    {@const remaining=contract.total_cents-contract.paid_cents}
    {@const opening=contract.paid_cents-contract.payments.reduce((sum,p)=>sum+p.amount_cents,0)}
    <article>
      <h3>{contract.name}</h3>
      <p>{contract.installments} parcelas · Parcela atual de {money(contract.installment_cents)}</p>
      <dl>
        <div><dt>Total do contrato</dt><dd>{money(contract.total_cents)}</dd></div>
        <div><dt>Valor pago</dt><dd>{money(contract.paid_cents)}</dd></div>
        <div><dt>Falta pagar</dt><dd>{money(remaining)}</dd></div>
        <div><dt>Parcelas pagas no histórico</dt><dd>{contract.payments.length}</dd></div>
      </dl>
      {#if remaining>0}<form onsubmit={(e)=>{e.preventDefault();void pay(contract);}}>
        <label>Data do pagamento<input type="date" value={dates[contract.id] ?? today()} max={today()} required oninput={(e)=>dates[contract.id]=e.currentTarget.value} /></label>
        <button class="primary" disabled={busy}>Pagar parcela · {money(Math.min(remaining,contract.installment_cents))}</button>
      </form>
      {:else}<strong>Quitado</strong>{/if}
      <details><summary>Histórico de pagamentos ({contract.payments.length})</summary>
        {#if opening>0}<p>Valor pago anterior ao histórico: {money(opening)}.</p>{/if}
        {#each contract.payments as payment (payment.id)}
          <div class="payment"><time datetime={payment.paid_on}>{payment.paid_on.split('-').reverse().join('/')}</time>
            <strong>{money(payment.amount_cents)}</strong><button class="secondary" disabled={busy} onclick={()=>void remove(contract,payment)}>Excluir pagamento</button>
          </div>
        {:else}<p>Nenhuma parcela registrada no histórico.</p>{/each}
      </details>
    </article>
  {:else}<p>{loaded ? 'Nenhum financiamento cadastrado.' : 'Carregando financiamentos…'}</p>{/each}
</section>
<style>
  form,dl {display:grid;grid-template-columns:repeat(auto-fit,minmax(min(100%,160px),1fr));gap:14px;align-items:end;margin:16px 0 0;}
  .contracts {margin-top:18px;}
  details {margin-top:16px;} summary {cursor:pointer;} .payment {display:flex;align-items:center;gap:12px;flex-wrap:wrap;margin-top:10px;}
  article {padding-block:18px;border-top:1px solid var(--line);margin-top:14px;}
  dl {margin-bottom:16px;} dd {margin:4px 0 0;font-weight:600;} dt,p {color:var(--muted);}
</style>
