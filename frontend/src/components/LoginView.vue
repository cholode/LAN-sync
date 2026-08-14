<template>
  <div id="login-view" class="view-container">
    <div class="login-box">
      <h2>LAN-IM 系统登录</h2>
      <input v-model="username" type="text" placeholder="用户名" autocomplete="off" />
      <input v-model="password" type="password" placeholder="密码" />
      <button class="btn-login" type="button" @click="submit('login')">登录</button>
      <button class="btn-reg" type="button" @click="submit('register')">注册</button>
      <div id="msg">{{ message }}</div>
    </div>
  </div>
</template>

<script setup>
import { ref } from 'vue';
import { auth } from '../composables/useAuth.js';

const username = ref('');
const password = ref('');
const message = ref('');

async function submit(action) {
  const result = await auth(action, username.value, password.value);
  message.value = result.message;
  if (result.ok && action === 'login') {
    message.value = '登录成功，正在进入...';
  }
}
</script>
