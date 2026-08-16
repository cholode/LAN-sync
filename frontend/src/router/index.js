import { createRouter, createWebHistory } from 'vue-router'
import { useAuthStore } from '../stores/auth.js'

const router = createRouter({
  history: createWebHistory(),
  routes: [
    { path: '/login', component: () => import('../views/LoginView.vue') },
    { path: '/register', component: () => import('../views/RegisterView.vue') },
    { path: '/chat', component: () => import('../views/ChatView.vue'), meta: { auth: true } },
    {
      path: '/admin', component: () => import('../layouts/AdminLayout.vue'), meta: { auth: true, admin: true },
      children: [
        { path: '', redirect: '/admin/dashboard' },
        { path: 'dashboard', component: () => import('../views/admin/DashboardView.vue') },
        { path: 'users', component: () => import('../views/admin/UsersView.vue') },
        { path: 'rooms', component: () => import('../views/admin/RoomsView.vue') },
        { path: 'moderation', component: () => import('../views/admin/ModerationView.vue') },
        { path: 'agent', component: () => import('../views/admin/AgentOpsView.vue') },
        { path: 'system', component: () => import('../views/admin/SystemView.vue') },
        { path: 'audit', component: () => import('../views/admin/AuditView.vue') },
      ],
    },
    { path: '/', redirect: '/chat' },
    { path: '/:pathMatch(.*)*', component: () => import('../views/NotFoundView.vue') },
  ],
})

router.beforeEach((to) => {
  const auth = useAuthStore()
  if (to.meta.auth && !auth.isLoggedIn) return { path: '/login', query: { redirect: to.fullPath } }
  if (to.meta.admin && !auth.isSuperAdmin) return '/chat'
  if (to.path === '/login' && auth.isLoggedIn) return auth.isSuperAdmin ? '/admin/dashboard' : '/chat'
})

export default router
