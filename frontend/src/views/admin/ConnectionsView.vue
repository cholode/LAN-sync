<template>
  <div class="page">
    <div class="page-head"><div><h1 class="page-title">连接管理</h1><p class="page-subtitle">WebSocket 会话、发送队列与强制下线</p></div><button class="btn" @click="load"><RefreshCw :size="14"/>刷新</button></div>
    <section class="app-card">
      <div class="filters"><div class="search"><Search :size="15"/><input v-model="q" placeholder="用户、IP、连接 ID" @keyup.enter="load"/></div><button class="btn" @click="load">查询</button><StatusBadge :tone="error?'danger':'success'" dot>{{error||`共 ${rows.length} 条连接`}}</StatusBadge></div>
      <div class="table-wrap"><table class="data-table"><thead><tr><th>用户</th><th>连接 ID</th><th>IP / 客户端</th><th>加入房间</th><th>发送队列</th><th>连接时间</th><th></th></tr></thead><tbody>
        <tr v-for="row in rows" :key="row.connection_id"><td><b>{{row.username||`User #${row.user_id}`}}</b><small>#{{row.user_id}}</small></td><td><code>{{row.connection_id}}</code></td><td>{{row.remote_ip||'—'}}<small>{{row.client_version||row.user_agent||'—'}}</small></td><td>{{row.room_ids?.length??0}}</td><td><StatusBadge :tone="row.send_queue_len>0?'warning':'success'">{{row.send_queue_len??0}}</StatusBadge></td><td>{{fmtTime(row.connected_at)}}</td><td><div class="actions"><button class="btn btn-sm" @click="close(row)">关闭连接</button><button class="btn btn-sm btn-danger" @click="offline(row)">强制下线</button></div></td></tr>
        <tr v-if="!rows.length"><td colspan="7" class="empty">{{error||'暂无在线连接'}}</td></tr>
      </tbody></table></div>
    </section>
  </div>
</template>
<script setup>
import{onMounted,ref}from'vue';import{RefreshCw,Search}from'lucide-vue-next';import StatusBadge from'../../components/common/StatusBadge.vue';import{adminApi}from'../../api/admin.js';
const rows=ref([]),q=ref(''),error=ref('');const fmtTime=v=>v?new Date(v).toLocaleString('zh-CN'):'—';
async function load(){error.value='';try{const d=await adminApi.connections({q:q.value});rows.value=d?.items||[]}catch(e){rows.value=[];error.value=e.message}}
async function close(row){if(!confirm(`关闭连接 ${row.connection_id}？`))return;try{await adminApi.closeConnection(row.connection_id);await load()}catch(e){alert(e.message)}}
async function offline(row){if(!confirm(`强制用户 ${row.username||row.user_id} 下线？`))return;try{await adminApi.forceOffline(row.user_id);await load()}catch(e){alert(e.message)}}
onMounted(load)
</script>
<style scoped>.page{display:grid;gap:16px}.page-head{display:flex;justify-content:space-between}.filters{padding:14px;display:flex;gap:10px;align-items:center}.search{height:38px;width:320px;display:flex;align-items:center;gap:8px;padding:0 11px;border:1px solid var(--line-strong);border-radius:9px}.search input{border:0;outline:0;background:transparent;color:var(--text);width:100%}td small{display:block;color:var(--text-3);font-size:10px;margin-top:3px}code{font-size:10px}.actions{display:flex;justify-content:flex-end;gap:6px}.empty{text-align:center!important;color:var(--text-3);padding:30px!important}</style>
