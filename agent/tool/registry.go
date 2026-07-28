package tool

import (
	"encoding/json"
	"lan-im-go/agent/llm"
)

// Handler 函数调用处理器接口
type Handler interface {
	Name() string
	Definition() llm.FunctionDef
	Handle(args json.RawMessage) (string, error)
}

// Registry 函数注册表，每个 Agent / Chunker 持有独立实例
type Registry struct {
	handlers map[string]Handler
	defs     []llm.Tool
}

// NewRegistry 创建注册表
func NewRegistry() *Registry {
	return &Registry{handlers: make(map[string]Handler)}
}

// Register 注册工具
func (r *Registry) Register(h Handler) {
	r.handlers[h.Name()] = h
	r.defs = append(r.defs, llm.Tool{
		Type:     "function",
		Function: h.Definition(),
	})
}

// Dispatch 根据名称分发调用
func (r *Registry) Dispatch(name string, args json.RawMessage) (string, error) {
	h, ok := r.handlers[name]
	if !ok {
		return "", nil
	}
	return h.Handle(args)
}

// AllTools 返回所有已注册的工具定义
func (r *Registry) AllTools() []llm.Tool { return r.defs }
