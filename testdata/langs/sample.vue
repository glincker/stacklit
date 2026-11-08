<template>
  <div>{{ msg }}</div>
</template>

<script setup lang="ts">
import { ref } from 'vue'
import Child from './Child.vue'

export interface Props {
  msg: string
}

export function helper(a: number): number {
  return a
}

const count = ref(0)
</script>
