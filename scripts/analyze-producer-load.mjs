import fs from 'node:fs';
import path from 'node:path';
import { gunzipSync } from 'node:zlib';

const dir = path.resolve(process.argv[2]);
const read = name => JSON.parse(fs.readFileSync(path.join(dir, name), 'utf8').replace(/^\uFEFF/, ''));
const meta = read('run.json');
const analysis = read('analysis.json');

function sumMetric(raw, name, label = '') {
  let total = 0;
  for (const line of raw.split('\n')) {
    if (!line.startsWith(name + '{') && !line.startsWith(name + ' ')) continue;
    if (label && !line.includes(label)) continue;
    total += Number(line.trim().split(/\s+/).at(-1));
  }
  return total;
}

const series = fs.readdirSync(path.join(dir, 'metrics'))
  .filter(name => name.startsWith('gateway-')).sort().map(name => {
    const at = Number(name.match(/gateway-(\d+)/)[1]);
    const raw = gunzipSync(fs.readFileSync(path.join(dir, 'metrics', name))).toString();
    return {
      at, seconds: (at - meta.t0) / 1000,
      active: sumMetric(raw, 'im_ws_connections_active'),
      read: sumMetric(raw, 'im_ws_read_messages_total'),
      write: sumMetric(raw, 'im_ws_write_messages_total'),
      queue: sumMetric(raw, 'im_kafka_producer_queue_depth'),
      rejections: sumMetric(raw, 'im_kafka_producer_queue_rejections_total'),
      batchSum: sumMetric(raw, 'im_kafka_producer_batch_size_sum'),
      batchCount: sumMetric(raw, 'im_kafka_producer_batch_size_count'),
      produceSuccess: sumMetric(raw, 'im_kafka_produce_total', 'status="success"'),
      produceError: sumMetric(raw, 'im_kafka_produce_total', 'status="error"'),
      produceLatencySum: sumMetric(raw, 'im_kafka_produce_latency_seconds_sum'),
      produceLatencyCount: sumMetric(raw, 'im_kafka_produce_latency_seconds_count'),
    };
  });
const first = series[0], last = series.at(-1);
const delta = (a, b, key) => b[key] - a[key];
const nearest = seconds => series.reduce((a, b) => Math.abs(a.seconds - seconds) < Math.abs(b.seconds - seconds) ? a : b);
const batchCount = delta(first, last, 'batchCount');
const latencyCount = delta(first, last, 'produceLatencyCount');
analysis.producer_batch = {
  max_queue_depth: Math.max(...series.map(row => row.queue)),
  queue_rejections: delta(first, last, 'rejections'),
  kafka_write_calls: batchCount,
  average_batch_messages: batchCount ? delta(first, last, 'batchSum') / batchCount : null,
  acknowledged_messages: delta(first, last, 'produceSuccess'),
  failed_messages: delta(first, last, 'produceError'),
  average_ack_latency_ms: latencyCount ? delta(first, last, 'produceLatencySum') / latencyCount * 1000 : null,
  stages: [
    nearest(analysis.ramp_seconds),
    nearest(analysis.ramp_seconds + analysis.send_seconds),
    nearest(analysis.ramp_seconds + analysis.send_seconds + analysis.quiet_seconds),
  ].map(row => ({
    seconds: row.seconds, active: row.active,
    gateway_read_delta: delta(first, row, 'read'),
    gateway_write_delta: delta(first, row, 'write'),
    queue_depth: row.queue,
    acknowledged_delta: delta(first, row, 'produceSuccess'),
    failed_delta: delta(first, row, 'produceError'),
  })),
};
fs.writeFileSync(path.join(dir, 'producer-series.json'), JSON.stringify(series, null, 2));
fs.writeFileSync(path.join(dir, 'analysis.json'), JSON.stringify(analysis, null, 2));
const p = analysis.producer_batch;
fs.appendFileSync(path.join(dir, 'report.md'), `
## Gateway Kafka 批量生产

| 指标 | 结果 |
|---|---:|
| Kafka实际批量写调用 | ${p.kafka_write_calls} |
| 平均实际批次（条） | ${p.average_batch_messages?.toFixed(1) ?? 'N/A'} |
| 队列采样峰值（条） | ${p.max_queue_depth} |
| 队列满拒绝 | ${p.queue_rejections} |
| Kafka确认成功消息 | ${p.acknowledged_messages} |
| Kafka批次失败涉及消息 | ${p.failed_messages} |
| 入队至Kafka确认平均延迟（ms） | ${p.average_ack_latency_ms?.toFixed(2) ?? 'N/A'} |

批次最多1000条、1 MiB、等待5ms；有界队列上限20000条。调用方在所属批次得到Kafka确认后才返回成功。
原始 Gateway 指标每2秒采样，队列瞬时峰值可能高于表中采样峰值。

| 阶段（起点后秒） | 活跃连接 | Gateway累计读取 | Gateway累计写出 | 当前生产队列 | Kafka确认成功 | Kafka失败 |
|---:|---:|---:|---:|---:|---:|---:|
${p.stages.map(s => `| ${s.seconds.toFixed(1)} | ${s.active} | ${s.gateway_read_delta} | ${s.gateway_write_delta} | ${s.queue_depth} | ${s.acknowledged_delta} | ${s.failed_delta} |`).join('\n')}
`);
console.log(JSON.stringify(p, null, 2));
