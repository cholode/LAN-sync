<template>
  <div class="admin-shell">
    <aside :class="{collapsed:ui.sidebarCollapsed}">
      <div class="brand"><AppLogo :compact="ui.sidebarCollapsed" /></div>
      <nav>
        <div class="nav-group">运营控制台</div>
        <RouterLink v-for="item in primary" :key="item.to" :to="item.to" class="nav-item"><component :is="item.icon" :size="17" /><span>{{ item.label }}</span></RouterLink>
        <div class="nav-group">智能治理</div>
        <RouterLink v-for="item in ai" :key="item.to" :to="item.to" class="nav-item"><component :is="item.icon" :size="17" /><span>{{ item.label }}</span></RouterLink>
        <div class="nav-group">平台与安全</div>
        <RouterLink v-for="item in ops" :key="item.to" :to="item.to" class="nav-item"><component :is="item.icon" :size="17" /><span>{{ item.label }}</span></RouterLink>
      </nav>
      <div class="sidebar-foot"><RouterLink to="/chat" class="nav-item"><MessageSquareText :size="17" /><span>返回聊天</span></RouterLink></div>
    </aside>
    <div class="main" :class="{collapsed:ui.sidebarCollapsed}">
      <header class="topbar">
        <div class="top-left"><button class="icon-btn" @click="ui.sidebarCollapsed=!ui.sidebarCollapsed"><PanelLeftClose v-if="!ui.sidebarCollapsed" :size="18"/><PanelLeftOpen v-else :size="18"/></button><div class="global-search"><Search :size="15"/><input placeholder="搜索用户、群聊、事件…"/><kbd>⌘ K</kbd></div></div>
        <div class="top-actions"><StatusBadge tone="success" dot>服务正常</StatusBadge><button class="icon-btn" @click="ui.toggleTheme"><Moon v-if="ui.theme==='light'" :size="17"/><Sun v-else :size="17"/></button><RouterLink to="/admin/operations" class="icon-btn"><Bell :size="17"/><span v-if="alertCount" class="notice">{{alertCount>99?'99+':alertCount}}</span></RouterLink><div class="avatar">{{ initial }}</div></div>
      </header>
      <main class="content"><RouterView /></main>
    </div>
  </div>
</template>
<script setup>
import { computed, onMounted, ref } from 'vue'
import { Users, LayoutDashboard, MessagesSquare, ShieldCheck, Bot, Activity, ScrollText, MessageSquareText, PanelLeftClose, PanelLeftOpen, Search, Moon, Sun, Bell, Network, FolderOpen, Siren } from 'lucide-vue-next'
import AppLogo from '../components/common/AppLogo.vue'; import StatusBadge from '../components/common/StatusBadge.vue'; import { useUiStore } from '../stores/ui.js'; import { useAuthStore } from '../stores/auth.js'; import { adminApi } from '../api/admin.js'
const ui=useUiStore(), auth=useAuthStore(), alertCount=ref(0); const initial=computed(()=>String(auth.user?.username||'A').slice(0,1).toUpperCase());onMounted(async()=>{try{alertCount.value=Number((await adminApi.unresolvedAlertCount())?.count)||0}catch{alertCount.value=0}})
const primary=[{to:'/admin/dashboard',label:'总览 Dashboard',icon:LayoutDashboard},{to:'/admin/users',label:'用户管理',icon:Users},{to:'/admin/rooms',label:'群聊管理',icon:MessagesSquare}]
const ai=[{to:'/admin/moderation',label:'内容治理',icon:ShieldCheck},{to:'/admin/agent',label:'Agent & RAG',icon:Bot}]
const ops=[{to:'/admin/connections',label:'连接管理',icon:Network},{to:'/admin/files',label:'文件管理',icon:FolderOpen},{to:'/admin/operations',label:'运维事件',icon:Siren},{to:'/admin/system',label:'系统运行',icon:Activity},{to:'/admin/audit',label:'审计日志',icon:ScrollText}]
</script>
<style scoped>
.admin-shell{min-height:100vh;background:var(--bg)}aside{position:fixed;inset:0 auto 0 0;width:var(--sidebar);background:var(--surface);border-right:1px solid var(--line);z-index:20;display:flex;flex-direction:column;transition:width .2s ease}.brand{height:var(--topbar);display:flex;align-items:center;padding:0 18px;border-bottom:1px solid var(--line)}nav{padding:14px 10px;overflow:auto;flex:1}.nav-group{padding:14px 10px 7px;color:var(--text-3);font-size:10px;font-weight:700;letter-spacing:.08em;text-transform:uppercase}.nav-item{height:40px;padding:0 10px;border-radius:9px;display:flex;align-items:center;gap:10px;color:var(--text-2);margin:2px 0;white-space:nowrap;overflow:hidden}.nav-item:hover{background:var(--surface-soft);color:var(--text)}.nav-item.router-link-active{background:var(--primary-soft);color:var(--primary);font-weight:650}.sidebar-foot{padding:10px;border-top:1px solid var(--line)}aside.collapsed{width:72px}aside.collapsed .brand{padding:0 19px}aside.collapsed .nav-group,aside.collapsed .nav-item span{display:none}aside.collapsed .nav-item{justify-content:center}.main{margin-left:var(--sidebar);transition:margin .2s ease}.main.collapsed{margin-left:72px}.topbar{height:var(--topbar);position:sticky;top:0;z-index:15;background:color-mix(in srgb,var(--surface) 92%,transparent);backdrop-filter:blur(12px);border-bottom:1px solid var(--line);display:flex;align-items:center;justify-content:space-between;padding:0 22px}.top-left,.top-actions{display:flex;align-items:center;gap:10px}.icon-btn{width:36px;height:36px;border:1px solid var(--line);border-radius:9px;background:var(--surface);color:var(--text-2);display:grid;place-items:center;position:relative}.notice{position:absolute;right:8px;top:7px;width:6px;height:6px;border-radius:50%;background:var(--danger);border:1px solid var(--surface)}.global-search{height:36px;width:300px;border:1px solid var(--line);border-radius:9px;background:var(--surface-soft);display:flex;align-items:center;gap:8px;padding:0 10px;color:var(--text-3)}.global-search input{min-width:0;flex:1;border:0;outline:0;background:transparent;color:var(--text)}kbd{font-size:10px;border:1px solid var(--line-strong);border-radius:5px;padding:2px 5px;background:var(--surface)}.avatar{width:34px;height:34px;border-radius:10px;background:var(--primary);color:#fff;display:grid;place-items:center;font-weight:700}.content{padding:24px;max-width:1680px;margin:0 auto}@media(max-width:820px){aside{transform:translateX(-100%)}.main,.main.collapsed{margin-left:0}.global-search{display:none}.content{padding:16px}}
.notice{right:-5px;top:-5px;min-width:17px;width:auto;height:17px;padding:0 4px;border-radius:9px;color:#fff;font-size:8px;display:grid;place-items:center}
</style>
