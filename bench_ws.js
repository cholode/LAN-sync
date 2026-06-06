import ws from "k6/ws";
import http from "k6/http";
import exec from "k6/execution";
import { sleep } from "k6";
import { Counter, Gauge, Trend } from "k6/metrics";

const msgSentTotal  = new Counter("custom_msg_sent_total");
const msgRecvTotal  = new Counter("custom_msg_recv_total");
const wsConnSuccess = new Counter("custom_ws_connect_success");
const httpReqFailed = new Counter("custom_http_req_failed");
const onlineUsers   = new Gauge("custom_online_user_count");
const msgLatency    = new Trend("custom_msg_latency_ms", true);

const diagLoginFail   = new Counter("diag_login_fail");
const diagWsUpgrade   = new Counter("diag_ws_upgrade_fail");
const diagPremClose   = new Counter("diag_premature_close");
const diagNormalClose = new Counter("diag_normal_close");

const TOTAL_USERS     = 10;
const MSG_INTERVAL_MS = 1;
const SENDER_COUNT    = 10;

const STAGE_RAMP_UP   = 5;
const STAGE_FIRE      = 120;
const STAGE_RAMP_DOWN = 5;

const BASE_URL = "http://127.0.0.1:8080/api/v1";
const WS_URL   = "ws://127.0.0.1:8080/api/v1/ws";

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
  discardResponseBodies: true,
};

export function handleSummary(data) {
  const val = (name) => {
    const m = data.metrics[name];
    return m && m.values ? m.values.count : 0;
  };

  console.log("\n========== 断连诊断分布 ==========");
  console.log(`登录失败         : ${val("diag_login_fail")}`);
  console.log(`WS 握手失败      : ${val("diag_ws_upgrade_fail")}`);
  console.log(`提前断连(异常)   : ${val("diag_premature_close")}`);
  console.log(`正常关闭(计划内) : ${val("diag_normal_close")}`);
  console.log("==================================\n");
}

// 【防重连】标记每个 VU 是否已经完成本轮测试
const vuDone = new Set();

export function vuMain() {
  const vuId = exec.vu.idInTest;

  // 已经正常关闭过，不再重连
  if (vuDone.has(vuId)) {
    sleep(9999);
    return;
  }

  const scenarioStart = exec.scenario.startTime;
  const ROOM_ID = 1;

  const fireStartMs = scenarioStart + STAGE_RAMP_UP * 1000;
  const fireEndMs   = scenarioStart + (STAGE_RAMP_UP + STAGE_FIRE) * 1000;

  const loginRes = http.post(
    `${BASE_URL}/login`,
    JSON.stringify({ username: `vu_${vuId}`, password: "pass123" }),
    { headers: { "Content-Type": "application/json" }, responseType: "text" }
  );

  if (loginRes.status !== 200) {
    httpReqFailed.add(1);
    diagLoginFail.add(1);
    sleep(1);
    return;
  }

  let token = "";
  try {
    token = JSON.parse(loginRes.body).token;
  } catch(e) {
    httpReqFailed.add(1);
    diagLoginFail.add(1);
    sleep(1);
    return;
  }

  const wsUrl = `${WS_URL}?token=${token}`;
  const isSender = vuId <= SENDER_COUNT;

  ws.connect(wsUrl, null, function (socket) {
    let vuMsgSeq = 0;
    let msgTimer = null;
    let disconnectTimer = null;
    let wsOpened = false;
    let plannedClose = false;

    socket.on("open", function () {
      wsConnSuccess.add(1);
      onlineUsers.add(1);
      wsOpened = true;

      const now = Date.now();
      const delayToFire = Math.max(1, Math.ceil(fireStartMs - now));
      const delayToClose = Math.max(1, Math.ceil(fireEndMs - now));

      if ((fireEndMs - now) <= 0) {
        vuDone.add(vuId);
        socket.close();
        return;
      }

      disconnectTimer = socket.setTimeout(function() {
        plannedClose = true;
        vuDone.add(vuId);
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

      if ((fireStartMs - now) > 0) {
        socket.setTimeout(beginFiring, delayToFire);
      } else {
        beginFiring();
      }
    });

    socket.on("message", function (raw) {
      msgRecvTotal.add(1);
      const match = raw.match(/"ts":(\d+)/);
      if (match && match[1]) {
        msgLatency.add(Date.now() - parseInt(match[1], 10));
      }
    });

    socket.on("close", function (code) {
      onlineUsers.add(-1);
      if (msgTimer) socket.clearInterval(msgTimer);
      if (disconnectTimer) socket.clearTimeout(disconnectTimer);

      if (!wsOpened) return;

      if (plannedClose || code === 1001) {
        diagNormalClose.add(1);
      } else {
        diagPremClose.add(1);
        console.log(`[DIAG] VU#${vuId} 提前断连 close=${code}`);
      }
    });

    socket.on("error", function () {
      if (!wsOpened) {
        diagWsUpgrade.add(1);
      }
    });
  });
}
