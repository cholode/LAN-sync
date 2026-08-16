<template>
  <div class="logo"><div class="mark"><RadioTower :size="19" /></div><div v-if="!compact"><b>LAN IM</b><span>Realtime Console</span></div></div>
</template>
<script setup>import { RadioTower } from 'lucide-vue-next'; defineProps({ compact: Boolean })</script>
<style scoped>
.logo{display:flex;align-items:center;gap:11px}.mark{width:34px;height:34px;border-radius:10px;background:linear-gradient(145deg,#3159e8,#6f85f5);color:#fff;display:grid;place-items:center;box-shadow:0 7px 16px rgba(49,89,232,.22)}b{font-size:14px;letter-spacing:.02em}span{display:block;font-size:10px;color:var(--text-3);margin-top:2px;text-transform:uppercase;letter-spacing:.08em}
</style>
