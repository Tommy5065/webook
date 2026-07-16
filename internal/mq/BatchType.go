package mq

import (
	"context"
	"webookApp/internal/mq/domain"
)

// 实现不同消费者的批量分流接口
type MqRepository interface {
	Batch(ctx context.Context, records []*domain.MqRecorde) []interface{}
}
