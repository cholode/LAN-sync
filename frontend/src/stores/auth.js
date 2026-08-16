import { defineStore } from 'pinia'
import { computed, ref } from 'vue'
import { authApi } from '../api/auth.js'

function decodeJwt(token) {
  try {
    const p = token.split('.')[1].replace(/-/g, '+').replace(/_/g, '/')
    const json = decodeURIComponent(atob(p).split('').map(c => `%${('00' + c.charCodeAt(0).toString(16)).slice(-2)}`).join(''))
    return JSON.parse(json)
  } catch { return {} }
}

export const useAuthStore = defineStore('auth', () => {
  const token = ref(localStorage.getItem('lan_im_token') || '')
  const user = ref(JSON.parse(localStorage.getItem('lan_im_user') || 'null'))
  const payload = computed(() => token.value ? decodeJwt(token.value) : {})
  const role = computed(() => user.value?.role || payload.value?.role || payload.value?.user_role || '')
  const isSuperAdmin = computed(() => {
    const roleValue = String(role.value ?? '').toLowerCase()
    return Number(role.value) === 1 || ['1', 'super_admin', 'superadmin', 'admin'].includes(roleValue)
  })
  const isLoggedIn = computed(() => !!token.value)

  function persist(nextToken, nextUser) {
    token.value = nextToken || ''
    user.value = nextUser || null
    if (token.value) localStorage.setItem('lan_im_token', token.value); else localStorage.removeItem('lan_im_token')
    if (user.value) localStorage.setItem('lan_im_user', JSON.stringify(user.value)); else localStorage.removeItem('lan_im_user')
  }

  async function login(form) {
    const data = await authApi.login(form)
    const nextToken = data?.token || data?.access_token || data?.jwt || data?.data?.token
    if (!nextToken) throw new Error('登录成功但响应中没有找到 JWT token')
    persist(nextToken, data?.user || data?.data?.user || { username: form.username })
    return data
  }

  async function register(form) { return authApi.register(form) }
  function logout() { persist('', null) }
  return { token, user, role, isSuperAdmin, isLoggedIn, login, register, logout }
})
