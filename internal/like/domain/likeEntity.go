package domain

import (
	mq "webookApp/internal/mq/redmq"
)

type LikeProducer struct {
	Producer *mq.MqProducer
}

type ResultResponde struct {
	IsNew       bool  `json:"isNew"`
	TotalNumber int64 `json:"totalNumber"`
}

type LikeMqConsumerAllConfig struct {
	NormalConfig *mq.MqConsumerConfig
	RetryConfig  *mq.MqRetryConsumerConfig
}

type LikeConsumer struct {
	Consumer *mq.MqConsumer
}

func (l *LikeConsumer) Stop() {
	l.Consumer.Stop()
}
