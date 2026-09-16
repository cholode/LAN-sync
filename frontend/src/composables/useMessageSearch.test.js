import test from 'node:test'
import assert from 'node:assert/strict'
import { ref } from 'vue'
import { useMessageSearch } from './useMessageSearch.js'

test('分页使用已提交关键词和服务端总数，输入修改不污染后续页', async () => {
  const calls = []
  const state = useMessageSearch(ref({ id: '7' }), async (id, params) => {
    calls.push({ id, ...params })
    return { items: [{ id: String(params.from + 1), content: '群聊消息' }], total: 2 }
  })
  state.keyword.value = ' 消息 '
  await state.search()
  assert.equal(state.total.value, 2)
  assert.equal(state.hasMore.value, true)
  state.keyword.value = '其他词'
  await state.search(true)
  assert.deepEqual(calls, [{ id: '7', q: '消息', from: 0, size: 20 }, { id: '7', q: '消息', from: 1, size: 20 }])
  assert.equal(state.results.value.length, 2)
  assert.equal(state.hasMore.value, false)
})

test('退出搜索后晚到的响应不能重新打开搜索', async () => {
  let resolve
  const state = useMessageSearch(ref({ id: '1' }), () => new Promise((done) => { resolve = done }))
  state.keyword.value = '测试'
  const pending = state.search()
  state.clear()
  resolve({ items: [{ id: '1' }], total: 1 })
  await pending
  assert.equal(state.active.value, false)
  assert.deepEqual(state.results.value, [])
})

test('切换群聊后旧请求不会覆盖新群结果或解除新请求的加载状态', async () => {
  const room = ref({ id: '1' })
  const pending = []
  const state = useMessageSearch(room, () => new Promise((done) => pending.push(done)))
  state.keyword.value = '旧群'
  const old = state.search()
  room.value = { id: '2' }
  state.clear()
  state.keyword.value = '新群'
  const fresh = state.search()
  pending[0]({ items: [{ id: 'old' }], total: 1 })
  await old
  assert.equal(state.loading.value, true)
  assert.deepEqual(state.results.value, [])
  pending[1]({ items: [{ id: 'new' }], total: 1 })
  await fresh
  assert.equal(state.results.value[0].id, 'new')
  assert.equal(state.loading.value, false)
})

test('接口错误可重试，空白关键词不发送请求', async () => {
  let count = 0
  const state = useMessageSearch(ref({ id: '1' }), async () => {
    if (++count === 1) throw new Error('搜索服务不可用')
    return { items: [], total: 0 }
  })
  state.keyword.value = '  '
  await state.search()
  assert.equal(count, 0)
  state.keyword.value = '测试'
  await state.search()
  assert.equal(state.error.value, '搜索服务不可用')
  assert.equal(state.loading.value, false)
  await state.search()
  assert.equal(state.error.value, '')
  assert.equal(state.active.value, true)
})
