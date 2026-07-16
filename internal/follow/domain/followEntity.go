package domain

import (
	mq "webookApp/internal/mq/redmq"
)

type FollowProducer struct {
	Producer *mq.MqProducer
}

type ResultResponde struct {
	IsNew       bool  `json:"isNew"`
	TotalNumber int64 `json:"totalNumber"`
}

type FollowConsumerAllConfig struct {
	NormalConfig *mq.MqConsumerConfig
	RetryConfig  *mq.MqRetryConsumerConfig
}

type FollowConsumer struct {
	Consumer *mq.MqConsumer
}

func (f *FollowConsumer) Stop() {
	f.Consumer.Stop()
}
