import { computed, ref } from 'vue'

export function useMessageSearch(current, searchMessages, enrich = (message) => message) {
  const keyword = ref('')
  const loading = ref(false)
  const active = ref(false)
  const results = ref([])
  const total = ref(0)
  const error = ref('')
  let version = 0
  let submitted = ''
  const exhausted = ref(false)
  const hasMore = computed(() => !exhausted.value && results.value.length < total.value)

  function clear() {
    version++
    keyword.value = ''
    loading.value = false
    active.value = false
    results.value = []
    total.value = 0
    error.value = ''
    submitted = ''
    exhausted.value = false
  }

  async function search(more = false) {
    const roomId = current.value?.id
    const q = more ? submitted : keyword.value.trim()
    if (!roomId || !q || loading.value || (more && !hasMore.value)) return
    const request = ++version
    if (!more) {
      results.value = []
      total.value = 0
      submitted = q
      exhausted.value = false
    }
    active.value = true
    loading.value = true
    error.value = ''
    try {
      const page = await searchMessages(roomId, { q, from: results.value.length, size: 20 })
      if (request !== version || current.value?.id !== roomId) return
      results.value = [...results.value, ...page.items.map(enrich)]
      total.value = page.total
      exhausted.value = page.items.length === 0
    } catch (e) {
      if (request === version && current.value?.id === roomId) error.value = e.message || '聊天记录搜索失败，请重试'
    } finally {
      if (request === version) loading.value = false
    }
  }

  return { keyword, loading, active, results, total, error, hasMore, search, clear }
}
