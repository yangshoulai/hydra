<template>
  <section v-if="$slots.actions || $slots.default" class="page-shell">
    <header v-if="$slots.actions" class="page-shell__header toolbar">
      <div class="toolbar__right" style="margin-left: auto">
        <slot name="actions" />
      </div>
    </header>
    <div class="page-shell__content" v-if="$slots.default">
      <slot />
    </div>
  </section>
</template>

<script setup lang="ts">
defineProps<{
  title?: string
  subtitle?: string
}>()
</script>
