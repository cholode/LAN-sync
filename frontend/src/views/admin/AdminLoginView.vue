<template>
  <div class="auth-page">
    <section class="form-card app-card">
      <AppLogo />
      <h1>管理后台</h1>
      <p>使用管理员账号登录</p>
      <form @submit.prevent="submit">
        <label>用户名</label>
        <input v-model="form.username" class="input" autocomplete="username" placeholder="请输入管理员用户名" />
        <label>密码</label>
        <input v-model="form.password" class="input" type="password" autocomplete="current-password" placeholder="请输入密码" />
        <div v-if="error" class="error">{{ error }}</div>
        <button class="btn btn-primary submit" :disabled="loading">{{ loading ? '登录中…' : '登录管理后台' }}</button>
      </form>
    </section>
  </div>
</template>

<script setup>
import { reactive, ref } from 'vue'
import { useRouter } from 'vue-router'
import AppLogo from '../../components/common/AppLogo.vue'
import { useAuthStore } from '../../stores/auth.js'

const form = reactive({ username: '', password: '' })
const error = ref('')
const loading = ref(false)
const auth = useAuthStore()
const router = useRouter()

async function submit() {
  error.value = ''
  loading.value = true
  try {
    await auth.adminLogin(form)
	await router.replace('/admin/dashboard')
  } catch (e) {
    error.value = e.message
  } finally {
    loading.value = false
  }
}
</script>

<style scoped>
.auth-page{min-height:100vh;display:grid;place-items:center;padding:28px;background:var(--bg)}
.form-card{width:min(420px,100%);padding:34px}.form-card h1{font-size:24px;margin:28px 0 4px}.form-card>p{color:var(--text-2);margin:0 0 26px}
form{display:grid;gap:10px}label{font-size:12px;font-weight:650;margin-top:4px}.submit{margin-top:10px;height:42px}
.error{padding:9px 10px;border-radius:8px;background:var(--danger-soft);color:var(--danger);font-size:12px}
</style>
