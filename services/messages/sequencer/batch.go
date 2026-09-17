package sequencer

import (
	"context"
	"errors"
	"fmt"
	"log"
	"time"

	"github.com/segmentio/kafka-go"
	protocol "lan-im-go/contracts/events"
)

// BatchOptions bounds the application batch. MaxBytes counts input key/value
// bytes; the Kafka writer must also bound its encoded output request size.
type BatchOptions struct {
	MaxMessages int
	MaxBytes    int
	MaxWait     time.Duration
}

type BatchWriteFunc func(context.Context, ...kafka.Message) error

// RunBatched keeps one batch in flight. Numbering follows input order; output
// retries reuse identical IDs, and offsets advance only after the entire output
// batch is acknowledged. Like Run, it does not provide restart recovery.
func RunBatched(ctx context.Context, reader Reader, write BatchWriteFunc, state *Sequencer, opts BatchOptions) error {
	return consumeBatches(ctx, reader, opts, func(records []kafka.Message) error {
		outputs := make([]kafka.Message, 0, len(records))
		for _, record := range records {
			input, err := protocol.Unmarshal(record.Value)
			if err != nil {
				return fmt.Errorf("待处理消息解析失败 offset=%d: %w", record.Offset, err)
			}
			message, err := state.Assign(input)
			if err != nil {
				log.Printf("拒绝待处理消息 offset=%d: %v", record.Offset, err)
				continue
			}
			value, err := protocol.Marshal(message)
			if err != nil {
				return err
			}
			outputs = append(outputs, kafka.Message{Key: []byte(fmt.Sprint(message.RoomID)), Value: value})
		}
		if len(outputs) > 0 {
			if err := Retry(ctx, func() error { return write(ctx, outputs...) }); err != nil {
				return err
			}
		}
		return nil
	})
}

func consumeBatches(ctx context.Context, reader Reader, opts BatchOptions, handle func([]kafka.Message) error) error {
	if opts.MaxMessages < 1 || opts.MaxMessages > 10000 || opts.MaxBytes < 1 || opts.MaxWait <= 0 {
		return fmt.Errorf("invalid sequencer batch bounds")
	}
	var pending *kafka.Message
	for {
		var first kafka.Message
		var err error
		if pending != nil {
			first = *pending
			pending = nil
		} else {
			first, err = reader.FetchMessage(ctx)
		}
		if err != nil {
			return err
		}
		if len(first.Key)+len(first.Value) > opts.MaxBytes {
			return fmt.Errorf("input message exceeds batch byte limit")
		}
		records := make([]kafka.Message, 1, min(opts.MaxMessages, 64))
		records[0] = first
		bytes := len(first.Key) + len(first.Value)
		batchCtx, stop := context.WithTimeout(ctx, opts.MaxWait)
		var terminal error
		for len(records) < opts.MaxMessages && bytes < opts.MaxBytes {
			if batchCtx.Err() != nil {
				break
			}
			next, fetchErr := reader.FetchMessage(batchCtx)
			if fetchErr != nil {
				if !errors.Is(fetchErr, context.DeadlineExceeded) || batchCtx.Err() == nil {
					terminal = fetchErr
				}
				break
			}
			if bytes+len(next.Key)+len(next.Value) > opts.MaxBytes {
				pending = &next
				break
			}
			records = append(records, next)
			bytes += len(next.Key) + len(next.Value)
		}
		stop()
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if err := handle(records); err != nil {
			return err
		}
		if err := Retry(ctx, func() error { return reader.CommitMessages(ctx, records...) }); err != nil {
			return err
		}
		if terminal != nil {
			return terminal
		}
	}
}
