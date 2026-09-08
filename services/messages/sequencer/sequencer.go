// Package sequencer 提供单进程、内存状态的群消息编号器，不包含崩溃恢复。
package sequencer

import (
	"fmt"
	"sync"
	"time"

	protocol "lan-im-go/contracts/events"
)

type requestKey struct {
	sender int64
	client string
}

// Sequencer 必须在一个实例中使用；锁保证同群并发输入也不会重复分配序号。
type Sequencer struct {
	mu       sync.Mutex
	rooms    map[int64]int64
	requests map[requestKey]protocol.MessageEnvelope
	lastID   int64
}

func New() *Sequencer {
	return &Sequencer{rooms: make(map[int64]int64), requests: make(map[requestKey]protocol.MessageEnvelope)}
}

// Assign 重试返回第一次接受的完整消息，避免重试时间或内容改变正式结果。
func (s *Sequencer) Assign(input protocol.MessageEnvelope) (protocol.MessageEnvelope, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if input.RoomID <= 0 || input.SenderID <= 0 || input.ClientMsgID == "" || len(input.ClientMsgID) > 64 {
		return protocol.MessageEnvelope{}, fmt.Errorf("消息身份或客户端消息 ID 非法")
	}
	key := requestKey{input.SenderID, input.ClientMsgID}
	if old, ok := s.requests[key]; ok {
		if old.RoomID != input.RoomID || old.Content != input.Content || old.Type != input.Type {
			return protocol.MessageEnvelope{}, fmt.Errorf("同一客户端消息 ID 携带不同内容或房间")
		}
		return old, nil
	}
	// 单实例时间序列 ID：低 12 位保留序列空间，时钟回拨时继续逻辑递增。
	// 不作为跨进程雪花 ID 使用；本版本明确不支持编号器重启或多实例。
	id := (time.Now().UnixMilli() - 1750000000000) << 12
	if id <= s.lastID {
		id = s.lastID + 1
	}
	s.lastID = id
	s.rooms[input.RoomID]++
	input.MessageID = id
	input.RoomSeq = s.rooms[input.RoomID]
	s.requests[key] = input
	return input, nil
}
