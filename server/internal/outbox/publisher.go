package outbox

import (
	"context"
	"log/slog"
	"time"

	"github.com/jellyfish-p/ephyra/server/internal/persist"
	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"
)

const Stream = "ephyra.events"

type Publisher struct {
	DB     *gorm.DB
	Redis  *redis.Client
	Stream string
}

func (p *Publisher) streamName() string {
	if p.Stream != "" {
		return p.Stream
	}
	return Stream
}

func (p *Publisher) Run(ctx context.Context) error {
	t := time.NewTicker(time.Second)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-t.C:
			if err := p.Flush(ctx); err != nil && ctx.Err() == nil {
				slog.Error("outbox", "err", err)
			}
		}
	}
}

func (p *Publisher) Flush(ctx context.Context) error {
	if p.Redis == nil {
		return nil
	}
	var events []persist.OutboxEvent
	if err := p.DB.WithContext(ctx).Where("published_at IS NULL").Order("created_at").Limit(100).Find(&events).Error; err != nil {
		return err
	}
	now := time.Now().UTC()
	for _, ev := range events {
		if err := p.Redis.XAdd(ctx, &redis.XAddArgs{
			Stream: p.streamName(),
			Values: StreamFields(ev),
		}).Err(); err != nil {
			return err
		}
		if err := p.DB.WithContext(ctx).Model(&ev).Update("published_at", now).Error; err != nil {
			return err
		}
	}
	return nil
}
