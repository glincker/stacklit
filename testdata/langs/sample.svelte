<script lang="ts">
  import { onMount } from 'svelte';
  import Child from './Child.svelte';

  export let name: string;
  export function reset(): void {}
  export class Counter {}
</script>

<h1>Hello {name}</h1>
