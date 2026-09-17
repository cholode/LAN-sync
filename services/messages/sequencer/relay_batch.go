package sequencer

import (
	"context"
	"fmt"

	"github.com/segmentio/kafka-go"
	protocol "lan-im-go/contracts/events"
)

type RelayMessage struct {
	Message protocol.MessageEnvelope
	Value   []byte
}

// RelayBatched publishes one ordered batch at a time, then commits its offsets.
// An uncertain publish result retries the whole batch with unchanged IDs. A retry
// can duplicate deliveries; subscribers must deduplicate by business message ID.
func RelayBatched(ctx context.Context, reader Reader, publish func(context.Context, []RelayMessage) error, opts BatchOptions) error {
	return consumeBatches(ctx, reader, opts, func(records []kafka.Message) error {
		messages := make([]RelayMessage, 0, len(records))
		for _, record := range records {
			message, err := protocol.Unmarshal(record.Value)
			if err != nil {
				return fmt.Errorf("正式消息解析失败 offset=%d: %w", record.Offset, err)
			}
			if message.RoomID <= 0 || message.MessageID <= 0 || message.RoomSeq <= 0 {
				return fmt.Errorf("正式消息缺少房间或编号 offset=%d", record.Offset)
			}
			messages = append(messages, RelayMessage{Message: message, Value: record.Value})
		}
		return Retry(ctx, func() error { return publish(ctx, messages) })
	})
}
