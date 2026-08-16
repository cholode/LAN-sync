<template><div ref="el" class="chart"></div></template>
<script setup>
import * as echarts from 'echarts'
import { onBeforeUnmount, onMounted, ref, watch } from 'vue'
const props=defineProps({ option:{type:Object,required:true} }); const el=ref(); let chart; let ro
function render(){ if(!chart||!props.option)return; chart.setOption(props.option,true) }
onMounted(()=>{ chart=echarts.init(el.value); render(); ro=new ResizeObserver(()=>chart.resize()); ro.observe(el.value) })
watch(()=>props.option,render,{deep:true}); onBeforeUnmount(()=>{ro?.disconnect();chart?.dispose()})
</script>
<style scoped>.chart{width:100%;height:100%;min-height:260px}</style>
