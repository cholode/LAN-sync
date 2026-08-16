<template>
  <div class="page">
    <div class="page-head">
      <div><h1 class="page-title">内容治理</h1><p class="page-subtitle">Agent 审核、违规事件、自动工具执行与人工复核</p></div>
      <div class="head-actions"><StatusBadge :tone="error?'danger':'success'" dot>{{ error ? '审核接口异常' : '审核数据已同步' }}</StatusBadge><button class="btn" @click="load"><RefreshCw :size="14"/>刷新</button></div>
    </div>
    <div class="summary">
      <div class="app-card"><span>今日审核</span><b>{{ fmt(summary.reviewed) }}</b><small>消息审核任务</small></div>
      <div class="app-card"><span>违规率</span><b>{{ summary.violation_rate ?? 0 }}%</b><small>基于已记录审核事件</small></div>
      <div class="app-card"><span>自动处置</span><b>{{ fmt(summary.auto_actions) }}</b><small>Kick / Mute / Ban</small></div>
      <div class="app-card"><span>待人工复核</span><b>{{ fmt(summary.pending_review) }}</b><small class="warn">需要处理</small></div>
    </div>
    <section class="app-card">
      <div class="filters">
        <select v-model="risk" class="select" @change="load"><option value="">全部风险</option><option value="high">高风险</option><option value="medium">中风险</option><option value="low">低风险</option></select>
        <div class="search"><Search :size="15"/><input v-model="q" placeholder="搜索用户、群聊、事件 ID" @keyup.enter="load"/></div>
        <button class="btn" @click="load">查询</button>
      </div>
      <div class="table-wrap"><table class="data-table"><thead><tr><th>事件</th><th>时间</th><th>用户 / 群聊</th><th>分类</th><th>风险</th><th>模型摘要</th><th>Tool</th><th>结果</th><th></th></tr></thead><tbody>
        <tr v-for="e in items" :key="e.id"><td><b>{{e.id}}</b></td><td class="muted">{{e.time||formatTime(e.created_at)}}</td><td><b>{{e.user||e.username||`User #${e.user_id||'—'}`}}</b><small>{{e.room||e.room_name||`Room #${e.room_id||'—'}`}}</small></td><td>{{e.category||'其他'}}</td><td><StatusBadge :tone="e.risk==='high'?'danger':e.risk==='medium'?'warning':'neutral'">{{e.risk||'low'}}</StatusBadge></td><td class="summary-cell">{{e.summary||e.reason||'—'}}</td><td><code>{{e.action||'RecordOnly'}}</code></td><td><StatusBadge :tone="e.result==='failed'?'danger':e.review_status==='pending'?'warning':'success'">{{e.result||e.review_status||'recorded'}}</StatusBadge></td><td><button class="btn btn-sm" :disabled="reviewing===e.id" @click="review(e)">{{ reviewing===e.id?'提交中':'复核' }}</button></td></tr>
        <tr v-if="!items.length"><td colspan="9" class="empty">{{error||'暂无违规事件'}}</td></tr>
      </tbody></table></div>
    </section>
  </div>
</template>
<script setup>
import{onMounted,ref}from'vue';import{Search,RefreshCw}from'lucide-vue-next';import StatusBadge from'../../components/common/StatusBadge.vue';import{adminApi}from'../../api/admin.js';
const items=ref([]),summary=ref({reviewed:0,violation_rate:0,auto_actions:0,pending_review:0}),risk=ref(''),q=ref(''),error=ref(''),reviewing=ref(null)
const fmt=v=>new Intl.NumberFormat('zh-CN',{notation:Number(v)>9999?'compact':'standard'}).format(Number(v||0));const formatTime=v=>v?new Date(v).toLocaleString('zh-CN'):'—'
async function load(){error.value='';try{const d=await adminApi.moderation({risk:risk.value,q:q.value,page_size:100});items.value=Array.isArray(d)?d:(d?.items||[]);summary.value=d?.summary||summary.value}catch(e){items.value=[];error.value=e.message}}
async function review(e){const decision=prompt('复核结果：approved（确认违规）/ false_positive（误判）','approved');if(!decision)return;reviewing.value=e.id;try{const d=await adminApi.reviewModeration(e.id,{decision,note:''});Object.assign(e,d?.event||{review_status:decision})}catch(err){alert(err.message)}finally{reviewing.value=null;await load()}}
onMounted(load)
</script>
<style scoped>.page{display:grid;gap:14px}.page-head,.head-actions{display:flex;justify-content:space-between;align-items:flex-start;gap:10px}.summary{display:grid;grid-template-columns:repeat(4,1fr);gap:12px}.summary>div{padding:15px}.summary span,.summary b,.summary small{display:block}.summary span{font-size:11px;color:var(--text-3)}.summary b{font-size:24px;margin:5px 0}.summary small{color:var(--text-3)}.warn{color:var(--warning)!important}.filters{padding:14px;display:flex;gap:10px}.select{width:140px}.search{height:38px;width:330px;display:flex;align-items:center;gap:8px;padding:0 11px;border:1px solid var(--line-strong);border-radius:9px;color:var(--text-3)}.search input{border:0;outline:0;background:transparent;color:var(--text);width:100%}td small{display:block;color:var(--text-3);font-size:10px;margin-top:3px}.summary-cell{max-width:260px;color:var(--text-2);font-size:11px}code{font-size:10px;background:var(--surface-muted);padding:4px 6px;border-radius:6px}.empty{text-align:center!important;color:var(--text-3);padding:30px!important}@media(max-width:980px){.summary{grid-template-columns:repeat(2,1fr)}}</style>
