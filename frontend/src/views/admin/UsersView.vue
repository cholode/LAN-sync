<template>
  <div class="page">
    <div class="page-head"><div><h1 class="page-title">用户管理</h1><p class="page-subtitle">账号状态、角色、在线连接与违规风险</p></div><button class="btn" @click="exportCsv"><Download :size="15"/>导出</button></div>
    <section class="app-card"><div class="filters"><div class="search"><Search :size="15"/><input v-model="q" placeholder="搜索用户名 / 用户 ID" @keyup.enter="load"/></div><select v-model="status" class="select" @change="load"><option value="">全部状态</option><option>online</option><option>offline</option><option>banned</option></select><button class="btn" @click="load">查询</button><StatusBadge :tone="error?'danger':'success'" dot>{{error?'接口异常':`共 ${total} 人`}}</StatusBadge></div>
      <div class="table-wrap"><table class="data-table"><thead><tr><th>用户</th><th>角色</th><th>状态</th><th>群聊</th><th>消息</th><th>违规</th><th>最后活跃</th><th></th></tr></thead><tbody><tr v-for="u in users" :key="u.id"><td><div class="user"><span>{{String(u.username||'U').slice(0,1).toUpperCase()}}</span><div><b>{{u.username||`User #${u.id}`}}</b><small>#{{u.id}}</small></div></div></td><td><StatusBadge :tone="u.role==='super_admin'?'primary':'neutral'">{{u.role||'user'}}</StatusBadge></td><td><StatusBadge dot :tone="u.status==='online'?'success':u.status==='banned'?'danger':'neutral'">{{u.status||'offline'}}</StatusBadge></td><td>{{u.rooms??0}}</td><td>{{fmt(u.messages)}}</td><td><b :class="u.violations?'risk':''">{{u.violations??0}}</b></td><td class="muted">{{formatTime(u.last_active)}}</td><td><div class="actions"><button class="btn btn-sm" @click="detail(u)">详情</button><button class="btn btn-sm" @click="manage(u)">管理</button><button class="btn btn-sm btn-danger" @click="remove(u)"><Trash2 :size="13"/>删除</button></div></td></tr><tr v-if="!users.length"><td colspan="8" class="empty">{{error||'暂无用户数据'}}</td></tr></tbody></table></div>
    </section>
    <dialog ref="dialog" class="detail-dialog" @click.self="dialog?.close()">
      <article v-if="selected" class="detail-card">
        <header class="detail-head">
          <div class="detail-identity">
            <span class="detail-avatar">{{String(selected.username||'U').slice(0,1).toUpperCase()}}</span>
            <div><h2>{{selected.username||`User #${selected.id}`}}</h2><p>用户 ID：{{selected.id}}</p></div>
          </div>
          <div class="detail-head-actions">
            <StatusBadge :tone="selected.role==='super_admin'?'primary':'neutral'">{{roleLabel(selected.role)}}</StatusBadge>
            <StatusBadge dot :tone="selected.status==='online'?'success':selected.status==='banned'?'danger':'neutral'">{{statusLabel(selected.status)}}</StatusBadge>
            <button class="dialog-close" aria-label="关闭" @click="dialog?.close()">×</button>
          </div>
        </header>

        <div class="detail-body">
          <section class="metric-grid">
            <div><span>加入群聊</span><strong>{{fmt(roomCount(selected))}}</strong></div>
            <div><span>发送消息</span><strong>{{fmt(messageCount(selected))}}</strong></div>
            <div><span>违规记录</span><strong :class="violationCount(selected)?'risk':''">{{fmt(violationCount(selected))}}</strong></div>
          </section>

          <section class="detail-section">
            <h3>账号信息</h3>
            <dl class="info-grid">
              <div><dt>账号状态</dt><dd>{{statusLabel(selected.status)}}</dd></div>
              <div><dt>用户角色</dt><dd>{{roleLabel(selected.role)}}</dd></div>
              <div><dt>注册时间</dt><dd>{{formatTime(selected.created_at)}}</dd></div>
              <div><dt>最后登录</dt><dd>{{formatTime(selected.last_login_at)}}</dd></div>
              <div><dt>最后活跃</dt><dd>{{formatTime(selected.last_active_at||selected.last_active)}}</dd></div>
              <div><dt>在线状态</dt><dd>{{selected.online||selected.status==='online'?'当前在线':'当前离线'}}</dd></div>
            </dl>
          </section>

          <section class="detail-section">
            <div class="section-title"><h3>所在群聊</h3><span>{{roomCount(selected)}} 个</span></div>
            <div v-if="Array.isArray(selected.rooms)&&selected.rooms.length" class="room-list">
              <div v-for="room in selected.rooms" :key="room.room_id" class="room-item">
                <div><b>{{room.room_name||`群聊 #${room.room_id}`}}</b><small>#{{room.room_id}}</small></div>
                <StatusBadge tone="neutral">{{roomRoleLabel(room.role)}}</StatusBadge>
              </div>
            </div>
            <div v-else class="detail-empty">暂无群聊记录</div>
          </section>

          <section v-if="Array.isArray(selected.violations)&&selected.violations.length" class="detail-section">
            <div class="section-title"><h3>最近违规</h3><span>显示最近 {{Math.min(selected.violations.length,3)}} 条</span></div>
            <div class="violation-list">
              <div v-for="item in selected.violations.slice(0,3)" :key="item.id" class="violation-item">
                <div><b>{{item.category||item.risk_level||'违规事件'}}</b><small>{{formatTime(item.created_at)}}</small></div>
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
import{onMounted,ref}from'vue';import{Search,Download,Trash2}from'lucide-vue-next';import StatusBadge from'../../components/common/StatusBadge.vue';import{adminApi}from'../../api/admin.js';
const users=ref([]),q=ref(''),status=ref(''),error=ref(''),total=ref(0),dialog=ref(),selected=ref(null);const fmt=v=>v==null?'—':new Intl.NumberFormat('zh-CN',{notation:Number(v)>9999?'compact':'standard'}).format(v);const formatTime=v=>v?new Date(v).toLocaleString('zh-CN'):'—';
const roomCount=u=>u?.room_count??(Array.isArray(u?.rooms)?u.rooms.length:u?.rooms??0);const messageCount=u=>u?.message_count??u?.messages??0;const violationCount=u=>u?.violation_count??(Array.isArray(u?.violations)?u.violations.length:u?.violations??0);
const roleLabel=role=>({super_admin:'超级管理员',moderator:'审核员',operator:'运营人员',user:'普通用户'})[role]||role||'普通用户';const statusLabel=value=>({online:'在线',offline:'离线',banned:'已封禁'})[value]||value||'离线';const roomRoleLabel=role=>({1:'成员',2:'管理员',3:'群主'})[String(role)]||'成员';
async function load(){error.value='';try{const d=await adminApi.users({q:q.value,status:status.value,page_size:100});users.value=Array.isArray(d)?d:(d?.items||d?.users||[]);total.value=d?.total??users.value.length}catch(e){users.value=[];error.value=e.message}}
async function detail(u){try{selected.value=await adminApi.user(u.id)}catch{selected.value=u}dialog.value?.showModal()}
async function manage(u){const action=prompt('操作：ban / unban / force_offline / role_user / role_operator / role_moderator / role_super_admin',u.status==='banned'?'unban':'force_offline');if(!action)return;if(!confirm(`对 ${u.username||u.id} 执行 ${action}？`))return;try{await adminApi.userAction(u.id,action.trim());await load()}catch(e){alert(e.message)}}
async function remove(u){if(!confirm(`确定删除用户 ${u.username||u.id}？该操作不可恢复。`))return;try{await adminApi.deleteUser(u.id);await load()}catch(e){alert(e.message)}}
function exportCsv(){const rows=[['id','username','role','status','rooms','messages','violations','last_active'],...users.value.map(u=>[u.id,u.username,u.role,u.status,u.rooms,u.messages,u.violations,u.last_active])];const csv='\ufeff'+rows.map(r=>r.map(x=>`"${String(x??'').replaceAll('"','""')}"`).join(',')).join('\n');const url=URL.createObjectURL(new Blob([csv],{type:'text/csv'}));const a=document.createElement('a');a.href=url;a.download='lan-im-users.csv';a.click();URL.revokeObjectURL(url)}onMounted(load)
</script>
<style scoped>.page{display:grid;gap:16px}.page-head{display:flex;justify-content:space-between;align-items:flex-start}.filters{padding:14px;display:flex;gap:10px;align-items:center;border-bottom:1px solid var(--line)}.search{height:38px;min-width:280px;display:flex;align-items:center;gap:8px;padding:0 11px;border:1px solid var(--line-strong);border-radius:9px;color:var(--text-3)}.search input{border:0;outline:0;background:transparent;color:var(--text);width:100%}.select{width:150px}.user{display:flex;align-items:center;gap:9px}.user>span{width:34px;height:34px;border-radius:10px;background:var(--primary-soft);color:var(--primary);display:grid;place-items:center;font-weight:800}.user b,.user small{display:block}.user small{font-size:10px;color:var(--text-3);margin-top:2px}.actions{display:flex;gap:6px;justify-content:flex-end}.risk{color:var(--danger)}.empty{text-align:center!important;color:var(--text-3);padding:30px!important}.detail-dialog{width:min(680px,calc(100vw - 28px));max-height:min(760px,calc(100vh - 40px));border:1px solid var(--line);border-radius:16px;background:var(--surface);color:var(--text);padding:0;box-shadow:var(--shadow-lg);overflow:hidden}.detail-dialog::backdrop{background:rgba(10,18,32,.52);backdrop-filter:blur(2px)}.detail-card{display:flex;flex-direction:column;max-height:inherit}.detail-head{display:flex;justify-content:space-between;align-items:center;gap:18px;padding:18px 20px;border-bottom:1px solid var(--line);background:var(--surface)}.detail-identity{display:flex;align-items:center;gap:12px;min-width:0}.detail-avatar{width:44px;height:44px;flex:0 0 auto;border-radius:13px;background:var(--primary-soft);color:var(--primary);display:grid;place-items:center;font-size:18px;font-weight:800}.detail-identity h2{font-size:17px;margin:0;white-space:nowrap;overflow:hidden;text-overflow:ellipsis}.detail-identity p{margin:4px 0 0;color:var(--text-3);font-size:11px}.detail-head-actions{display:flex;align-items:center;gap:7px}.dialog-close{width:30px;height:30px;border:0;border-radius:8px;background:var(--surface-soft);color:var(--text-2);font-size:20px;line-height:1;cursor:pointer}.dialog-close:hover{background:var(--danger-soft);color:var(--danger)}.detail-body{padding:18px 20px 22px;overflow:auto}.metric-grid{display:grid;grid-template-columns:repeat(3,1fr);gap:10px}.metric-grid>div{padding:13px 14px;border:1px solid var(--line);border-radius:11px;background:var(--surface-soft)}.metric-grid span{display:block;color:var(--text-3);font-size:11px}.metric-grid strong{display:block;margin-top:5px;font-size:20px}.detail-section{margin-top:20px}.detail-section h3{font-size:12px;margin:0 0 10px}.section-title{display:flex;align-items:center;justify-content:space-between}.section-title span{color:var(--text-3);font-size:10px}.info-grid{display:grid;grid-template-columns:repeat(2,1fr);margin:0;border:1px solid var(--line);border-radius:11px;overflow:hidden}.info-grid>div{padding:11px 13px;border-bottom:1px solid var(--line)}.info-grid>div:nth-child(odd){border-right:1px solid var(--line)}.info-grid>div:nth-last-child(-n+2){border-bottom:0}.info-grid dt{color:var(--text-3);font-size:10px}.info-grid dd{margin:4px 0 0;font-size:12px;font-weight:600}.room-list,.violation-list{display:grid;gap:7px}.room-item{display:flex;align-items:center;justify-content:space-between;padding:10px 12px;border:1px solid var(--line);border-radius:10px}.room-item b,.room-item small{display:block}.room-item b{font-size:12px}.room-item small,.violation-item small{margin-top:3px;color:var(--text-3);font-size:10px}.violation-item{padding:11px 12px;border-left:3px solid var(--danger);border-radius:8px;background:var(--danger-soft)}.violation-item>div{display:flex;justify-content:space-between;gap:12px}.violation-item b{font-size:11px}.violation-item p{margin:6px 0 0;color:var(--text-2);font-size:11px;line-height:1.55;display:-webkit-box;-webkit-line-clamp:2;-webkit-box-orient:vertical;overflow:hidden}.detail-empty{padding:20px;border:1px dashed var(--line-strong);border-radius:10px;text-align:center;color:var(--text-3);font-size:11px}@media(max-width:720px){.filters{align-items:stretch;flex-direction:column}.search{min-width:0}.select{width:100%}.detail-head{align-items:flex-start}.detail-head-actions>.status-badge{display:none}.metric-grid{grid-template-columns:1fr}.info-grid{grid-template-columns:1fr}.info-grid>div,.info-grid>div:nth-child(odd),.info-grid>div:nth-last-child(-n+2){border-right:0;border-bottom:1px solid var(--line)}.info-grid>div:last-child{border-bottom:0}}</style>
