# 10000 连接云端压测

这套脚本固定使用 10000 个账号、1000 个群、每群 10 人。账号、密码和 JWT 保存在 `data/load-fixture-10000x1000/`，该目录已被 Git 忽略，不能提交到仓库。

## 地址

- HTTP API：`http://47.113.227.173/api/v1`
- WebSocket：`ws://47.113.227.173/api/v1/ws?token=<JWT>`
- 健康检查：`http://47.113.227.173/health/ready`
- Grafana：`http://47.113.227.173:3000/`
- Prometheus：仅服务器本机 `http://127.0.0.1:9090`，脚本通过 Workbench CLI 采集，不对公网开放。

## 一次性准备账号和群

在仓库根目录运行：

```powershell
$env:PERF_API_BASE='http://47.113.227.173/api/v1'
node scripts/seed-load-fixture.mjs data/load-fixture-10000x1000
```

脚本可断点续跑。完成后 `users.json` 保存全部账号、密码、JWT、用户 ID 和群 ID；`rooms.json` 保存群与成员；`report.json` 保存校验结果；`state.json` 是断点状态。

## 直接运行压测

默认用 120 秒平缓建立连接，全部 10000 个连接保持并发送 180 秒。每个用户每秒发送一条单字符 `#`，即入站 10000 条/秒、理论广播投递 100000 条/秒。

```powershell
$stamp=Get-Date -Format 'yyyyMMdd-HHmmss'
$env:PERF_RECEIVE_ONLY='true'
node scripts/run-cloud-room-load.mjs "data/load-cloud-10000x1000-$stamp"
node scripts/analyze-room-load.mjs "data/load-cloud-10000x1000-$stamp"
```

`PERF_RECEIVE_ONLY=true` 让客户端只统计收到的 WebSocket 帧，不解析广播 JSON，避免把 k6 的解析能力误当成服务端瓶颈。设为 `false` 可以校验消息内容和单群顺序，但 10 万投递/秒时客户端 CPU 和内存会明显增加。

常用覆盖项：

```powershell
$env:PERF_RAMP_SECONDS='120'
$env:PERF_SEND_SECONDS='180'
$env:PERF_QUIET_SECONDS='0'
$env:PERF_HEARTBEAT_ENABLED='true'
$env:PERF_K6_EXE='C:\path\to\k6.exe'
```

## 结果和指标

每次输出目录包含：

- `report.md` / `analysis.json`：最终结论和机器可读汇总。
- `run.json`：拓扑、时间线、进程退出码。
- `k6-summary.json`、`k6.out`、`k6.err`：客户端连接、发送、断连和资源结果。
- `metrics/gateway-*.prom.gz`：三个 Gateway 的原始 `/metrics`，每 2 秒采样。
- `metrics/prometheus-*.json.gz`：Prometheus 中全部 `im_*` 指标，每 5 秒采样。
- `containers.jsonl`：云服务器所有容器 CPU/内存，每 10 秒采样。
- `host-client.jsonl`：k6 客户端 CPU/内存和 Windows 可用内存，每 10 秒采样。
- `capture-errors.jsonl`：仅采集失败时出现。

报告中的业务延迟只采用 Gateway 服务端指标：从 Gateway 完成入站 WebSocket 读取，到对应出站 WebSocket 帧成功写完。它排除了客户端到服务端的网络 RTT、客户端事件循环和 JSON 解析时间。`load_handshake_ms` 是客户端视角建连耗时，不能当作业务链路延迟。

其他 Agent 只需在同一台机器、仓库根目录运行上述两条命令。Workbench CLI 使用已配置的实例 `i-f8zcl1p77evq5haq9svg`（`cn-heyuan`）读取云端指标，Prometheus 无需开放公网端口。

手机热点等网络无法维持 Workbench 控制通道时，可通过 Grafana 代理采集 Prometheus 指标：

```powershell
$env:PERF_METRICS_MODE='grafana'
$env:PERF_GRAFANA_USER='admin'
$env:PERF_GRAFANA_PASSWORD='<Grafana 密码>'
```

该模式仍保存全部 `im_*` 服务指标，但无法执行远端 `docker stats`，所以 `containers.jsonl` 会明确标记容器资源采样不可用。

仅测试静默连接时，使用专用 Go 客户端可避免 k6 每 VU 事件循环影响 WebSocket ping/pong：

```powershell
$env:PERF_CLIENT_MODE='go'
$env:PERF_SENDERS_PER_ROOM='0'
$env:PERF_SEND_SECONDS='600'
```

当公网链路本身限制连接数时，可将已编译的客户端和 `users.json` 放到服务器 `/opt/lan-im/loadtest/`，再设 `$env:PERF_CLIENT_MODE='remote-go'`。Runner 会在服务器启动临时 `im-load-client` 容器，经宿主机 Nginx 完整走鉴权和三 Gateway 负载均衡，并在结束后自动删除该容器。

远端连接器的一次性安装命令：

```powershell
$env:GOOS='linux'; $env:GOARCH='amd64'
go build -o data/ws-silent-load-linux-amd64 scripts/ws-silent-load.go
Remove-Item Env:GOOS,Env:GOARCH
$wb="$env:LOCALAPPDATA\Programs\workbench\workbench.exe"
& $wb exec -i i-f8zcl1p77evq5haq9svg -r cn-heyuan -c 'mkdir -p /opt/lan-im/loadtest && chmod 700 /opt/lan-im/loadtest' --timeout 30 -o json
& $wb upload data/ws-silent-load-linux-amd64 /tmp/ -i i-f8zcl1p77evq5haq9svg -r cn-heyuan -f -o json
& $wb upload data/load-fixture-10000x1000/users.json /opt/lan-im/loadtest/users.json -i i-f8zcl1p77evq5haq9svg -r cn-heyuan -f -o json
& $wb exec -i i-f8zcl1p77evq5haq9svg -r cn-heyuan -c 'mv /tmp/ws-silent-load-linux-amd64 /opt/lan-im/loadtest/ws-silent-load && chmod 700 /opt/lan-im/loadtest/ws-silent-load && chmod 600 /opt/lan-im/loadtest/users.json' --timeout 30 -o json
```

正式静默连接测试：

```powershell
$stamp=Get-Date -Format 'yyyyMMdd-HHmmss'
$env:PERF_CLIENT_MODE='remote-go'
$env:PERF_METRICS_MODE='workbench'
$env:PERF_CONNECTIONS='10000'
$env:PERF_ROOMS='1000'
$env:PERF_MEMBERS_PER_ROOM='10'
$env:PERF_SENDERS_PER_ROOM='0'
$env:PERF_RAMP_SECONDS='120'
$env:PERF_SEND_SECONDS='600'
node scripts/run-cloud-room-load.mjs "data/load-cloud-remote-go-silent-10000-10m-$stamp"
node scripts/analyze-room-load.mjs "data/load-cloud-remote-go-silent-10000-10m-$stamp"
```
