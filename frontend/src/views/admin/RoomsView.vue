<template>
  <div class="page">
    <div><h1 class="page-title">群聊管理</h1><p class="page-subtitle">全局查看群聊活跃度、成员、Agent 与内容治理状态</p></div>
    <section class="app-card">
      <div class="filters">
        <div class="search"><Search :size="15"/><input v-model="q" placeholder="搜索群聊 / Room ID" @keyup.enter="load"/></div>
        <select v-model="sort" class="select" @change="load"><option value="active">活跃度排序</option><option value="members">成员数排序</option><option value="created">创建时间排序</option></select>
        <button class="btn" @click="load">查询</button>
        <StatusBadge :tone="error?'danger':'success'" dot>{{error?'接口异常':`共 ${total} 个群聊`}}</StatusBadge>
      </div>
      <div class="table-wrap"><table class="data-table"><thead><tr><th>群聊</th><th>群主</th><th>成员</th><th>在线</th><th>今日消息</th><th>Agent</th><th>违规</th><th>最后活跃</th><th></th></tr></thead><tbody><tr v-for="r in rooms" :key="r.id"><td><b>{{r.name||r.room_name||`Room #${r.id}`}}</b><small>#{{r.id}}</small></td><td>{{r.owner||r.owner_name||'—'}}</td><td>{{r.members??0}}</td><td>{{r.online??0}}</td><td>{{fmt(r.messages_today)}}</td><td><StatusBadge :tone="r.agent?'success':'neutral'" dot>{{r.agent?'启用':'关闭'}}</StatusBadge></td><td><span :class="r.violations?'risk':''">{{r.violations??0}}</span></td><td class="muted">{{formatTime(r.last_active)}}</td><td><div class="actions"><button class="btn btn-sm" @click="detail(r)">详情</button><button class="btn btn-sm" @click="manage(r)">管理</button><button class="btn btn-danger btn-sm" @click="remove(r)"><Trash2 :size="13"/>解散</button></div></td></tr><tr v-if="!rooms.length"><td colspan="9" class="empty">{{error||'暂无群聊数据'}}</td></tr></tbody></table></div>
    </section>

    <dialog ref="dialog" class="detail-dialog" @click.self="dialog?.close()">
      <article v-if="selected" class="detail-card">
        <header class="detail-head">
          <div class="detail-identity">
            <span class="detail-avatar">{{String(selected.name||selected.room_name||'R').slice(0,1).toUpperCase()}}</span>
            <div><h2>{{selected.name||selected.room_name||`Room #${selected.id}`}}</h2><p>群聊 ID：{{selected.id}} · 群主 ID：{{selected.owner_id||'—'}}</p></div>
          </div>
          <div class="detail-head-actions">
            <StatusBadge :tone="roomFrozen(selected)?'danger':'success'" dot>{{roomFrozen(selected)?'已冻结':'正常运行'}}</StatusBadge>
            <StatusBadge :tone="agentEnabled(selected)?'primary':'neutral'">Agent {{agentEnabled(selected)?'已启用':'未启用'}}</StatusBadge>
            <button class="dialog-close" aria-label="关闭" @click="dialog?.close()">×</button>
          </div>
        </header>

        <div class="detail-body">
          <section class="metric-grid">
            <div><span>成员总数</span><strong>{{fmt(memberCount(selected))}}</strong></div>
            <div><span>当前在线</span><strong>{{fmt(onlineCount(selected))}}</strong></div>
            <div><span>今日消息</span><strong>{{fmt(todayMessages(selected))}}</strong></div>
            <div><span>累计消息</span><strong>{{fmt(totalMessages(selected))}}</strong></div>
          </section>

          <section class="detail-section">
            <h3>群聊信息</h3>
            <dl class="info-grid">
              <div><dt>群聊类型</dt><dd>{{roomTypeLabel(selected.type)}}</dd></div>
              <div><dt>运行状态</dt><dd>{{roomFrozen(selected)?'已冻结':'正常'}}</dd></div>
              <div><dt>创建时间</dt><dd>{{formatTime(selected.created_at)}}</dd></div>
              <div><dt>最后活跃</dt><dd>{{formatTime(selected.last_active_at||selected.last_active)}}</dd></div>
              <div><dt>内容治理</dt><dd>{{selected.moderation_enabled?'已启用':'未启用'}}</dd></div>
              <div><dt>今日违规</dt><dd :class="violationCount(selected)?'risk':''">{{violationCount(selected)}} 条</dd></div>
            </dl>
          </section>

          <section class="detail-section">
            <div class="section-title"><h3>群聊成员</h3><span>{{memberCount(selected)}} 人</span></div>
            <div v-if="Array.isArray(selected.members)&&selected.members.length" class="member-list">
              <div v-for="member in selected.members" :key="member.user_id" class="member-item">
                <span class="member-avatar">{{String(member.username||'U').slice(0,1).toUpperCase()}}</span>
                <div class="member-name"><b>{{member.username||`用户 #${member.user_id}`}}</b><small>#{{member.user_id}}</small></div>
                <StatusBadge :tone="member.online?'success':'neutral'" dot>{{member.online?'在线':'离线'}}</StatusBadge>
                <StatusBadge tone="neutral">{{memberRoleLabel(member.role)}}</StatusBadge>
              </div>
            </div>
            <div v-else class="detail-empty">暂无成员数据</div>
          </section>

          <section v-if="selected.agent_config" class="detail-section">
            <div class="section-title"><h3>Agent 配置</h3><span>{{agentEnabled(selected)?'运行中':'未启用'}}</span></div>
            <dl class="agent-grid">
              <div><dt>模型</dt><dd>{{selected.agent_config.model_name||'默认模型'}}</dd></div>
              <div><dt>触发模式</dt><dd>{{triggerModeLabel(selected.agent_config.trigger_mode)}}</dd></div>
              <div><dt>RAG</dt><dd>{{selected.agent_config.rag_enabled?'已启用':'未启用'}}</dd></div>
              <div><dt>Top K</dt><dd>{{selected.agent_config.top_k??'—'}}</dd></div>
            </dl>
          </section>

          <section v-if="Array.isArray(selected.violations)&&selected.violations.length" class="detail-section">
            <div class="section-title"><h3>最近违规</h3><span>显示最近 {{Math.min(selected.violations.length,3)}} 条</span></div>
            <div class="violation-list">
              <div v-for="item in selected.violations.slice(0,3)" :key="item.id" class="violation-item">
                <div><b>{{item.username||item.category||'违规事件'}}</b><small>{{formatTime(item.created_at)}}</small></div>
                <p>{{item.model_reason||item.original_msg||item.review_status||'暂无事件说明'}}</p>
              </div>
            </div>
          </section>
        </div>
      </article>
    </dialog>
  </div>
</template>

<script setup>
import{onMounted,ref}from'vue';import{Search,Trash2}from'lucide-vue-next';import StatusBadge from'../../components/common/StatusBadge.vue';import{adminApi}from'../../api/admin.js';
const rooms=ref([]),q=ref(''),sort=ref('active'),error=ref(''),total=ref(0),dialog=ref(),selected=ref(null);
const fmt=v=>v==null?'—':new Intl.NumberFormat('zh-CN',{notation:Number(v)>9999?'compact':'standard'}).format(v);const formatTime=v=>v?new Date(v).toLocaleString('zh-CN'):'—';
const memberCount=r=>r?.member_count??(Array.isArray(r?.members)?r.members.length:r?.members??0);const onlineCount=r=>r?.online_member_count??r?.online??0;const todayMessages=r=>r?.today_message_count??r?.messages_today??0;const totalMessages=r=>r?.total_message_count??r?.messages??0;const violationCount=r=>r?.violation_count??(Array.isArray(r?.violations)?r.violations.length:r?.violations??0);
const agentEnabled=r=>Boolean(r?.agent_enabled??r?.agent);const roomFrozen=r=>Number(r?.status)===1;const roomTypeLabel=type=>({0:'普通群聊',1:'公开群聊',2:'私密群聊'})[String(type)]||'普通群聊';const memberRoleLabel=role=>({1:'成员',2:'管理员',3:'群主'})[String(role)]||'成员';const triggerModeLabel=mode=>({1:'@提及时',2:'全部消息',3:'关键词触发'})[String(mode)]||'默认模式';
async function load(){error.value='';try{const d=await adminApi.rooms({q:q.value,sort:sort.value,page_size:100});rooms.value=Array.isArray(d)?d:(d?.items||d?.rooms||[]);total.value=d?.total??rooms.value.length}catch(e){rooms.value=[];error.value=e.message}}
async function detail(r){try{selected.value=await adminApi.room(r.id)}catch{selected.value=r}dialog.value?.showModal()}
async function manage(r){const action=prompt('操作：freeze / unfreeze / agent_enable / agent_disable / moderation_enable / moderation_disable / remove_member / set_admin / transfer_owner','freeze');if(!action)return;let target=0;if(['remove_member','set_admin','transfer_owner'].includes(action.trim())){target=Number(prompt('目标用户 ID',''))||0;if(!target)return}if(!confirm(`对房间 ${r.name||r.id} 执行 ${action}？`))return;try{await adminApi.roomAction(r.id,action.trim(),target);await load()}catch(e){alert(e.message)}}
async function remove(r){if(!confirm(`确定强制解散「${r.name||r.room_name||r.id}」？`))return;try{await adminApi.deleteRoom(r.id);await load()}catch(e){alert(e.message)}}onMounted(load)
</script>

<style scoped>
.page{display:grid;gap:16px}.filters{padding:14px;display:flex;gap:10px;align-items:center}.search{height:38px;width:320px;display:flex;align-items:center;gap:8px;padding:0 11px;border:1px solid var(--line-strong);border-radius:9px;color:var(--text-3)}.search input{border:0;outline:0;background:transparent;color:var(--text);width:100%}.select{width:160px}td small{display:block;color:var(--text-3);font-size:10px;margin-top:3px}.risk{color:var(--danger);font-weight:700}.actions{display:flex;justify-content:flex-end;gap:6px}.empty{text-align:center!important;color:var(--text-3);padding:30px!important}
.detail-dialog{width:min(720px,calc(100vw - 28px));max-height:min(800px,calc(100vh - 40px));border:1px solid var(--line);border-radius:16px;background:var(--surface);color:var(--text);padding:0;box-shadow:var(--shadow-lg);overflow:hidden}.detail-dialog::backdrop{background:rgba(10,18,32,.52);backdrop-filter:blur(2px)}.detail-card{display:flex;flex-direction:column;max-height:inherit}.detail-head{display:flex;justify-content:space-between;align-items:center;gap:18px;padding:18px 20px;border-bottom:1px solid var(--line);background:var(--surface)}.detail-identity{display:flex;align-items:center;gap:12px;min-width:0}.detail-avatar{width:44px;height:44px;flex:0 0 auto;border-radius:13px;background:var(--primary-soft);color:var(--primary);display:grid;place-items:center;font-size:18px;font-weight:800}.detail-identity h2{font-size:17px;margin:0;white-space:nowrap;overflow:hidden;text-overflow:ellipsis}.detail-identity p{margin:4px 0 0;color:var(--text-3);font-size:11px}.detail-head-actions{display:flex;align-items:center;gap:7px}.dialog-close{width:30px;height:30px;border:0;border-radius:8px;background:var(--surface-soft);color:var(--text-2);font-size:20px;line-height:1;cursor:pointer}.dialog-close:hover{background:var(--danger-soft);color:var(--danger)}.detail-body{padding:18px 20px 22px;overflow:auto}.metric-grid{display:grid;grid-template-columns:repeat(4,1fr);gap:9px}.metric-grid>div{padding:13px;border:1px solid var(--line);border-radius:11px;background:var(--surface-soft)}.metric-grid span{display:block;color:var(--text-3);font-size:10px}.metric-grid strong{display:block;margin-top:5px;font-size:19px}.detail-section{margin-top:20px}.detail-section h3{font-size:12px;margin:0 0 10px}.section-title{display:flex;align-items:center;justify-content:space-between}.section-title span{color:var(--text-3);font-size:10px}.info-grid,.agent-grid{display:grid;grid-template-columns:repeat(2,1fr);margin:0;border:1px solid var(--line);border-radius:11px;overflow:hidden}.info-grid>div,.agent-grid>div{padding:11px 13px;border-bottom:1px solid var(--line)}.info-grid>div:nth-child(odd),.agent-grid>div:nth-child(odd){border-right:1px solid var(--line)}.info-grid>div:nth-last-child(-n+2),.agent-grid>div:nth-last-child(-n+2){border-bottom:0}.info-grid dt,.agent-grid dt{color:var(--text-3);font-size:10px}.info-grid dd,.agent-grid dd{margin:4px 0 0;font-size:12px;font-weight:600}.member-list{display:grid;gap:7px;max-height:260px;overflow:auto}.member-item{display:flex;align-items:center;gap:9px;padding:9px 11px;border:1px solid var(--line);border-radius:10px}.member-avatar{width:30px;height:30px;flex:0 0 auto;border-radius:9px;background:var(--primary-soft);color:var(--primary);display:grid;place-items:center;font-size:11px;font-weight:800}.member-name{min-width:0;flex:1}.member-name b,.member-name small{display:block}.member-name b{font-size:11px}.member-name small{margin-top:2px;color:var(--text-3);font-size:9px}.violation-list{display:grid;gap:7px}.violation-item{padding:11px 12px;border-left:3px solid var(--danger);border-radius:8px;background:var(--danger-soft)}.violation-item>div{display:flex;justify-content:space-between;gap:12px}.violation-item b{font-size:11px}.violation-item small{color:var(--text-3);font-size:10px}.violation-item p{margin:6px 0 0;color:var(--text-2);font-size:11px;line-height:1.55;display:-webkit-box;-webkit-line-clamp:2;-webkit-box-orient:vertical;overflow:hidden}.detail-empty{padding:20px;border:1px dashed var(--line-strong);border-radius:10px;text-align:center;color:var(--text-3);font-size:11px}
@media(max-width:720px){.filters{align-items:stretch;flex-direction:column}.search,.select{width:100%}.detail-head{align-items:flex-start}.detail-head-actions>.status-badge{display:none}.metric-grid{grid-template-columns:repeat(2,1fr)}.info-grid,.agent-grid{grid-template-columns:1fr}.info-grid>div,.agent-grid>div,.info-grid>div:nth-child(odd),.agent-grid>div:nth-child(odd),.info-grid>div:nth-last-child(-n+2),.agent-grid>div:nth-last-child(-n+2){border-right:0;border-bottom:1px solid var(--line)}.info-grid>div:last-child,.agent-grid>div:last-child{border-bottom:0}}
</style>
