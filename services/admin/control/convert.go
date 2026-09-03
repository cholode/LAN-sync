package admincontrol

import (
	"time"

	admincontrolv1 "lan-im-go/proto/admincontrol/v1"
	"lan-im-go/services/gateway/websocket"
	"lan-im-go/shared/observability/metrics"
)

func connectionSnapshotsToProto(items []core.ConnectionSnapshot) []*admincontrolv1.ConnectionSnapshot {
	out := make([]*admincontrolv1.ConnectionSnapshot, 0, len(items))
	for _, item := range items {
		out = append(out, &admincontrolv1.ConnectionSnapshot{
			UserId:            item.UserID,
			Username:          item.Username,
			ConnectionId:      item.ConnectionID,
			RemoteIp:          item.RemoteIP,
			UserAgent:         item.UserAgent,
			ClientVersion:     item.ClientVersion,
			ConnectedAtUnixMs: item.ConnectedAt.UnixMilli(),
			LastReadAtUnixMs:  item.LastReadAt.UnixMilli(),
			LastWriteAtUnixMs: item.LastWriteAt.UnixMilli(),
			SendQueueLen:      int32(item.SendQueueLen),
			RoomIds:           append([]int64(nil), item.RoomIDs...),
		})
	}
	return out
}

func protoSnapshotsToConnections(items []*admincontrolv1.ConnectionSnapshot) []core.ConnectionSnapshot {
	out := make([]core.ConnectionSnapshot, 0, len(items))
	for _, item := range items {
		if item == nil {
			continue
		}
		out = append(out, core.ConnectionSnapshot{
			UserID:        item.GetUserId(),
			Username:      item.GetUsername(),
			ConnectionID:  item.GetConnectionId(),
			RemoteIP:      item.GetRemoteIp(),
			UserAgent:     item.GetUserAgent(),
			ClientVersion: item.GetClientVersion(),
			ConnectedAt:   time.UnixMilli(item.GetConnectedAtUnixMs()),
			LastReadAt:    time.UnixMilli(item.GetLastReadAtUnixMs()),
			LastWriteAt:   time.UnixMilli(item.GetLastWriteAtUnixMs()),
			SendQueueLen:  int(item.GetSendQueueLen()),
			RoomIDs:       append([]int64(nil), item.GetRoomIds()...),
		})
	}
	return out
}

func runtimeSnapshotToProto(snapshot metrics.RuntimeSnapshot) *admincontrolv1.RuntimeSnapshot {
	return &admincontrolv1.RuntimeSnapshot{
		Websocket: &admincontrolv1.WebSocketRuntime{
			CurrentConnections:          snapshot.WebSocket.CurrentConnections,
			EstablishedTotal:            snapshot.WebSocket.EstablishedTotal,
			ClosedTotal:                 snapshot.WebSocket.ClosedTotal,
			AbnormalClosedTotal:         snapshot.WebSocket.AbnormalClosedTotal,
			AverageConnectionDurationMs: snapshot.WebSocket.AverageConnectionDurationMS,
			ReadMessagesPerMinute:       snapshot.WebSocket.ReadMessagesPerMinute,
			WriteMessagesPerMinute:      snapshot.WebSocket.WriteMessagesPerMinute,
			SendQueueBacklog:            snapshot.WebSocket.SendQueueBacklog,
			SlowClients:                 snapshot.WebSocket.SlowClients,
		},
		Golang: &admincontrolv1.GoRuntime{
			Goroutines:    int64(snapshot.Golang.Goroutines),
			Gomaxprocs:    int64(snapshot.Golang.GOMAXPROCS),
			HeapAlloc:     snapshot.Golang.HeapAlloc,
			HeapSys:       snapshot.Golang.HeapSys,
			GcCount:       snapshot.Golang.GCCount,
			LastGcUnix:    snapshot.Golang.LastGCUnix,
			GcPauseNs:     snapshot.Golang.GCPauseNS,
			UptimeSeconds: snapshot.Golang.UptimeSeconds,
		},
		Api: &admincontrolv1.APIRuntime{
			Qps_1M:           snapshot.API.QPS1m,
			Qps_5M:           snapshot.API.QPS5m,
			AverageLatencyMs: snapshot.API.AverageLatencyMS,
			P50LatencyMs:     snapshot.API.P50LatencyMS,
			P95LatencyMs:     snapshot.API.P95LatencyMS,
			P99LatencyMs:     snapshot.API.P99LatencyMS,
			Status_4Xx:       snapshot.API.Status4xx,
			Status_5Xx:       snapshot.API.Status5xx,
			ErrorRate:        snapshot.API.ErrorRate,
		},
	}
}

func protoToRuntimeSnapshot(snapshot *admincontrolv1.RuntimeSnapshot) metrics.RuntimeSnapshot {
	if snapshot == nil {
		return metrics.RuntimeSnapshot{}
	}
	ws := snapshot.GetWebsocket()
	golang := snapshot.GetGolang()
	api := snapshot.GetApi()
	return metrics.RuntimeSnapshot{
		WebSocket: metrics.WebSocketRuntime{
			CurrentConnections:          ws.GetCurrentConnections(),
			EstablishedTotal:            ws.GetEstablishedTotal(),
			ClosedTotal:                 ws.GetClosedTotal(),
			AbnormalClosedTotal:         ws.GetAbnormalClosedTotal(),
			AverageConnectionDurationMS: ws.GetAverageConnectionDurationMs(),
			ReadMessagesPerMinute:       ws.GetReadMessagesPerMinute(),
			WriteMessagesPerMinute:      ws.GetWriteMessagesPerMinute(),
			SendQueueBacklog:            ws.GetSendQueueBacklog(),
			SlowClients:                 ws.GetSlowClients(),
		},
		Golang: metrics.GoRuntime{
			Goroutines:    int(golang.GetGoroutines()),
			GOMAXPROCS:    int(golang.GetGomaxprocs()),
			HeapAlloc:     golang.GetHeapAlloc(),
			HeapSys:       golang.GetHeapSys(),
			GCCount:       golang.GetGcCount(),
			LastGCUnix:    golang.GetLastGcUnix(),
			GCPauseNS:     golang.GetGcPauseNs(),
			UptimeSeconds: golang.GetUptimeSeconds(),
		},
		API: metrics.APIRuntime{
			QPS1m:            api.GetQps_1M(),
			QPS5m:            api.GetQps_5M(),
			AverageLatencyMS: api.GetAverageLatencyMs(),
			P50LatencyMS:     api.GetP50LatencyMs(),
			P95LatencyMS:     api.GetP95LatencyMs(),
			P99LatencyMS:     api.GetP99LatencyMs(),
			Status4xx:        api.GetStatus_4Xx(),
			Status5xx:        api.GetStatus_5Xx(),
			ErrorRate:        api.GetErrorRate(),
		},
	}
}

func agentSnapshotToProto(snapshot metrics.AgentRuntimeSnapshot) *admincontrolv1.AgentRuntimeSnapshot {
	failures := make([]*admincontrolv1.AgentFailureEvent, 0, len(snapshot.RecentFailures))
	for _, item := range snapshot.RecentFailures {
		failures = append(failures, &admincontrolv1.AgentFailureEvent{
			TimeUnixMs: item.Time.UnixMilli(),
			Model:      item.Model,
			RequestId:  item.RequestID,
			ErrorType:  item.ErrorType,
			HttpStatus: int32(item.HTTPStatus),
			LatencyMs:  item.LatencyMS,
			Retries:    int32(item.Retries),
		})
	}
	return &admincontrolv1.AgentRuntimeSnapshot{
		CallsToday:           snapshot.CallsToday,
		CurrentRequests:      snapshot.CurrentRequests,
		SuccessRate:          snapshot.SuccessRate,
		FailureRate:          snapshot.FailureRate,
		AverageResponseMs:    snapshot.AverageResponseMS,
		P95ResponseMs:        snapshot.P95ResponseMS,
		TotalTokens:          snapshot.TotalTokens,
		InputTokens:          snapshot.InputTokens,
		OutputTokens:         snapshot.OutputTokens,
		AverageTokensPerCall: snapshot.AverageTokensPerCall,
		ToolCalls:            snapshot.ToolCalls,
		ToolSuccessRate:      snapshot.ToolSuccessRate,
		RagCalls:             snapshot.RAGCalls,
		TimeToolCalls:        snapshot.TimeToolCalls,
		ModerationCalls:      snapshot.ModerationCalls,
		EmbeddingCalls:       snapshot.EmbeddingCalls,
		EmbeddingFailures:    snapshot.EmbeddingFailures,
		RecentFailures:       failures,
	}
}

func protoToAgentSnapshot(snapshot *admincontrolv1.AgentRuntimeSnapshot) metrics.AgentRuntimeSnapshot {
	if snapshot == nil {
		return metrics.AgentRuntimeSnapshot{}
	}
	failures := make([]metrics.AgentFailureEvent, 0, len(snapshot.GetRecentFailures()))
	for _, item := range snapshot.GetRecentFailures() {
		if item == nil {
			continue
		}
		failures = append(failures, metrics.AgentFailureEvent{
			Time:       time.UnixMilli(item.GetTimeUnixMs()),
			Model:      item.GetModel(),
			RequestID:  item.GetRequestId(),
			ErrorType:  item.GetErrorType(),
			HTTPStatus: int(item.GetHttpStatus()),
			LatencyMS:  item.GetLatencyMs(),
			Retries:    int(item.GetRetries()),
		})
	}
	return metrics.AgentRuntimeSnapshot{
		CallsToday:           snapshot.GetCallsToday(),
		CurrentRequests:      snapshot.GetCurrentRequests(),
		SuccessRate:          snapshot.GetSuccessRate(),
		FailureRate:          snapshot.GetFailureRate(),
		AverageResponseMS:    snapshot.GetAverageResponseMs(),
		P95ResponseMS:        snapshot.GetP95ResponseMs(),
		TotalTokens:          snapshot.GetTotalTokens(),
		InputTokens:          snapshot.GetInputTokens(),
		OutputTokens:         snapshot.GetOutputTokens(),
		AverageTokensPerCall: snapshot.GetAverageTokensPerCall(),
		ToolCalls:            snapshot.GetToolCalls(),
		ToolSuccessRate:      snapshot.GetToolSuccessRate(),
		RAGCalls:             snapshot.GetRagCalls(),
		TimeToolCalls:        snapshot.GetTimeToolCalls(),
		ModerationCalls:      snapshot.GetModerationCalls(),
		EmbeddingCalls:       snapshot.GetEmbeddingCalls(),
		EmbeddingFailures:    snapshot.GetEmbeddingFailures(),
		RecentFailures:       failures,
	}
}
