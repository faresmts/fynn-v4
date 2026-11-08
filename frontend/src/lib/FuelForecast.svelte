<script lang="ts">
  import {onMount} from 'svelte';
  type Route = {name:string; distance:number | undefined; trips:number | undefined; round_trip:boolean};
  type Inputs = {efficiency:number; price:number; routes:Route[]};
  let {month, api, money}: {month:string; api:<T>(path:string,method?:string,body?:unknown)=>Promise<T>; money:(cents:number)=>string} = $props();
  const blankRoute = ():Route => ({name:'',distance:undefined,trips:undefined,round_trip:false});
  const number = new Intl.NumberFormat('pt-BR',{maximumFractionDigits:2});
  let efficiency = $state<number>(), price = $state<number>(), routes = $state<Route[]>([blankRoute()]);
  let ready = $state(false), saving = $state(false), message = $state(''), error = $state('');
  const complete = $derived(routes.every(r => r.distance && r.distance > 0 && r.trips && r.trips > 0 && Number.isInteger(r.trips)));
  const distance = $derived(routes.reduce((sum,r) => sum + Math.round((r.distance ?? 0)*100) * (r.trips ?? 0) * (r.round_trip ? 2 : 1),0));
  const efficiencyScaled = $derived(Math.round((efficiency ?? 0)*100));
  const litres = $derived(efficiencyScaled > 0 ? distance / efficiencyScaled : 0);
  const cost = $derived(efficiencyScaled > 0 ? Math.round(distance * Math.round((price ?? 0)*100) / efficiencyScaled) : 0);
  async function load(){
    error='';
    try {
      const saved = await api<Inputs|null>('/fuel/forecast/'+month);
      if(saved){ efficiency=saved.efficiency; price=saved.price; routes=saved.routes; }
      ready=true;
    }catch(e){error=(e as Error).message;}
  }
  onMount(()=>{void load();});
  async function save(){
    saving=true; error=''; message='';
    try{
      await api('/fuel/forecast/'+month,'PUT',{efficiency,price,routes});
      message='Previsão salva.';
    }catch(e){error=(e as Error).message;}
    finally{saving=false;}
  }
</script>
<section class="panel">
  {#if ready}
    <form onsubmit={(e)=>{e.preventDefault();void save();}} oninput={()=>{message='';error='';}}>
      <fieldset disabled={saving}>
        <legend>Previsão mensal</legend>
        <div class="parameters">
          <label>Consumo do veículo (km/L)<input type="number" min=".01" max="1000" step=".01" required bind:value={efficiency} placeholder="10" /></label>
          <label>Preço por litro (R$)<input type="number" min=".01" max="1000" step=".01" required bind:value={price} placeholder="6,00" /></label>
        </div>
        <h3>Trajetos</h3>
        {#each routes as route,i}
          <div class="route">
            <label>Nome do trajeto<input bind:value={route.name} maxlength="100" required placeholder="Casa → trabalho" /></label>
            <label>Distância de ida (km)<input type="number" min=".01" max="100000" step=".01" required bind:value={route.distance} /></label>
            <label>Vezes no mês<input type="number" min="1" max="1000" step="1" required bind:value={route.trips} /></label>
            <label class="round-trip"><input type="checkbox" bind:checked={route.round_trip} />Incluir volta</label>
            {#if routes.length>1}<button type="button" class="secondary" onclick={()=>{routes.splice(i,1);message='';}}>Remover</button>{/if}
          </div>
        {/each}
        <button type="button" class="secondary" disabled={routes.length>=100} onclick={()=>{routes.push(blankRoute());message='';}}>Adicionar trajeto</button>
        <dl aria-live="polite">
          <div><dt>Distância mensal</dt><dd>{complete ? number.format(distance/100)+' km' : '—'}</dd></div>
          <div><dt>Combustível estimado</dt><dd>{complete && efficiencyScaled>0 ? number.format(litres)+' L' : '—'}</dd></div>
          <div><dt>Custo previsto</dt><dd>{complete && efficiencyScaled>0 && price && price>0 ? money(cost) : '—'}</dd></div>
        </dl>
        <button class="primary">{saving ? 'Salvando…' : 'Salvar previsão'}</button>
      </fieldset>
    </form>
  {:else if !error}<p>Carregando previsão…</p>{/if}
  {#if error}<p role="alert" class="form-error">{error}</p>{#if !ready}<button class="secondary" onclick={load}>Tentar novamente</button>{/if}{/if}
  {#if message}<p role="status" class="form-success">{message}</p>{/if}
</section>
<style>
  fieldset { border:0; padding:0; margin:0; min-width:0; }
  legend { font-size:1.15rem; font-weight:600; padding:0; }
  .parameters,.route,dl { display:flex; gap:16px; flex-wrap:wrap; align-items:end; margin:18px 0; }
  .route { border-top:1px solid var(--line); padding-top:16px; }
  label { flex:1 1 140px; }
  .round-trip { display:flex; flex:0 0 auto; align-items:center; flex-direction:row; min-height:40px; }
  .round-trip input { width:18px; height:18px; }
  dl { padding:16px 0; border-block:1px solid var(--line); }
  dl div { flex:1 1 160px; } dt { color:var(--muted); } dd { margin:6px 0 0; font-weight:600; font-size:1.2rem; }
</style>
