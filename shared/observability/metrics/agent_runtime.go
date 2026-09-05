package metrics

import (
	"sync"
	"sync/atomic"
	"time"
)

var (
	agentInflight          int64
	agentTotal             int64
	agentSuccess           int64
	agentFailure           int64
	agentLatencySumMS      int64
	agentLatencyCount      int64
	agentToolCalls         int64
	agentToolSuccess       int64
	agentRAGCalls          int64
	agentTimeToolCalls     int64
	agentModerationCalls   int64
	agentEmbeddingCalls    int64
	agentEmbeddingFailures int64
	agentInputTokens       int64
	agentOutputTokens      int64
	agentLatencyWindow     = newLatencyWindow(1024)
	agentFailureMu         sync.Mutex
	agentRecentFailures    []AgentFailureEvent
)

// AgentFailureEvent 记录近期 Agent 失败信息，不保存提示词或 API Key。
type AgentFailureEvent struct {
	Time       time.Time `json:"time"`
	Model      string    `json:"model"`
	RequestID  string    `json:"request_id"`
	ErrorType  string    `json:"error_type"`
	HTTPStatus int       `json:"http_status"`
	LatencyMS  float64   `json:"latency_ms"`
	Retries    int       `json:"retries"`
}

// AgentRuntimeSnapshot 汇总 Agent 管理面板所需的运行指标。
type AgentRuntimeSnapshot struct {
	CallsToday           int64               `json:"calls_today"`
	CurrentRequests      int64               `json:"current_requests"`
	SuccessRate          float64             `json:"success_rate"`
	FailureRate          float64             `json:"failure_rate"`
	AverageResponseMS    float64             `json:"average_response_ms"`
	P95ResponseMS        float64             `json:"p95_response_ms"`
	TotalTokens          int64               `json:"total_tokens"`
	InputTokens          int64               `json:"input_tokens"`
	OutputTokens         int64               `json:"output_tokens"`
	AverageTokensPerCall float64             `json:"average_tokens_per_call"`
	ToolCalls            int64               `json:"tool_calls"`
	ToolSuccessRate      float64             `json:"tool_success_rate"`
	RAGCalls             int64               `json:"rag_calls"`
	TimeToolCalls        int64               `json:"time_tool_calls"`
	ModerationCalls      int64               `json:"moderation_calls"`
	EmbeddingCalls       int64               `json:"embedding_calls"`
	EmbeddingFailures    int64               `json:"embedding_failures"`
	RecentFailures       []AgentFailureEvent `json:"recent_failures"`
}

func recordAgentStarted() {
	atomic.AddInt64(&agentInflight, 1)
}

func recordAgentFinished(model, requestID string, start time.Time, err error) {
	atomic.AddInt64(&agentInflight, -1)
	atomic.AddInt64(&agentTotal, 1)
	latencyMS := float64(time.Since(start).Microseconds()) / 1000.0
	agentLatencyWindow.Add(latencyMS)
	atomic.AddInt64(&agentLatencySumMS, int64(latencyMS))
	atomic.AddInt64(&agentLatencyCount, 1)
	if err != nil {
		atomic.AddInt64(&agentFailure, 1)
		appendAgentFailure(AgentFailureEvent{
			Time:      time.Now(),
			Model:     model,
			RequestID: requestID,
			ErrorType: errorLabel(err),
			LatencyMS: latencyMS,
		})
	} else {
		atomic.AddInt64(&agentSuccess, 1)
	}
}

func appendAgentFailure(event AgentFailureEvent) {
	agentFailureMu.Lock()
	defer agentFailureMu.Unlock()
	if len(agentRecentFailures) >= 50 {
		copy(agentRecentFailures, agentRecentFailures[1:])
		agentRecentFailures[len(agentRecentFailures)-1] = event
		return
	}
	agentRecentFailures = append(agentRecentFailures, event)
}

// RecordAgentToolCall 记录一次工具调用及其结果。
func RecordAgentToolCall(success bool) {
	atomic.AddInt64(&agentToolCalls, 1)
	if success {
		atomic.AddInt64(&agentToolSuccess, 1)
	}
}

// RecordAgentRAGCall 记录一次 RAG 检索调用。
func RecordAgentRAGCall() {
	atomic.AddInt64(&agentRAGCalls, 1)
}

// RecordAgentTimeToolCall 记录一次时间工具调用。
func RecordAgentTimeToolCall() {
	atomic.AddInt64(&agentTimeToolCalls, 1)
}

// RecordAgentModerationCall 记录一次内容审核调用。
func RecordAgentModerationCall() {
	atomic.AddInt64(&agentModerationCalls, 1)
}

// RecordEmbeddingCall 记录一次向量化调用及其结果。
func RecordEmbeddingCall(success bool) {
	atomic.AddInt64(&agentEmbeddingCalls, 1)
	if !success {
		atomic.AddInt64(&agentEmbeddingFailures, 1)
	}
}

// RecordAgentTokenUsage 累计 Agent 的 Token 用量。
func RecordAgentTokenUsage(inputTokens, outputTokens int64) {
	atomic.AddInt64(&agentInputTokens, inputTokens)
	atomic.AddInt64(&agentOutputTokens, outputTokens)
}

// AgentRuntimeSnapshotNow 返回当前 Agent 运行指标快照。
func AgentRuntimeSnapshotNow() AgentRuntimeSnapshot {
	total := atomic.LoadInt64(&agentTotal)
	var successRate, failureRate float64
	if total > 0 {
		successRate = float64(atomic.LoadInt64(&agentSuccess)) / float64(total)
		failureRate = float64(atomic.LoadInt64(&agentFailure)) / float64(total)
	}
	count := atomic.LoadInt64(&agentLatencyCount)
	var avg float64
	if count > 0 {
		avg = float64(atomic.LoadInt64(&agentLatencySumMS)) / float64(count)
	}
	input := atomic.LoadInt64(&agentInputTokens)
	output := atomic.LoadInt64(&agentOutputTokens)
	var avgTokens float64
	if total > 0 {
		avgTokens = float64(input+output) / float64(total)
	}
	toolTotal := atomic.LoadInt64(&agentToolCalls)
	var toolSuccessRate float64
	if toolTotal > 0 {
		toolSuccessRate = float64(atomic.LoadInt64(&agentToolSuccess)) / float64(toolTotal)
	}

	agentFailureMu.Lock()
	failures := make([]AgentFailureEvent, len(agentRecentFailures))
	copy(failures, agentRecentFailures)
	agentFailureMu.Unlock()

	return AgentRuntimeSnapshot{
		CallsToday:           total,
		CurrentRequests:      atomic.LoadInt64(&agentInflight),
		SuccessRate:          successRate,
		FailureRate:          failureRate,
		AverageResponseMS:    avg,
		P95ResponseMS:        agentLatencyWindow.Percentile(0.95),
		TotalTokens:          input + output,
		InputTokens:          input,
		OutputTokens:         output,
		AverageTokensPerCall: avgTokens,
		ToolCalls:            toolTotal,
		ToolSuccessRate:      toolSuccessRate,
		RAGCalls:             atomic.LoadInt64(&agentRAGCalls),
		TimeToolCalls:        atomic.LoadInt64(&agentTimeToolCalls),
		ModerationCalls:      atomic.LoadInt64(&agentModerationCalls),
		EmbeddingCalls:       atomic.LoadInt64(&agentEmbeddingCalls),
		EmbeddingFailures:    atomic.LoadInt64(&agentEmbeddingFailures),
		RecentFailures:       failures,
	}
}
