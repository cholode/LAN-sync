<template>
  <div class="page">
    <div class="page-head">
      <div><h1 class="page-title">Agent 与 RAG 管理</h1><p class="page-subtitle">维护 Agent 策略、配置版本和 RAG 查询记录；运行指标统一在 Grafana 查看</p></div>
      <button class="btn" @click="load"><RefreshCw :size="14"/>刷新</button>
    </div>
    <div class="two-col">
      <section class="app-card config">
        <div class="panel-head"><div><h3>Agent 策略</h3><p>配置变更会写入审计日志，API Key 不在此页面展示。</p></div><SlidersHorizontal :size="17"/></div>
        <label>默认模型<input v-model="config.default_model" class="input"/></label>
        <label>Embedding 模型<input v-model="config.embedding_model" class="input"/></label>
        <div class="form-row"><label>RAG Top-K<input v-model.number="config.rag_top_k" class="input" type="number" min="1" max="50"/></label><label>Similarity Threshold<input v-model.number="config.similarity_threshold" class="input" type="number" min="0" max="1" step="0.01"/></label></div>
        <div class="form-row"><label>Chunk Size<input v-model.number="config.chunk_size" class="input" type="number" min="128"/></label><label>Chunk Overlap<input v-model.number="config.chunk_overlap" class="input" type="number" min="0"/></label></div>
        <label>System Prompt<textarea v-model="config.system_prompt" class="textarea" rows="6"></textarea></label>
        <div class="save"><span>{{saveMessage||error||'修改会记录管理员与修改时间'}}</span><button class="btn btn-primary" :disabled="saving" @click="save">{{saving?'保存中…':'保存策略'}}</button></div>
      </section>
      <section class="app-card records">
        <div class="panel-head"><div><h3>配置版本</h3><p>最近的配置变更，可回滚到上一版本</p></div><button class="btn btn-sm" @click="rollback">回滚上一版本</button></div>
        <div class="table-wrap"><table class="data-table"><thead><tr><th>版本</th><th>管理员</th><th>时间</th></tr></thead><tbody><tr v-for="row in history" :key="row.id"><td>v{{row.version}}</td><td>{{row.admin_username||row.admin_user_id}}</td><td>{{fmtTime(row.created_at)}}</td></tr><tr v-if="!history.length"><td colspan="3" class="empty">暂无配置历史</td></tr></tbody></table></div>
      </section>
    </div>
    <section class="app-card records">
      <div class="panel-head"><div><h3>RAG 查询记录</h3><p>用于检查召回结果；Qdrant 性能和 Agent 调用延迟请在 Grafana 查看</p></div></div>
      <div class="table-wrap"><table class="data-table"><thead><tr><th>问题</th><th>房间</th><th>召回</th><th>耗时</th></tr></thead><tbody><tr v-for="row in queries" :key="row.id"><td class="question">{{row.question}}</td><td>#{{row.room_id}}</td><td>{{row.retrieved_count}}</td><td>{{row.query_latency_ms}} ms</td></tr><tr v-if="!queries.length"><td colspan="4" class="empty">暂无 RAG 查询</td></tr></tbody></table></div>
    </section>
  </div>
</template>
<script setup>
import{onMounted,reactive,ref}from'vue';import{SlidersHorizontal,RefreshCw}from'lucide-vue-next';import{adminApi}from'../../api/admin.js';
const error=ref(''),saving=ref(false),saveMessage=ref(''),history=ref([]),queries=ref([]);const config=reactive({default_model:'',embedding_model:'',temperature:0.2,max_tokens:2048,rag_top_k:8,similarity_threshold:.72,chunk_size:768,chunk_overlap:96,system_prompt:''});const fmtTime=v=>v?new Date(v).toLocaleString('zh-CN'):'—';
async function load(){error.value='';try{const [cfg,historyData,queryData]=await Promise.all([adminApi.agentConfig(),adminApi.agentConfigHistory(),adminApi.ragQueries()]);Object.assign(config,cfg||{});history.value=historyData?.items||[];queries.value=queryData?.items||[]}catch(e){error.value=e.message}}
async function save(){saving.value=true;saveMessage.value='';try{const r=await adminApi.saveAgentConfig({...config});Object.assign(config,r?.config||r||{});saveMessage.value='已保存并写入审计日志'}catch(e){saveMessage.value=e.message}finally{saving.value=false}}
async function rollback(){if(!confirm('回滚到上一版本 Agent 配置？'))return;try{const r=await adminApi.rollbackAgentConfig();Object.assign(config,r?.config||r||{});saveMessage.value='已回滚并写入审计日志';await load()}catch(e){saveMessage.value=e.message}}
onMounted(load)
</script>
<style scoped>.page{display:grid;gap:14px}.page-head{display:flex;justify-content:space-between;align-items:flex-start;gap:10px}.config,.records{padding:18px}.panel-head{display:flex;justify-content:space-between;color:var(--text-2);margin-bottom:16px}.panel-head h3{margin:0;color:var(--text);font-size:14px}.panel-head p{margin:4px 0 0;font-size:11px;color:var(--text-3)}label{display:grid;gap:6px;font-size:11px;font-weight:650;margin-bottom:12px}.form-row{display:grid;grid-template-columns:1fr 1fr;gap:12px}.save{display:flex;justify-content:space-between;align-items:center;color:var(--text-3);font-size:10px;gap:10px}.question{max-width:560px;white-space:nowrap;overflow:hidden;text-overflow:ellipsis}.empty{text-align:center!important;color:var(--text-3)}</style>
