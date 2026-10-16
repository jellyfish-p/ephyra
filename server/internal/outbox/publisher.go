package outbox

import (
	"context"
	"log"
	"time"

	"github.com/jellyfish-p/ephyra/server/internal/persist"
	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"
)

const Stream = "ephyra.events"

type Publisher struct {
	DB    *gorm.DB
	Redis *redis.Client
}

func (p *Publisher) Run(ctx context.Context) {
	t := time.NewTicker(time.Second)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			if err := p.flush(ctx); err != nil {
				log.Printf("outbox: %v", err)
			}
		}
	}
}

func (p *Publisher) flush(ctx context.Context) error {
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
			Stream: Stream,
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
