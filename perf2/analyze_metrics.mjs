import fs from 'node:fs'
import path from 'node:path'

const dir = process.cwd()
const metricsDir = path.join(dir, 'metrics')
const prefixes = [
  'http_05vu', 'http_10vu', 'http_20vu', 'http_50vu',
  'wsconn_10vu', 'wsconn_50vu', 'wsconn_100vu', 'wsconn_200vu', 'wsconn_300vu', 'wsconn_400vu', 'wsconn_500vu',
  'wsmsg_50rps', 'wsmsg_100rps', 'wsmsg_200rps', 'wsmsg_500rps',
  'broadcast_10', 'broadcast_100', 'broadcast_500', 'broadcast_1000', 'broadcast_1000b',
]
const gauges = new Set([
  'im_uptime_seconds', 'im_ws_connections_active', 'im_hub_clients_total', 'im_hub_rooms_total',
  'im_hub_task_pool_capacity', 'im_hub_task_pool_running', 'im_hub_task_pool_waiting',
  'im_kafka_consumer_lag', 'im_db_pool_idle_connections', 'im_db_pool_in_use_connections', 'im_db_pool_open_connections',
  'im_redis_pool_idle_connections', 'im_redis_pool_total_connections',
  'im_agent_inflight_requests', 'im_agent_rooms_enabled',
])
const counters = new Set([
  'im_ws_connections_total', 'im_ws_read_messages_total', 'im_ws_write_messages_total', 'im_ws_read_errors_total', 'im_ws_write_errors_total',
  'im_hub_dispatched_messages_total', 'im_db_query_total', 'im_db_query_errors_total', 'im_db_pool_wait_count_total',
  'im_kafka_consume_total', 'im_kafka_produce_total', 'im_redis_ops_total', 'im_redis_errors_total', 'im_redis_pubsub_total',
  'im_agent_messages_received_total', 'im_agent_messages_triggered_total', 'im_agent_processed_total',
])
const histograms = new Set([
  'im_db_query_duration_seconds', 'im_redis_latency_seconds', 'im_kafka_produce_latency_seconds',
  'im_kafka_consume_latency_seconds', 'im_hub_dispatch_latency_seconds', 'im_ws_connection_duration_seconds',
  'im_agent_reply_latency_seconds',
])

function parseProm(text) {
  const result = { gauges: {}, counters: {}, hist: {} }
  const buckets = {}
  for (const raw of text.split(/\r?\n/)) {
    const line = raw.trim()
    if (!line || line.startsWith('#')) continue
    const m = line.match(/^([a-zA-Z_:][a-zA-Z0-9_:]*)(?:\{([^}]*)\})?\s+([0-9.eE+NaInf-]+)/)
    if (!m) continue
    const name = m[1]
    const value = Number(m[3])
    if (!Number.isFinite(value)) continue
    if (name.endsWith('_bucket')) {
      const base = name.slice(0, -7)
      const le = m[2] ? Object.fromEntries(m[2].split(',').filter(Boolean).map(kv => { const i = kv.indexOf('='); return [kv.slice(0,i), kv.slice(i+1).replace(/^"|"$/g,'')] })) : {}
      const key = `${base}|${le.le || ''}`
      buckets[key] = value
      continue
    }
    if (name.endsWith('_sum') || name.endsWith('_count')) {
      const base = name.endsWith('_sum') ? name.slice(0, -4) : name.slice(0, -6)
      if (!result.hist[base]) result.hist[base] = { sum: 0, count: 0 }
      if (name.endsWith('_sum')) result.hist[base].sum += value
      else result.hist[base].count += value
      continue
    }
    if (gauges.has(name)) result.gauges[name] = (result.gauges[name] || 0) + value
    else if (counters.has(name)) result.counters[name] = (result.counters[name] || 0) + value
    else if (name.endsWith('_total')) result.counters[name] = (result.counters[name] || 0) + value
  }
  return result
}

function filesFor(prefix) {
  return fs.readdirSync(metricsDir)
    .filter(f => f.startsWith(prefix + '_') && f.endsWith('.txt'))
    .map(f => {
      const file = path.join(metricsDir, f)
      const ts = Number(f.match(/_(\d+)\.txt$/)?.[1] || 0)
      return { file, ts }
    })
    .sort((a, b) => a.ts - b.ts)
}

function stats(values) {
  const arr = values.slice().sort((a,b)=>a-b)
  const sum = values.reduce((a,b)=>a+b,0)
  return { avg: sum/values.length, min: arr[0], max: arr[arr.length-1], p95: arr[Math.min(arr.length-1, Math.floor(arr.length*0.95))] }
}

const out = []
for (const prefix of prefixes) {
  const files = filesFor(prefix)
  if (!files.length) continue
  const samples = []
  for (const {file, ts} of files) {
    const text = fs.readFileSync(file, 'utf8')
    const p = parseProm(text)
    samples.push({ ts, ...p })
  }
  const first = samples[0]
  const last = samples[samples.length-1]
  const duration = (last.ts - first.ts) / 1000
  const entry = { prefix, samples: samples.length, duration_sec: duration }
  for (const name of gauges) {
    const values = samples.map(s => s.gauges[name] || 0)
    entry[`gauge_${name}_max`] = Math.max(...values)
    entry[`gauge_${name}_avg`] = values.reduce((a,b)=>a+b,0)/values.length
    entry[`gauge_${name}_last`] = last.gauges[name] || 0
  }
  for (const name of counters) {
    const delta = (last.counters[name] || 0) - (first.counters[name] || 0)
    if (delta) entry[`counter_${name}_delta`] = delta
  }
  for (const name of histograms) {
    const dsum = (last.hist[name]?.sum || 0) - (first.hist[name]?.sum || 0)
    const dcount = (last.hist[name]?.count || 0) - (first.hist[name]?.count || 0)
    if (dcount > 0) entry[`hist_${name}_avg_ms`] = dsum / dcount * 1000
    if (dcount > 0) entry[`hist_${name}_count`] = dcount
  }
  out.push(entry)
}

fs.writeFileSync('metrics_summary.json', JSON.stringify(out, null, 2), 'utf8')
console.log(JSON.stringify(out, null, 2))


