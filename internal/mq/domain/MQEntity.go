package domain

type MqRecorde struct {
	Action      string
	DrivingID   int64
	PassivityID int64
	Ts          int64
	MessagesID  []string
}

type MqConsumerRecord struct {
	DrivingID   int64
	PassivityID int64
	Status      int
	TimeStamp   int64
	MessagesID  []string
}
