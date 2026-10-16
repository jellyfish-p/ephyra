package stream

import (
	"context"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"github.com/jellyfish-p/ephyra/server/internal/outbox"
	"github.com/jellyfish-p/ephyra/server/internal/persist"
	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"
)

type Consumer struct {
	DB       *gorm.DB
	Redis    *redis.Client
	Stream   string
	Group    string
	Consumer string
	OnReady  func(ctx context.Context) error
}

func (c *Consumer) streamName() string {
	if c.Stream != "" {
		return c.Stream
	}
	return outbox.Stream
}

func (c *Consumer) Run(ctx context.Context) error {
	if c.Redis == nil {
		return nil
	}
	_ = c.Redis.XGroupCreateMkStream(ctx, c.streamName(), c.Group, "0").Err()
	for {
		select {
		case <-ctx.Done():
			return nil
		default:
		}
		streams, err := c.Redis.XReadGroup(ctx, &redis.XReadGroupArgs{
			Group:    c.Group,
			Consumer: c.Consumer,
			Streams:  []string{c.streamName(), ">"},
			Count:    20,
			Block:    2 * time.Second,
		}).Result()
		if err != nil && err != redis.Nil {
			if ctx.Err() != nil {
				return nil
			}
			slog.Error("stream", "err", err)
			time.Sleep(time.Second)
			continue
		}
		for _, st := range streams {
			for _, msg := range st.Messages {
				if err := c.handle(ctx, msg); err != nil {
					slog.Error("stream handle", "id", msg.ID, "err", err)
					continue
				}
				if err := c.Redis.XAck(ctx, c.streamName(), c.Group, msg.ID).Err(); err != nil && ctx.Err() == nil {
					slog.Error("stream ack", "id", msg.ID, "err", err)
				}
			}
		}
	}
}

func (c *Consumer) handle(ctx context.Context, msg redis.XMessage) error {
	raw, _ := msg.Values["event_id"].(string)
	eventID, err := uuid.Parse(raw)
	if err != nil {
		return nil
	}
	now := time.Now().UTC()
	ins := persist.StreamConsumption{EventID: eventID, ProcessedAt: now}
	if err := c.DB.WithContext(ctx).Where("event_id = ?", eventID).FirstOrCreate(&ins).Error; err != nil {
		return err
	}
	if ApplyEvent(true).RunSchedule && c.OnReady != nil {
		return c.OnReady(ctx)
	}
	return nil
}
