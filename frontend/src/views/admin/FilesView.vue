<template>
  <div class="page">
    <div class="page-head"><div><h1 class="page-title">文件管理</h1><p class="page-subtitle">对象存储记录、异常扫描与孤立对象清理</p></div><div class="actions"><button class="btn" @click="scan">异常扫描</button><button class="btn btn-danger" @click="cleanup">清理孤立对象</button></div></div>
    <div v-if="notice" class="notice">{{notice}}</div>
    <section class="app-card"><div class="filters"><div class="search"><Search :size="15"/><input v-model="q" placeholder="文件名 / Object Key" @keyup.enter="load"/></div><select v-model="status" class="select" @change="load"><option value="">全部状态</option><option value="uploaded">uploaded</option><option value="deleted">deleted</option></select><button class="btn" @click="load">查询</button><StatusBadge :tone="error?'danger':'success'" dot>{{error||`共 ${total} 个文件`}}</StatusBadge></div>
      <div class="table-wrap"><table class="data-table"><thead><tr><th>文件</th><th>上传者 / 房间</th><th>大小</th><th>存储</th><th>状态</th><th>创建时间</th><th></th></tr></thead><tbody>
        <tr v-for="file in files" :key="file.id"><td><b>{{file.original_name||file.object_key}}</b><small>{{file.object_key}}</small></td><td>User #{{file.uploader_id}}<small>Room #{{file.room_id||'—'}}</small></td><td>{{fmtSize(file.size)}}</td><td>{{file.backend||'—'}}</td><td><StatusBadge :tone="file.status==='uploaded'?'success':'neutral'">{{file.status}}</StatusBadge></td><td>{{fmtTime(file.created_at)}}</td><td><div class="actions"><button class="btn btn-sm" @click="download(file)">下载</button><button class="btn btn-sm btn-danger" @click="remove(file)">删除</button></div></td></tr>
        <tr v-if="!files.length"><td colspan="7" class="empty">{{error||'暂无文件记录'}}</td></tr>
      </tbody></table></div>
    </section>
  </div>
</template>
<script setup>
import{onMounted,ref}from'vue';import{Search}from'lucide-vue-next';import StatusBadge from'../../components/common/StatusBadge.vue';import{adminApi}from'../../api/admin.js';
const files=ref([]),total=ref(0),q=ref(''),status=ref(''),error=ref(''),notice=ref('');const fmtTime=v=>v?new Date(v).toLocaleString('zh-CN'):'—';const fmtSize=v=>{const n=Number(v)||0;if(n<1024)return`${n} B`;if(n<1048576)return`${(n/1024).toFixed(1)} KB`;return`${(n/1048576).toFixed(1)} MB`};
async function load(){error.value='';try{const d=await adminApi.files({q:q.value,status:status.value});files.value=d?.items||[];total.value=d?.total??files.value.length}catch(e){files.value=[];error.value=e.message}}
async function download(file){try{const d=await adminApi.fileDownload(file.id);if(d?.download_url)window.open(d.download_url,'_blank','noopener')}catch(e){alert(e.message)}}
async function remove(file){if(!confirm(`永久删除文件「${file.original_name||file.id}」？`))return;try{await adminApi.deleteFile(file.id);await load()}catch(e){alert(e.message)}}
async function scan(){try{const d=await adminApi.scanFiles();notice.value=`扫描完成：${JSON.stringify(d)}`}catch(e){notice.value=e.message}}
async function cleanup(){if(!confirm('只会清理后端判定为孤立的对象，是否继续？'))return;try{const d=await adminApi.cleanupFiles();notice.value=`已清理 ${d?.cleaned??0} 个孤立对象`;await load()}catch(e){notice.value=e.message}}
onMounted(load)
</script>
<style scoped>.page{display:grid;gap:16px}.page-head,.actions{display:flex;justify-content:space-between;gap:8px}.notice{padding:10px 12px;border-radius:9px;background:var(--primary-soft);color:var(--primary);font-size:11px}.filters{padding:14px;display:flex;gap:10px;align-items:center}.search{height:38px;width:320px;display:flex;align-items:center;gap:8px;padding:0 11px;border:1px solid var(--line-strong);border-radius:9px}.search input{border:0;outline:0;background:transparent;color:var(--text);width:100%}.select{width:150px}td small{display:block;max-width:300px;overflow:hidden;text-overflow:ellipsis;color:var(--text-3);font-size:10px;margin-top:3px}.empty{text-align:center!important;color:var(--text-3);padding:30px!important}</style>
