<template><span class="badge" :class="tone"><span v-if="dot" class="dot"></span><slot /></span></template>
<script setup>defineProps({ tone: { type: String, default: 'neutral' }, dot: Boolean })</script>
<style scoped>
.badge{display:inline-flex;align-items:center;gap:6px;padding:4px 8px;border-radius:999px;font-size:11px;font-weight:650;background:var(--surface-muted);color:var(--text-2);white-space:nowrap}.dot{width:6px;height:6px;border-radius:50%;background:currentColor}.success{background:var(--success-soft);color:var(--success)}.warning{background:var(--warning-soft);color:var(--warning)}.danger{background:var(--danger-soft);color:var(--danger)}.primary{background:var(--primary-soft);color:var(--primary)}
</style>
