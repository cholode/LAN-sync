import ws from "k6/ws";
import http from "k6/http";
import exec from "k6/execution";
import { sleep } from "k6";
import { Counter, Gauge, Trend } from "k6/metrics";

// ============================================================================
// 自定义物理指标探针
// ============================================================================
const msgSentTotal  = new Counter("custom_msg_sent_total");
const msgRecvTotal  = new Counter("custom_msg_recv_total");
const wsConnSuccess = new Counter("custom_ws_connect_success");
const httpReqFailed = new Counter("custom_http_req_failed");
const onlineUsers   = new Gauge("custom_online_user_count");
const msgLatency    = new Trend("custom_msg_latency_ms", true); 

// ============================================================================
// 压测规模与战术编排
// ============================================================================
const TOTAL_USERS     = 100;
const MSG_INTERVAL_MS = 1000;
const SENDER_COUNT    = 10;    // 前 10 个 VU 为火力输出，其余 90 个纯听众

const STAGE_RAMP_UP   = 5; 
const STAGE_FIRE      = 120;
const STAGE_RAMP_DOWN = 5; 

const BASE_URL = "http://127.0.0.1:8080/api/v1";
const WS_URL   = "ws://127.0.0.1:8080/api/v1/ws";

// ============================================================================
// K6 引擎全局配置
// ============================================================================
export const options = {
  scenarios: {
    im_websocket_load: {
      executor: "ramping-vus",
      startVUs: 0,
      stages: [
        { duration: `${STAGE_RAMP_UP}s`,   target: TOTAL_USERS },
        { duration: `${STAGE_FIRE}s`,      target: TOTAL_USERS },
        { duration: `${STAGE_RAMP_DOWN}s`, target: 0 },
      ],
      gracefulRampDown: "5s",
      exec: "vuMain",
    },
  },
  discardResponseBodies: true, // 物理级内存优化
};

// 【已彻底抹除 handleSummary，将控制权交还给 K6 官方报表引擎】

// ============================================================================
// 虚拟用户 (VU) 主逻辑闭环
// ============================================================================
export function vuMain() {
  const vuId = exec.vu.idInTest;
  const scenarioStart = exec.scenario.startTime;
  const ROOM_ID = 1; 

  const fireStartMs = scenarioStart + STAGE_RAMP_UP * 1000;
  const fireEndMs   = scenarioStart + (STAGE_RAMP_UP + STAGE_FIRE) * 1000;

  // ---------- 1. HTTP 鉴权层穿透 ----------
  const loginRes = http.post(
    `${BASE_URL}/login`, 
    JSON.stringify({ username: `vu_${vuId}`, password: "pass123" }), 
    { 
      headers: { "Content-Type": "application/json" },
      responseType: "text" 
    }
  );
  
  if (loginRes.status !== 200) {
    httpReqFailed.add(1);
    sleep(1);
    return;
  }
  
  let token = "";
  try {
    token = JSON.parse(loginRes.body).token;
  } catch(e) {
    httpReqFailed.add(1);
    sleep(1);
    return;
  }

  // ---------- 2. WebSocket 长连接接驳 ----------
  const wsUrl = `${WS_URL}?token=${token}`;
  const isSender = vuId <= SENDER_COUNT;   

  ws.connect(wsUrl, null, function (socket) {
    let vuMsgSeq = 0;
    let msgTimer = null;
    let disconnectTimer = null;

    socket.on("open", function () {
      wsConnSuccess.add(1);
      onlineUsers.add(1);

      const now = Date.now();
      
      // 【终极物理防爆盾】：强行向上取整，并兜底最小值为 1ms，彻底绞杀 Go 引擎的 0.00 宕机
      const delayToFire = Math.max(1, Math.ceil(fireStartMs - now));
      const delayToClose = Math.max(1, Math.ceil(fireEndMs - now));

      // 真实时间校验：如果已经过了撤退时间，直接物理斩断
      if ((fireEndMs - now) <= 0) {
        socket.close();
        return; 
      }

      disconnectTimer = socket.setTimeout(function() {
        if (msgTimer) socket.clearInterval(msgTimer);
        socket.close();
      }, delayToClose);

      if (!isSender) return;   

      const beginFiring = function () {
        msgTimer = socket.setInterval(function () {
          const sendTime = Date.now();
          const globalMsgId = `${vuId}-${++vuMsgSeq}-${sendTime}`;
          socket.send(JSON.stringify({
            room_id: ROOM_ID,
            content: `#${vuId}`,
            client_msg_id: globalMsgId,
          }));
          msgSentTotal.add(1);
        }, MSG_INTERVAL_MS);
      };

      // 真实时间校验：如果还没到开火时间，挂载定时炸弹，否则立刻开火
      if ((fireStartMs - now) > 0) {
        socket.setTimeout(beginFiring, delayToFire);
      } else {
        beginFiring();
      }
    });

    socket.on("message", function (raw) {
      msgRecvTotal.add(1);
      // 【架构级探针突围】：绕过脆弱的 JSON 嵌套解析，用正则直接从内存字符串中强抠出时间戳
      const match = raw.match(/"ts":(\d+)/);
      if (match && match[1]) {
        msgLatency.add(Date.now() - parseInt(match[1], 10));
      }
    });

    socket.on("close", function () {
      onlineUsers.add(-1);
      if (msgTimer) socket.clearInterval(msgTimer);
      if (disconnectTimer) socket.clearTimeout(disconnectTimer);
    });

    socket.on("error", function () {});
  });
}