// perf6: 按级别聚合服务端 /metrics 采样（读 perf6/metrics/<prefix>_*.txt）
// 用法: node analyze_metrics.mjs <prefix1> <prefix2> ...
import fs from 'node:fs'
import path from 'node:path'

const metricsDir = path.join(process.cwd(), 'metrics')
const prefixes = process.argv.slice(2)

const gauges = new Set([
  'im_ws_connections_active',
  'im_hub_clients_total',
  'im_hub_rooms_total',
  'im_hub_task_pool_capacity',
  'im_hub_task_pool_running',
  'im_hub_task_pool_waiting',
  'im_kafka_consumer_lag',
  'im_db_pool_idle_connections',
  'im_db_pool_in_use_connections',
  'im_db_pool_open_connections',
  'im_redis_pool_idle_connections',
  'im_redis_pool_total_connections',
  'im_agent_inflight_requests',
  'im_agent_rooms_enabled',
])

const counters = new Set([
  'im_ws_connections_total',
  'im_ws_read_messages_total',
  'im_ws_write_messages_total',
  'im_ws_read_errors_total',
  'im_ws_write_errors_total',
  'im_hub_dispatched_messages_total',
  'im_hub_queue_drop_total',
  'im_db_query_total',
  'im_db_query_errors_total',
  'im_db_pool_wait_count_total',
  'im_kafka_consume_total',
  'im_kafka_produce_total',
  'im_redis_ops_total',
  'im_redis_errors_total',
  'im_redis_pubsub_total',
])

const histograms = new Set([
  'im_db_query_duration_seconds',
  'im_redis_latency_seconds',
  'im_kafka_produce_latency_seconds',
  'im_kafka_consume_latency_seconds',
  'im_hub_dispatch_latency_seconds',
  'im_ws_connection_duration_seconds',
])

function parseProm(text) {
  const result = { gauges: {}, counters: {}, hist: {} }
  for (const raw of text.split(/\r?\n/)) {
    const line = raw.trim()
    if (!line || line.startsWith('#')) continue
    const m = line.match(/^([a-zA-Z_:][a-zA-Z0-9_:]*)(?:\{([^}]*)\})?\s+([0-9.eE+NaInf-]+)/)
    if (!m) continue
    const name = m[1]
    const value = Number(m[3])
    if (!Number.isFinite(value)) continue
    if (name.endsWith('_sum')) {
      const base = name.slice(0, -4)
      if (!result.hist[base]) result.hist[base] = { sum: 0, count: 0 }
      result.hist[base].sum += value
    } else if (name.endsWith('_count')) {
      const base = name.slice(0, -6)
      if (!result.hist[base]) result.hist[base] = { sum: 0, count: 0 }
      result.hist[base].count += value
    } else if (gauges.has(name)) {
      result.gauges[name] = (result.gauges[name] || 0) + value
    } else if (counters.has(name) || name.endsWith('_total')) {
      result.counters[name] = (result.counters[name] || 0) + value
    }
  }
  return result
}

function filesFor(prefix) {
  return fs.readdirSync(metricsDir)
    .filter((f) => f.startsWith(prefix + '_') && f.endsWith('.txt'))
    .map((f) => {
      const file = path.join(metricsDir, f)
      const ts = Number(f.match(/_(\d+)\.txt$/)?.[1] || 0)
      return { file, ts }
    })
    .sort((a, b) => a.ts - b.ts)
}

function stat(values) {
  const arr = values.slice().sort((a, b) => a - b)
  return {
    avg: values.reduce((a, b) => a + b, 0) / values.length,
    min: arr[0],
    max: arr[arr.length - 1],
  }
}

const rows = []
for (const prefix of prefixes) {
  const files = filesFor(prefix)
  if (!files.length) {
    console.log(`[warn] no samples for ${prefix}`)
    continue
  }
  const samples = files.map(({ file, ts }) => ({ ts, ...parseProm(fs.readFileSync(file, 'utf8')) }))
  const first = samples[0]
  const last = samples[samples.length - 1]
  const row = { stage: prefix, samples: samples.length }
  for (const name of gauges) {
    const values = samples.map((s) => s.gauges[name] || 0)
    const st = stat(values)
    row[`${name}_max`] = st.max
    row[`${name}_avg`] = Number(st.avg.toFixed(2))
  }
  for (const name of counters) {
    const delta = (last.counters[name] || 0) - (first.counters[name] || 0)
    if (delta > 0) row[`${name}_delta`] = delta
  }
  for (const base of histograms) {
    const dsum = (last.hist[base]?.sum || 0) - (first.hist[base]?.sum || 0)
    const dcount = (last.hist[base]?.count || 0) - (first.hist[base]?.count || 0)
    if (dcount > 0) row[`${base}_avg_ms`] = Number(((dsum / dcount) * 1000).toFixed(3))
  }
  rows.push(row)
}

fs.writeFileSync('ws_metrics_summary.json', JSON.stringify(rows, null, 2), 'utf8')
console.log(JSON.stringify(rows, null, 2))
