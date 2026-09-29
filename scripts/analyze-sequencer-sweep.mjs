import fs from 'node:fs';
import path from 'node:path';

const dir = path.resolve(process.argv[2]);
const text = fs.readFileSync(path.join(dir, 'results.log'), 'utf8').replace(/^\uFEFF/, '');
const rounds = [];
let current;
for (const line of text.split(/\r?\n/)) {
  const run = line.match(/^=== RUN.*batch_(\d+)\/round_(\d+)/);
  if (run) { current = { cap: Number(run[1]), round: Number(run[2]) }; rounds.push(current); }
  if (current && /\b(BATCH|RESULT) /.test(line)) {
    for (const match of line.matchAll(/([a-zA-Z_]+)=([\d.]+)/g)) current[match[1]] = Number(match[2]);
  }
  if (current && line.includes('VERIFIED count=')) current.verified = true;
}
const caps = [0,100,500,1000,2000,3000,4000,5000,6000,7000,8000,9000,10000];
const mean = values => values.reduce((sum, n) => sum+n, 0) / values.length;
const rows = caps.map(cap => {
  const cases = rounds.filter(row => row.cap === cap && row.verified && row.steady_rate);
  if (cases.length !== 2) throw new Error(`Expected two verified measurements for batch ${cap}, got ${cases.length}`);
  return {
    batch_cap: cap,
    mean_messages_per_second: mean(cases.map(row => row.steady_rate)),
    min_messages_per_second: Math.min(...cases.map(row => row.steady_rate)),
    max_messages_per_second: Math.max(...cases.map(row => row.steady_rate)),
    mean_processing_ms: mean(cases.map(row => row.steady_seconds))*1000,
    peak_heap_MiB: Math.max(...cases.map(row => row.peak_heap_MiB)),
    mean_alloc_MiB: mean(cases.map(row => row.alloc_MiB)),
    mean_actual_batch: mean(cases.map(row => row.average)),
    largest_actual_batch: Math.max(...cases.map(row => row.largest)),
    mean_write_calls: mean(cases.map(row => row.write_calls)),
  };
});
fs.writeFileSync(path.join(dir, 'measurements.json'), JSON.stringify({ rounds, rows }, null, 2));
const fields = Object.keys(rows[0]);
fs.writeFileSync(path.join(dir, 'comparison.csv'), fields.join(',')+'\n'+rows.map(row => fields.map(field => row[field]).join(',')).join('\n')+'\n');
const best = rows.reduce((a,b) => a.mean_messages_per_second > b.mean_messages_per_second ? a : b);
const report = `# 编号器 Kafka 批量上限阶梯测试

每档两轮、每轮预写 10000 条消息到独立 Kafka 主题，均匀分布于 100 个逻辑群，正文为 #。
0 表示原始逐条处理基线；其余档位运行 RunBatched，每批最多等待 5ms，输入 key/value 总量上限 1 MiB，单批在途。
Kafka 单分区、RequireAll、同步写入；Writer.BatchSize 与应用批量上限一致，Writer.BatchBytes=1 MiB、BatchTimeout=5ms。
所有配置串行执行，以避免不同档位竞争同一 Kafka；原业务服务与暂停的编号器没有部署更新。

以下吞吐排除了消费者首次获取消息的启动等待，也不包括预写、结果验证及关闭客户端的时间。

| 每批上限 | 平均条/秒 | 两轮范围 | 处理1万条平均ms | Go堆采样峰值MiB | 平均实际批次 | 最大实际批次 | 总分配平均MiB |
|---|---:|---:|---:|---:|---:|---:|---:|
${rows.map(row => `| ${row.batch_cap || '0（逐条）'} | ${row.mean_messages_per_second.toFixed(0)} | ${row.min_messages_per_second.toFixed(0)}–${row.max_messages_per_second.toFixed(0)} | ${row.mean_processing_ms.toFixed(1)} | ${row.peak_heap_MiB.toFixed(2)} | ${row.mean_actual_batch.toFixed(1)} | ${row.largest_actual_batch} | ${row.mean_alloc_MiB.toFixed(2)} |`).join('\n')}

## 解释与限制

- 本轮最高平均吞吐出现在上限 ${best.batch_cap}，约 ${best.mean_messages_per_second.toFixed(0)} 条/秒；这是本机、短消息、预先积压、1万条/轮的结果，不代表生产最优值或端到端吞吐。
- 每个档位都实际运行并验证 10000 条正式输出：无额外记录、ID 唯一、100 个群的序号连续、输入顺序与内容一致。
- 批次上限不等于实际批次大小：5ms 收集窗口、读取供给及字节上限可能先触发。
- Go HeapAlloc 每50ms采样，并记录结束值；短测试仍可能漏掉瞬时峰值。表中内存不是容器RSS，不含Kafka Broker和进程全部开销。
- 总分配量是处理期间累积分配，不是同时驻留内存。每轮前执行GC，仍可能有系统缓存、GC及运行顺序带来的波动。
- 本机其他服务仍在运行，部分档位执行期间也运行了 messages 包测试，因此不是独占硬件环境。
- 收集与写入等待分别最多5ms，若没有凑满Kafka写入批次，两次等待可能叠加。
- 只改了可调用的批处理入口及测试，没有启用1000线程或修改当前部署。内存去重表与编号的重启恢复问题依旧存在。

原始日志 results.log；逐轮数据 measurements.json；表格 comparison.csv；Linux 测试二进制 sequencer.test。

## 测试调用

显式设置 KAFKA_SEQUENCER_TEST_BROKERS 后运行 TestKafkaSequencerBatchSweep；未设置时默认跳过，避免普通单元测试连接外部服务。
本次在 Docker 网络 lan-sync_lan-im-network 内用 kafka:9092，SEQUENCER_PERF_MESSAGES=10000。
可执行测试参数：-test.run=^TestKafkaSequencerBatchSweep$ -test.timeout=15m -test.v。
批处理边界、超时刷新、重试编号稳定性及失败不提交的单元测试通过；go test ./services/messages/... 通过。
`;
fs.writeFileSync(path.join(dir, 'report.md'), report);
console.log(JSON.stringify(rows, null, 2));
