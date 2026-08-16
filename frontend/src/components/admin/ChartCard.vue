<template><section class="app-card chart-card"><header><div><h3>{{ title }}</h3><p v-if="subtitle">{{ subtitle }}</p></div><slot name="actions" /></header><div class="body"><slot /></div></section></template>
<script setup>defineProps({title:String,subtitle:String})</script>
<style scoped>.chart-card{overflow:hidden}header{display:flex;justify-content:space-between;gap:12px;align-items:flex-start;padding:16px 18px 10px}h3{font-size:14px;margin:0}p{font-size:11px;color:var(--text-3);margin:4px 0 0}.body{height:310px;padding:0 8px 10px}</style>
