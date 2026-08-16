<template>
  <div class="metric app-card">
    <div class="metric-head"><div class="icon" :class="tone"><component :is="icon" :size="18" /></div><span v-if="status" class="status">{{ status }}</span></div>
    <div class="label">{{ label }}</div>
    <div class="value-row"><strong>{{ displayValue }}</strong><span v-if="suffix" class="suffix">{{ suffix }}</span></div>
    <div class="foot"><span v-if="delta !== undefined" :class="delta >= 0 ? 'up' : 'down'">{{ delta >= 0 ? '↑' : '↓' }} {{ Math.abs(delta) }}%</span><span>{{ hint }}</span></div>
  </div>
</template>
<script setup>
import { computed } from 'vue'
const props=defineProps({ label:String,value:[String,Number],suffix:String,hint:String,delta:Number,status:String,icon:[Object,Function],tone:{type:String,default:'primary'} })
const displayValue=computed(()=>typeof props.value==='number'?new Intl.NumberFormat('zh-CN',{notation:props.value>99999?'compact':'standard',maximumFractionDigits:1}).format(props.value):(props.value??'—'))
</script>
<style scoped>
.metric{padding:17px 18px;min-height:154px}.metric-head{display:flex;align-items:center;justify-content:space-between;margin-bottom:17px}.icon{width:36px;height:36px;border-radius:10px;display:grid;place-items:center;background:var(--primary-soft);color:var(--primary)}.icon.success{background:var(--success-soft);color:var(--success)}.icon.warning{background:var(--warning-soft);color:var(--warning)}.icon.purple{background:#f2efff;color:var(--purple)}.status{font-size:11px;color:var(--text-3)}.label{color:var(--text-2);font-size:12px}.value-row{display:flex;align-items:baseline;gap:5px;margin-top:5px}.value-row strong{font-size:28px;letter-spacing:-.03em}.suffix{color:var(--text-3);font-size:12px}.foot{display:flex;align-items:center;gap:8px;color:var(--text-3);font-size:11px;margin-top:9px}.up{color:var(--success);font-weight:650}.down{color:var(--danger);font-weight:650}
</style>
