// perf6 准备：注册 + 登录，批量获取 JWT token（不发消息，仅用于后续 WS 建连压测）
// 用法: node seed_users.mjs   （环境变量 PERF_USERS / PERF_SEED_CONCURRENCY / PERF_BASE 可覆盖）
// 幂等：已存在的用户 register 返回 409 后直接 login 刷新 token；支持断点续跑（读现有 users.json）
import fs from 'node:fs'
import path from 'node:path'
import { fileURLToPath } from 'node:url'

const BASE = process.env.PERF_BASE || 'http://8.130.151.211/api/v1'
const PASSWORD = process.env.PERF_PASSWORD || 'Perf@12345'
const TOTAL_USERS = Number(process.env.PERF_USERS || 10000)
const CONCURRENCY = Number(process.env.PERF_SEED_CONCURRENCY || 20)
const DIR = path.dirname(fileURLToPath(import.meta.url))
const USERS_FILE = path.join(DIR, 'users.json')

function loadJSON(file, fallback) {
  try {
    return JSON.parse(fs.readFileSync(file, 'utf8'))
  } catch {
    return fallback
  }
}

async function request(pathname, { method = 'GET', token, body } = {}) {
  const headers = { 'Content-Type': 'application/json' }
  if (token) headers.Authorization = `Bearer ${token}`
  const res = await fetch(`${BASE}${pathname}`, {
    method,
    headers,
    body: body === undefined ? undefined : JSON.stringify(body),
  })
  const text = await res.text()
  let data = null
  if (text) {
    try { data = JSON.parse(text) } catch { data = text }
  }
  return { status: res.status, data }
}

async function withRetry(fn, label, tries = 3) {
  for (let i = 1; i <= tries; i++) {
    try {
      return await fn()
    } catch (err) {
      if (i === tries) throw err
      await new Promise((r) => setTimeout(r, 500 * i))
      console.warn(`retry ${label} (${i}/${tries}): ${err.message}`)
    }
  }
}

async function registerOrLogin(username) {
  const body = { username, password: PASSWORD }
  await withRetry(async () => {
    const reg = await request('/register', { method: 'POST', body })
    if (reg.status !== 200 && reg.status !== 409) {
      throw new Error(`register ${username} -> HTTP ${reg.status}`)
    }
  }, `register ${username}`)

  const login = await withRetry(async () => {
    const res = await request('/login', { method: 'POST', body })
    if (res.status !== 200 || !res.data?.token) {
      throw new Error(`login ${username} -> HTTP ${res.status}`)
    }
    return res
  }, `login ${username}`)

  return {
    username,
    password: PASSWORD,
    token: login.data.token,
    user_id: login.data.user?.id ?? null,
  }
}

async function pool(items, worker, limit) {
  const results = new Array(items.length)
  let cursor = 0
  async function run() {
    while (true) {
      const index = cursor++
      if (index >= items.length) return
      results[index] = await worker(items[index], index)
    }
  }
  await Promise.all(Array.from({ length: Math.min(limit, items.length) }, run))
  return results
}

async function main() {
  const existing = new Map(loadJSON(USERS_FILE, []).map((u) => [u.username, u]))
  const missing = []
  for (let i = 0; i < TOTAL_USERS; i++) {
    const username = `perf_u_${String(i + 1).padStart(4, '0')}`
    if (!existing.has(username)) missing.push(username)
  }
  console.log(`target=${TOTAL_USERS} existing=${existing.size} to_create=${missing.length}`)

  const batch = 50
  for (let start = 0; start < missing.length; start += batch) {
    const chunk = missing.slice(start, start + batch)
    const created = await pool(chunk, registerOrLogin, CONCURRENCY)
    for (const item of created) if (item) existing.set(item.username, item)
    fs.writeFileSync(USERS_FILE, JSON.stringify(Array.from(existing.values()), null, 2), 'utf8')
    console.log(`seeded ${Math.min(start + batch, missing.length)}/${missing.length} (total=${existing.size})`)
  }

  // 已存在的用户也全部重新登录，刷新 token（JWT 24h 有效）
  console.log('refreshing tokens for all existing users...')
  const allUsers = Array.from(existing.values())
  const refreshed = await pool(allUsers, async (user) => {
    const login = await withRetry(async () => {
      const res = await request('/login', { method: 'POST', body: { username: user.username, password: PASSWORD } })
      if (res.status !== 200 || !res.data?.token) throw new Error(`relogin ${user.username} -> HTTP ${res.status}`)
      return res
    }, `relogin ${user.username}`)
    return { ...user, token: login.data.token }
  }, CONCURRENCY)
  fs.writeFileSync(USERS_FILE, JSON.stringify(refreshed, null, 2), 'utf8')
  console.log(`seed complete: ${refreshed.length} users with fresh tokens -> ${USERS_FILE}`)
}

main().catch((err) => {
  console.error(err)
  process.exit(1)
})
