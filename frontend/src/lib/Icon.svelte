<script lang="ts">
  let { name, size = 20 }: { name: string; size?: number } = $props();
  const paths: Record<string, string> = {
    fuel: 'M3 21h11M5 21V3h8v18M5 10h8m0 3h2a2 2 0 0 1 2 2v2a2 2 0 0 0 4 0V9l-4-4',
    grid: 'M3 3h7v7H3z M14 3h7v7h-7z M3 14h7v7H3z M14 14h7v7h-7z',
    arrows: 'M4 7h16m-4-4 4 4-4 4M20 17H4m4-4-4 4 4 4',
    wallet:
      'M20 8V5a2 2 0 0 0-2-2H5a3 3 0 0 0 0 6h15v12H5a3 3 0 0 1-3-3V6m18 7h-6v4h6',
    home: 'm3 10 9-7 9 7v11h-7v-7h-4v7H3z',
    repeat:
      'M3 8a8 8 0 0 1 14-4l3 3M20 2v5h-5M21 16a8 8 0 0 1-14 4l-3-3M4 22v-5h5',
    chart: 'M4 3v17h17M8 16v-5m5 5V7m5 9V4',
    settings:
      'M12 8a4 4 0 1 0 0 8 4 4 0 0 0 0-8M12 2v3m0 14v3M2 12h3m14 0h3M5 5l2 2m10 10 2 2M5 19l2-2M17 7l2-2',
    plus: 'M12 5v14M5 12h14',
    chevron: 'm9 5 7 7-7 7',
    down: 'm6 9 6 6 6-6',
    up: 'M12 19V5m-6 6 6-6 6 6',
    out: 'M12 5v14m-6-6 6 6 6-6',
    eye: 'M2 12s4-7 10-7 10 7 10 7-4 7-10 7S2 12 2 12m10-3a3 3 0 1 0 0 6 3 3 0 0 0 0-6',
    hidden:
      'm3 3 18 18M10 5c7-1 12 7 12 7s-1 2-4 4M6 6c-3 2-4 6-4 6s4 7 10 7c2 0 3-1 4-1',
    search: 'M10 3a7 7 0 1 0 0 14 7 7 0 0 0 0-14m5 12 6 6',
    calendar: 'M7 2v4m10-4v4M3 10h18M3 4h18v17H3z',
    logout: 'M9 4H3v16h6m5-13 5 5-5 5M8 12h13',
    close: 'm6 6 12 12M6 18 18 6',
    check: 'm5 12 4 4L19 6',
    clock: 'M12 3a9 9 0 1 0 0 18 9 9 0 0 0 0-18m0 4v5l3 2',
    shield: 'm12 2 8 4v6c0 5-8 10-8 10S4 17 4 12V6zm-4 10 3 3 5-5',
    menu: 'M4 6h16M4 12h16M4 18h16',
    leaf: 'M20 3C8 1 2 8 6 16s17 3 14-13M4 21 16 9',
    edit: 'm15 4 5 5M4 20l1-6L16 3l5 5-11 11z',
    trash: 'M3 6h18M9 6V3h6v3M5 6l1 15h12l1-15M10 10v7m4-7v7',
    info: 'M12 3a9 9 0 1 0 0 18 9 9 0 0 0 0-18m0 8v6m0-10v1',
    arrow: 'M5 12h14m-5-5 5 5-5 5',
    bank: 'm3 7 9-5 9 5H3m2 3v8m7-8v8m7-8v8M2 21h20',
    spark: 'm12 3 2.5 6.5L21 12l-6.5 2.5L12 21l-2.5-6.5L3 12l6.5-2.5z',
  };
</script>

<svg
  width={size}
  height={size}
  viewBox="0 0 24 24"
  fill="none"
  stroke="currentColor"
  stroke-width="1.7"
  stroke-linecap="round"
  stroke-linejoin="round"
  aria-hidden="true"><path d={paths[name] || paths.wallet} /></svg
>
