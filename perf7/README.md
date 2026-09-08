# Gateway 单实例 / 三实例对照压测

当前已准备配置，未部署或执行负载。先确认目标测试环境和账号数据。

## 部署模型

使用根目录 docker-compose.gateway-perf.yml 与原 Compose 文件叠加。
backend / gateway-2 / gateway-3 使用相同程序，Nginx least_conn 分配新连接。
归档已迁入独立 message-worker，三份 Gateway 均不再运行归档。
消息/文件 API 由独立 message-service 处理；Gateway 仍初始化数据库及现有 gRPC 等模块。
归档单点保持不变，本轮不宣称归档高可用。
每实例默认 1 CPU / 2G，可通过 PERF_GATEWAY_CPUS、PERF_GATEWAY_MEMORY 调整。

## 在已确认的测试服务器上运行（仓库根目录，Linux shell）

先构建三份相同源码镜像：

~~~sh
docker compose -f docker-compose.yml -f docker-compose.gateway-perf.yml build backend gateway-2 gateway-3 message-service message-worker nginx
~~~

基线使用同一个覆盖文件控制资源，只启动 backend，Nginx 使用原来的单上游模板：

~~~sh
PERF_NGINX_TEMPLATE=./deploy/nginx/templates/default.conf.template docker compose -f docker-compose.yml -f docker-compose.gateway-perf.yml up -d backend message-service message-worker nginx prometheus grafana
~~~

三实例阶段：

~~~sh
docker compose -f docker-compose.yml -f docker-compose.gateway-perf.yml up -d backend gateway-2 gateway-3 message-service message-worker nginx prometheus grafana
docker compose exec nginx nginx -t
~~~

必须等三份服务全部就绪，再开始压测。检查每实例日志与在线连接数。
覆盖文件会把 Prometheus 配置切换为 `perf7/prometheus.yml`，自动抓取
`backend:6060`、`gateway-2:6060` 和 `gateway-3:6060`，并以 `node_id` 区分实例。
打开 `http://YOUR_TEST_HOST:9090/targets`：单实例阶段允许 gateway-2、gateway-3 为 `DOWN`，但 gateway-1 必须为 `UP`；三实例阶段必须确认三个 `lan-im-gateways` Target 全部为 `UP` 后再执行测试。
不要直接使用 --scale backend=3：原服务有固定名称、宿主机端口和 NODE_ID。首次从旧版本切换时请按 services/messages/README.md 停止旧归档器；此后 message-worker 始终只运行一份。

## 测试账号

在 perf7/users.json 放置测试账号数组，每项含 token；所有用户须已加入指定测试群，账号不能重复、令牌不能过期。不要提交令牌文件。
负载机建议独立于服务器。使用 k6 执行，显式设置授权测试目标：

~~~sh
K6_PROMETHEUS_RW_SERVER_URL=http://YOUR_TEST_HOST:9090/api/v1/write \
K6_PROMETHEUS_RW_TREND_STATS='p(50),p(95),p(99),max' \
k6 run -o experimental-prometheus-rw \
  -e PERF_TEST_ID=single-500-r1 \
  -e PERF_TOPOLOGY=single \
  -e PERF_WS_HOST=ws://YOUR_TEST_HOST \
  -e PERF_ROOM_ID=4 \
  -e PERF_MEMBERS=500 \
  -e PERF_SENDERS=10 \
  -e PERF_MSG_RATE=20 \
  --summary-export=single-500.json perf7/k6-broadcast.js
~~~

PowerShell：

~~~powershell
$env:K6_PROMETHEUS_RW_SERVER_URL = "http://YOUR_TEST_HOST:9090/api/v1/write"
$env:K6_PROMETHEUS_RW_TREND_STATS = "p(50),p(95),p(99),max"
k6 run -o experimental-prometheus-rw `
  -e PERF_TEST_ID=single-500-r1 -e PERF_TOPOLOGY=single `
  -e PERF_WS_HOST=ws://YOUR_TEST_HOST -e PERF_ROOM_ID=4 `
  -e PERF_MEMBERS=500 -e PERF_SENDERS=10 -e PERF_MSG_RATE=20 `
  --summary-export=single-500.json perf7/k6-broadcast.js
~~~

三实例用完全相同参数，导出 triple-500.json。分别测试 100 / 500 / 1000 / 2000 用户，每档至少三轮。
三实例测试将 `PERF_TOPOLOGY` 改为 `triple`，每轮必须使用唯一的 `PERF_TEST_ID`，例如
`triple-500-r1`。这些标签会随 k6 指标写入 Prometheus，避免不同轮次的数据混在一起。
脚本每 VU 只建连一次，预热默认 30 秒，统一发送窗口 60 秒，最后留 10 秒收尾。
connected_before_send 必须为 100%；否则本轮不能用于完整触达比较，应延长预热或降低负载。
确认系统会回显给发送者后，预期触达 = messages_sent × members；与 unique_deliveries 对比。若排除发送者则使用 members - 1。
重复触达和提前断连必须为零，不能只看延迟。高扇出时负载机去重集合也会消耗内存，需要同步观察负载机资源。

## 对照口径

1. 单份 1 CPU 对三份各 1 CPU：增加资源后的扩容收益；宿主机必须有余量。
2. 单份 3 CPU 对三份各 1 CPU：相同总 CPU 配额下比较进程扩展效率，保持总内存相同。
3. 固定 Kafka、Redis、数据库、Agent 配置；保持测试账号、消息大小、速率和网络一致。
4. 报告记录每实例连接数、CPU、内存、P95/P99、成功率、缺失/重复、Kafka lag，以及压测机 CPU。
5. 吞吐扩展效率 = 三实例最大达标吞吐 / 单实例最大达标吞吐 / 3。达标标准必须预先相同。

## Grafana 看板

Prometheus 已启用 remote-write 接收端点，k6 使用 `-o experimental-prometheus-rw` 后，客户端测试指标会直接进入 Grafana 数据源。进入：

~~~text
http://YOUR_TEST_HOST:3000
Dashboards -> LAN IM -> LAN IM Gateway 压测
~~~

看板包含：连接就绪率、消息发送/唯一触达、重复投递、Socket 错误、投递 P50/P95/P99、三个 Gateway 的连接分布、吞吐、TaskPool、CPU 和内存。先选择本轮 `testid`，再比较 `single` 与 `triple`。

`9090/api/v1/write` 没有业务鉴权，只应在受控压测网络开放；公网服务器应通过安全组限制 9090 来源 IP，或通过 SSH 隧道写入。

## 回到普通部署

~~~sh
docker compose -f docker-compose.yml -f docker-compose.gateway-perf.yml stop gateway-2 gateway-3
docker compose up -d --force-recreate --no-deps backend nginx prometheus grafana
~~~

只停止额外实例，不删除数据卷。切换或停止实例会中断其现有连接，应安排在压测轮次之间。
