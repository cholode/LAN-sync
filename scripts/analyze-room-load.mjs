import fs from 'node:fs';
import path from 'node:path';
import { gunzipSync } from 'node:zlib';

const dir = path.resolve(process.argv[2]);
const read = name => JSON.parse(fs.readFileSync(path.join(dir, name), 'utf8').replace(/^\uFEFF/, ''));
const lines = name => fs.readFileSync(path.join(dir, name), 'utf8').trim().split(/\r?\n/).filter(Boolean).map(JSON.parse);
const meta = read('run.json');
const summary = read('k6-summary.json');
const metric = name => summary.metrics[name] || {};
const hosts = lines('host-client.jsonl');
const containers = lines('containers.jsonl');
const mib = bytes => (bytes / 1024 ** 2).toFixed(1);
const memoryBytes = raw => {
  const match = raw.match(/^([\d.]+)\s*(B|KiB|MiB|GiB|TiB)/);
  return match ? Number(match[1]) * 1024 ** ['B', 'KiB', 'MiB', 'GiB', 'TiB'].indexOf(match[2]) : 0;
};
const peaks = {};
for (const row of containers) for (const sample of row.stats) {
  const name = sample.Name;
  const entry = peaks[name] ||= { memory: 0, cpuPercent: 0 };
  entry.memory = Math.max(entry.memory, memoryBytes(sample.MemUsage));
  entry.cpuPercent = Math.max(entry.cpuPercent, parseFloat(sample.CPUPerc));
}
const snapshots = [];
const sampleValue = line => {
  const fields = line.trim().split(/\s+/);
  // Prometheus /federate appends a millisecond timestamp after the value;
  // direct /metrics exposition does not.
  return Number(fields.length >= 3 ? fields.at(-2) : fields.at(-1));
};
const sumMetric = (raw, name, label = '') => raw.split('\n').reduce((sum, line) => {
  if (!line.startsWith(name + '{') && !line.startsWith(name + ' ')) return sum;
  if (label && !line.includes(label)) return sum;
  return sum + sampleValue(line);
}, 0);
const histogramBuckets = (raw, name) => {
  const buckets = {};
  for (const line of raw.split('\n')) {
    if (!line.startsWith(name + '_bucket{')) continue;
    const le = line.match(/le="([^"]+)"/)?.[1];
    if (!le) continue;
    buckets[le] = (buckets[le] || 0) + sampleValue(line);
  }
  return buckets;
};
const valuesByLabel = (raw, name, labelName) => {
  const values = {};
  for (const line of raw.split('\n')) {
    if (!line.startsWith(name + '{')) continue;
    const label = line.match(new RegExp(`${labelName}="([^"]+)"`))?.[1];
    if (label) values[label] = (values[label] || 0) + sampleValue(line);
  }
  return values;
};
const histogramQuantile = (quantile, first, last) => {
  const cumulative = Object.keys(last).filter(le => le !== '+Inf').map(Number).sort((a, b) => a - b);
  const total = (last['+Inf'] || 0) - (first['+Inf'] || 0);
  if (!total) return null;
  const target = total * quantile;
  let previousCount = 0;
  let previousBound = 0;
  for (const bound of cumulative) {
    const key = String(bound);
    const count = (last[key] || 0) - (first[key] || 0);
    if (count >= target) {
      if (count === previousCount) return bound;
      return previousBound + (bound - previousBound) * (target - previousCount) / (count - previousCount);
    }
    previousCount = count;
    previousBound = bound;
  }
  return cumulative.at(-1) ?? null;
};
const gatewayFiles = fs.readdirSync(path.join(dir, 'metrics')).filter(name => name.startsWith('gateway-') && name.endsWith('.prom.gz'));
const gatewayGroups = new Map();
for (const file of gatewayFiles) {
  const match = file.match(/-(\d+)\.prom\.gz$/);
  if (!match) continue;
  const at = Number(match[1]);
  const files = gatewayGroups.get(at) || [];
  files.push(file);
  gatewayGroups.set(at, files);
}
for (const [at, files] of [...gatewayGroups.entries()].sort((a, b) => a[0] - b[0])) {
  const raw = files.map(file => gunzipSync(fs.readFileSync(path.join(dir, 'metrics', file))).toString()).join('\n');
  snapshots.push({
    at,
    active: sumMetric(raw, 'im_ws_connections_active'),
    activeByNode: valuesByLabel(raw, 'im_ws_connections_active', 'node_id'),
    read: sumMetric(raw, 'im_ws_read_messages_total'),
    write: sumMetric(raw, 'im_ws_write_messages_total'),
    frames: sumMetric(raw, 'im_ws_write_frames_total'),
    batchMessages: sumMetric(raw, 'im_ws_write_batch_messages_sum'),
    batchFrames: sumMetric(raw, 'im_ws_write_batch_messages_count'),
    gatewayTransitSeconds: sumMetric(raw, 'im_gateway_message_transit_seconds_total'),
    gatewayTransitMessages: sumMetric(raw, 'im_gateway_message_transit_total'),
    gatewayFrameLatencySum: sumMetric(raw, 'im_gateway_frame_oldest_message_latency_seconds_sum'),
    gatewayFrameLatencyCount: sumMetric(raw, 'im_gateway_frame_oldest_message_latency_seconds_count'),
    gatewayFrameLatencyBuckets: histogramBuckets(raw, 'im_gateway_frame_oldest_message_latency_seconds'),
    readErrors: sumMetric(raw, 'im_ws_read_errors_total'),
    readTimeouts: sumMetric(raw, 'im_ws_read_errors_total', 'error_type="timeout"'),
    writeErrors: sumMetric(raw, 'im_ws_write_errors_total'),
    drops: sumMetric(raw, 'im_hub_queue_drops_total'),
    produce: sumMetric(raw, 'im_kafka_produce_total'),
  });
}
const first = snapshots[0], last = snapshots.at(-1);
const gatewayTransitMessages = last.gatewayTransitMessages - first.gatewayTransitMessages;
const gatewayFrameLatencyCount = last.gatewayFrameLatencyCount - first.gatewayFrameLatencyCount;
const gatewayFrameP95 = histogramQuantile(0.95, first.gatewayFrameLatencyBuckets, last.gatewayFrameLatencyBuckets);
const gatewayFrameP99 = histogramQuantile(0.99, first.gatewayFrameLatencyBuckets, last.gatewayFrameLatencyBuckets);
const firstFull = snapshots.find(sample => sample.active >= 10000);
const holdStart = meta.t0 + meta.ramp_seconds * 1000;
const holdEnd = holdStart + (meta.send_seconds + meta.quiet_seconds) * 1000;
const holdSamples = snapshots.filter(sample => sample.at >= holdStart && sample.at < holdEnd);
const sent = metric('load_sent').count || 0;
const received = metric('load_received').count || 0;
const receivedFrames = metric('load_received_frames').count || metric('ws_msgs_received').count || 0;
const results = {
  ...meta,
  opened: metric('load_opened').count || 0,
  handshake_failures: metric('load_handshake_failures').count || 0,
  ramp_reconnects: metric('load_ramp_reconnects').count || 0,
  ramp_connection_errors: metric('load_ramp_connection_errors').count || 0,
  closed: metric('load_closed').count || 0,
  peak_server_connections: Math.max(...snapshots.map(sample => sample.active)),
  first_full_seconds: firstFull ? (firstFull.at - meta.t0) / 1000 : null,
  full_connection_distribution: firstFull?.activeByNode || null,
  hold_samples: holdSamples.length,
  minimum_server_connections_during_hold: holdSamples.length ? Math.min(...holdSamples.map(sample => sample.active)) : null,
  final_server_connections: last.active,
  planned_messages: meta.rooms * meta.senders_per_room * meta.rate_per_sender * meta.send_seconds,
  client_sent: sent,
  client_send_rate: sent / meta.send_seconds,
  client_send_slots_skipped: metric('load_send_slots_skipped').count || 0,
  client_deliveries: received,
  client_received_frames: receivedFrames,
  expected_deliveries_if_all_sent_delivered: sent * meta.members_per_room,
  socket_errors: metric('load_socket_errors').count || 0,
  early_closed: metric('load_early_closed').count || 0,
  nonmonotonic: metric('load_nonmonotonic_delivery').count || 0,
  malformed: metric('load_malformed_delivery').count || 0,
  gateway_message_transit_average_ms: gatewayTransitMessages ? (last.gatewayTransitSeconds - first.gatewayTransitSeconds) / gatewayTransitMessages * 1000 : null,
  gateway_frame_oldest_average_ms: gatewayFrameLatencyCount ? (last.gatewayFrameLatencySum - first.gatewayFrameLatencySum) / gatewayFrameLatencyCount * 1000 : null,
  gateway_frame_oldest_p95_ms: gatewayFrameP95 === null ? null : gatewayFrameP95 * 1000,
  gateway_frame_oldest_p99_ms: gatewayFrameP99 === null ? null : gatewayFrameP99 * 1000,
  server_read_delta: last.read - first.read,
  server_write_delta: last.write - first.write,
  server_frame_delta: last.frames - first.frames,
  average_messages_per_frame: (last.batchMessages - first.batchMessages) / Math.max(1, last.batchFrames - first.batchFrames),
  server_read_errors_delta: last.readErrors - first.readErrors,
  server_read_timeouts_delta: last.readTimeouts - first.readTimeouts,
  server_write_errors_delta: last.writeErrors - first.writeErrors,
  server_queue_drops_delta: last.drops - first.drops,
  client_peak_working_set: Math.max(...hosts.map(row => row.k6WorkingSet || 0)),
  client_peak_private: Math.max(...hosts.map(row => row.k6Private || 0)),
  wsl_peak_working_set: Math.max(...hosts.map(row => row.wslWorkingSet || 0)),
  windows_min_free_bytes: Math.min(...hosts.map(row => row.freePhysicalKiB * 1024)),
  container_peaks: peaks,
  gateway_samples: snapshots.length,
  gateway_raw_files: gatewayFiles.length,
  prometheus_samples: fs.readdirSync(path.join(dir, 'metrics')).filter(name => name.startsWith('prometheus-')).length,
};
fs.writeFileSync(path.join(dir, 'analysis.json'), JSON.stringify(results, null, 2));
fs.writeFileSync(path.join(dir, 'gateway-series.json'), JSON.stringify(snapshots));
const silent = meta.senders_per_room === 0;
const clientDescription = meta.client_mode === 'remote-go'
  ? '专用 Go 静默连接器在云服务器的 512 MiB/1 CPU 限制容器内运行；适用于连接稳定性验证。'
  : meta.client_mode === 'go'
    ? '专用 Go 静默连接器在 Windows 客户端运行。'
    : `k6 在 Windows 客户端运行，服务端位于 ${meta.server || '配置的目标环境'}。`;
const report = `# ${meta.connections} 连接、${meta.rooms} 群压测

- Run ID: ${meta.run_id}
- 起点 UTC：${new Date(meta.t0).toISOString()}（北京时间 +08:00）
- ${meta.ramp_seconds} 秒平缓建连；${meta.send_seconds} 秒${silent ? '静默保持' : '发送'}；${meta.quiet_seconds} 秒静默；随后断连。
- 每群 ${meta.members_per_room} 人，其中 ${meta.senders_per_room} 人各以 ${meta.rate_per_sender} 条/秒发送单字符 #。
- ${clientDescription}

| 项目 | 结果 |
|---|---:|
| 连接成功 | ${results.opened} |
| 握手失败 | ${results.handshake_failures} |
| 建连期指数退避重试 | ${results.ramp_reconnects} |
| 建连期连接错误（不计正式失败） | ${results.ramp_connection_errors} |
| 服务端连接峰值 | ${results.peak_server_connections} |
| 首次采样到全部连接（秒） | ${results.first_full_seconds} |
| 满连接时三 Gateway 分布 | ${results.full_connection_distribution ? Object.entries(results.full_connection_distribution).map(([node, count]) => `${node}=${count}`).join('，') : 'N/A'} |
| 保持窗口服务端采样数 | ${results.hold_samples} |
| 保持窗口最低活动连接 | ${results.minimum_server_connections_during_hold ?? 'N/A'} |
| 最后服务端连接数 | ${results.final_server_connections} |
| 计划发送消息 | ${results.planned_messages} |
| 客户端实际发送 | ${sent} |
| 实际平均发送速率（条/秒） | ${results.client_send_rate.toFixed(2)} |
| 客户端错过的发送时隙 | ${results.client_send_slots_skipped} |
| 客户端收到的广播次数 | ${received} |
| 客户端收到的WebSocket帧 | ${receivedFrames} |
| 全部发送均广播到 ${meta.members_per_room} 人时应有次数 | ${sent * meta.members_per_room} |
| 服务端读取消息增量 | ${results.server_read_delta} |
| 服务端写入消息增量 | ${results.server_write_delta} |
| 服务端实际写出帧数 | ${results.server_frame_delta} |
| 平均每帧业务消息数 | ${results.average_messages_per_frame.toFixed(2)} |
| 服务端读循环结束（含计划主动断连） | ${results.server_read_errors_delta} |
| 其中服务端读取超时 | ${results.server_read_timeouts_delta} |
| 服务端写错误 | ${results.server_write_errors_delta} |
| 服务端队列丢弃增量 | ${results.server_queue_drops_delta} |
| 提前断连 | ${results.early_closed} |
| Socket 错误 | ${results.socket_errors} |
| 重复或乱序广播 | ${results.nonmonotonic} |
| 非法或错群广播 | ${results.malformed} |
| Gateway内部消息平均延迟（毫秒） | ${results.gateway_message_transit_average_ms?.toFixed(2) ?? 'N/A'} |
| Gateway出站帧最老消息平均延迟（毫秒） | ${results.gateway_frame_oldest_average_ms?.toFixed(2) ?? 'N/A'} |
| Gateway出站帧最老消息 p95（毫秒） | ${results.gateway_frame_oldest_p95_ms?.toFixed(2) ?? 'N/A'} |
| Gateway出站帧最老消息 p99（毫秒） | ${results.gateway_frame_oldest_p99_ms?.toFixed(2) ?? 'N/A'} |
| ${meta.client_mode === 'remote-go' ? '本地协调进程' : '压测客户端'}物理内存峰值（MiB） | ${mib(results.client_peak_working_set)} |
| ${meta.client_mode === 'remote-go' ? '本地协调进程' : '压测客户端'}私有提交内存峰值（MiB） | ${mib(results.client_peak_private)} |
| WSL 物理内存峰值（MiB） | ${mib(results.wsl_peak_working_set)} |
| Windows 最小可用物理内存（MiB） | ${mib(results.windows_min_free_bytes)} |

Gateway链路延迟从入站WebSocket读取完成开始，到对应出站WebSocket帧在Gateway成功写完为止；不包含客户端到Gateway的上行网络、Gateway到客户端的下行网络、客户端事件循环和JSON解析。
客户端发送成功仅表示写入客户端连接，不代表服务端已接收、编号、入库或交付。
不能仅凭广播差额断言永久丢失，还需结合 Kafka backlog 判断积压。

## 容器资源峰值（10 秒采样，CPU 100% 相当于一个逻辑核）

| 容器 | 内存 MiB | CPU % |
|---|---:|---:|
${Object.entries(peaks).map(([name, peak]) => `| ${name} | ${mib(peak.memory)} | ${peak.cpuPercent.toFixed(2)} |`).join('\n')}

## 原始数据

- Gateway 聚合采样：${results.gateway_samples} 组，原始 /metrics 共 ${results.gateway_raw_files} 份，每 2 秒采样，metrics/gateway-*.prom.gz。
- Prometheus 服务端与本轮 k6 指标：${results.prometheus_samples} 份，每 5 秒采样，metrics/prometheus-*.json.gz。
- containers.jsonl、host-client.jsonl：每 10 秒容器及主机/客户端资源。
- k6-summary.json：客户端汇总；run.json：时间线与进程退出状态。
- capture-errors.jsonl（若存在）：采集错误。
`;
fs.writeFileSync(path.join(dir, 'report.md'), report);
console.log(JSON.stringify({ ...results, container_peaks: undefined }, null, 2));
