import { createRouter, createWebHistory } from 'vue-router'
import { useAuthStore } from '../stores/auth.js'

const router = createRouter({
  history: createWebHistory(),
  routes: [
    { path: '/login', component: () => import('../views/admin/AdminLoginView.vue') },
    {
	  path: '/admin', component: () => import('../layouts/AdminLayout.vue'), meta: { auth: true },
      children: [
		{ path: '', redirect: '/admin/dashboard' },
        { path: 'dashboard', component: () => import('../views/admin/DashboardView.vue') },
        { path: 'users', component: () => import('../views/admin/UsersView.vue') },
        { path: 'rooms', component: () => import('../views/admin/RoomsView.vue') },
        { path: 'moderation', component: () => import('../views/admin/ModerationView.vue') },
        { path: 'agent', component: () => import('../views/admin/AgentOpsView.vue') },
        { path: 'connections', component: () => import('../views/admin/ConnectionsView.vue') },
        { path: 'files', component: () => import('../views/admin/FilesView.vue') },
        { path: 'operations', component: () => import('../views/admin/OperationsView.vue') },
        { path: 'system', component: () => import('../views/admin/SystemView.vue') },
        { path: 'audit', component: () => import('../views/admin/AuditView.vue') },
      ],
    },
	{ path: '/', redirect: '/admin/dashboard' },
	{ path: '/:pathMatch(.*)*', redirect: '/admin/dashboard' },
  ],
})

router.beforeEach((to) => {
  const auth = useAuthStore()
  if (to.meta.auth && (!auth.isLoggedIn || !auth.isAdmin)) return '/login'
	if (to.path === '/login' && auth.isLoggedIn && auth.isAdmin) return '/admin/dashboard'
})

export default router
