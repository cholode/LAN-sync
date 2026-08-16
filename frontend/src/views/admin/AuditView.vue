<template>
  <div class="page">
    <div class="page-head"><div><h1 class="page-title">审计日志</h1><p class="page-subtitle">记录超级管理员敏感操作与 Agent 自动治理动作</p></div><button class="btn" @click="load"><RefreshCw :size="14"/>刷新</button></div>
    <section class="app-card">
      <div class="filters">
        <div class="search"><Search :size="15"/><input v-model="q" placeholder="搜索管理员 / action / target / Request ID" @keyup.enter="load"/></div>
        <select v-model="action" class="select" @change="load"><option value="">全部操作</option><option value="user.delete">user.delete</option><option value="room.delete">room.delete</option><option value="agent.config">agent.config</option><option value="moderation.review">moderation.review</option></select>
        <button class="btn" @click="load">查询</button>
        <StatusBadge :tone="error?'danger':'success'" dot>{{error?'接口异常':`共 ${total} 条`}}</StatusBadge>
      </div>
      <div class="table-wrap"><table class="data-table"><thead><tr><th>时间</th><th>操作者</th><th>Action</th><th>目标</th><th>Request ID</th><th>IP</th><th>结果</th></tr></thead><tbody><tr v-for="a in rows" :key="a.id"><td>{{formatTime(a.created_at)}}</td><td>{{a.admin||a.admin_user_id}}</td><td><code>{{a.action}}</code></td><td>{{a.target_type}} #{{a.target_id}}</td><td class="muted">{{a.request_id}}</td><td>{{a.remote_ip}}</td><td><StatusBadge :tone="a.result==='success'?'success':'danger'">{{a.result}}</StatusBadge></td></tr><tr v-if="!rows.length"><td colspan="7" class="empty">{{error||'暂无审计日志'}}</td></tr></tbody></table></div>
    </section>
  </div>
</template>
<script setup>
import{onMounted,ref}from'vue';import{Search,RefreshCw}from'lucide-vue-next';import StatusBadge from'../../components/common/StatusBadge.vue';import{adminApi}from'../../api/admin.js';
const rows=ref([]),q=ref(''),action=ref(''),error=ref(''),total=ref(0);const formatTime=v=>v?new Date(v).toLocaleString('zh-CN'):'—';
async function load(){error.value='';try{const d=await adminApi.audits({q:q.value,action:action.value,page_size:100});rows.value=Array.isArray(d)?d:(d?.items||[]);total.value=d?.total??rows.value.length}catch(e){rows.value=[];total.value=0;error.value=e.message}}
onMounted(load)
</script>
<style scoped>.page{display:grid;gap:16px}.page-head{display:flex;justify-content:space-between;align-items:flex-start}.filters{padding:14px;display:flex;gap:10px;align-items:center;flex-wrap:wrap}.search{height:38px;width:360px;display:flex;align-items:center;gap:8px;padding:0 11px;border:1px solid var(--line-strong);border-radius:9px;color:var(--text-3)}.search input{border:0;outline:0;background:transparent;color:var(--text);width:100%}.select{width:180px}code{font-size:10px;background:var(--surface-muted);padding:4px 6px;border-radius:6px}.empty{text-align:center!important;color:var(--text-3);padding:30px!important}</style>
