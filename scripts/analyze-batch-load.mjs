import fs from 'node:fs';
import path from 'node:path';
import { gunzipSync } from 'node:zlib';

const dir = path.resolve(process.argv[2]);
const read = name => JSON.parse(fs.readFileSync(path.join(dir, name), 'utf8').replace(/^\uFEFF/, ''));
const analysis = read('analysis.json');
let firstSocketError = null;
for (const name of fs.readdirSync(path.join(dir, 'metrics')).filter(name => name.startsWith('prometheus-')).sort()) {
  const snapshot = JSON.parse(gunzipSync(fs.readFileSync(path.join(dir, 'metrics', name))));
  const count = (snapshot.data?.result || []).filter(item => item.metric.__name__ === 'k6_load_socket_errors_total' && item.metric.testid === analysis.run_id).reduce((sum, item) => sum + Number(item.value[1]), 0);
  if (count > 0) { firstSocketError = (Number(name.match(/prometheus-(\d+)/)[1]) - analysis.t0) / 1000; break; }
}
analysis.first_socket_error_sample_seconds = firstSocketError;
const samples = fs.readFileSync(path.join(dir, 'kafka-lag.jsonl'), 'utf8').trim().split(/\r?\n/).filter(Boolean).map(JSON.parse).map(sample => {
  const groups = {};
  for (const line of sample.text.split('\n')) {
    const fields = line.trim().split(/\s+/);
    if (!/^im_(sequencer|relay)_/.test(fields[0]) || fields.length < 6) continue;
    if (![fields[3], fields[4], fields[5]].every(value => /^\d+$/.test(value))) continue;
    groups[fields[0].startsWith('im_sequencer_') ? 'sequencer' : 'relay'] = { committed: Number(fields[3]), end: Number(fields[4]), lag: Number(fields[5]) };
  }
  return { at: sample.at, seconds: (sample.at - analysis.t0) / 1000, archiver_last_offset: sample.archiver_last_offset ?? null, ...groups };
}).filter(sample => sample.sequencer && sample.relay);
const nearest = seconds => samples.reduce((a, b) => Math.abs(a.seconds - seconds) < Math.abs(b.seconds - seconds) ? a : b);
const sendEnd = analysis.ramp_seconds + analysis.send_seconds;
const disconnect = sendEnd + analysis.quiet_seconds;
const selected = [nearest(sendEnd), nearest(disconnect), samples.at(-1)];
analysis.kafka = {
  max_sequencer_lag: Math.max(...samples.map(sample => sample.sequencer.lag)),
  max_relay_lag: Math.max(...samples.map(sample => sample.relay.lag)),
  around_send_end: selected[0], around_disconnect: selected[1], final_sample: selected[2],
};
analysis.sequencer_metric_samples = fs.readdirSync(path.join(dir, 'metrics')).filter(name => name.startsWith('sequencer-')).length;
fs.writeFileSync(path.join(dir, 'kafka-series.json'), JSON.stringify(samples, null, 2));
fs.writeFileSync(path.join(dir, 'analysis.json'), JSON.stringify(analysis, null, 2));
fs.appendFileSync(path.join(dir, 'report.md'), `
## 批量编号与 Kafka 积压

编号器采用每批上限4000条、输入key/value上限1 MiB、收集等待5ms，写入RequireAll后才批量提交偏移。实际批次随供给变化，不保证凑满4000条。
本轮使用独立编号器容器；旧的暂停容器保留原内存状态。编号和广播 Relay 都使用相同批次边界；Relay 通过 Redis Pipeline 发布整批消息，成功后批量提交 Kafka 偏移。

| 采样时刻（起点后秒） | 入口末尾 | 编号已提交 | 编号积压 | 正式主题末尾 | 广播已提交 | 广播积压 | 归档最后offset |
|---:|---:|---:|---:|---:|---:|---:|---:|
${selected.map(s => `| ${s.seconds.toFixed(1)} | ${s.sequencer.end} | ${s.sequencer.committed} | ${s.sequencer.lag} | ${s.relay.end} | ${s.relay.committed} | ${s.relay.lag} | ${s.archiver_last_offset ?? '未采集'} |`).join('\n')}

- 编号积压采样峰值：${analysis.kafka.max_sequencer_lag}；广播积压采样峰值：${analysis.kafka.max_relay_lag}。
- Kafka主题包含正式压测前的预检消息；正式压测以run_id区分。偏移量表示记录位置，不等于已验证的唯一业务消息数。
- 每15秒启动一次Kafka组查询，查询耗时和两个组的读取时间差会影响瞬时数值；表中是接近阶段边界的采样。
- 新编号器原始metrics共${analysis.sequencer_metric_samples}份，每2秒抓取到metrics/sequencer-*.prom.gz；不依赖旧编号器的Prometheus目标。
- 本轮客户端已支持换行合帧解析。上一轮客户端有合帧解析缺陷，其接收量不能与本轮直接作准确交付率比较。
- k6退出码：${analysis.code}；Socket错误首次采样时刻：${firstSocketError === null ? '未观察到' : firstSocketError.toFixed(1)+'秒'}。错误文本未记录，不能仅凭时刻确定原因。
`);
console.log(JSON.stringify(analysis.kafka, null, 2));
