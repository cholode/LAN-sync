import { defineStore } from 'pinia'
import { ref, watch } from 'vue'

export const useUiStore = defineStore('ui', () => {
  const theme = ref(localStorage.getItem('lan_im_theme') || 'light')
  const sidebarCollapsed = ref(false)
  const apply = () => document.documentElement.setAttribute('data-theme', theme.value)
  watch(theme, () => { localStorage.setItem('lan_im_theme', theme.value); apply() }, { immediate: true })
  const toggleTheme = () => { theme.value = theme.value === 'dark' ? 'light' : 'dark' }
  return { theme, sidebarCollapsed, toggleTheme }
})
