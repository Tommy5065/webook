package snowflake

import (
	"time"

	sf "github.com/bwmarrin/snowflake"
	"go.uber.org/zap"
)

type SnowFlake struct {
	node *sf.Node
}

// NewSnowFlake 返回SnowFlake结构体
// epoch 传入起始的时间,格式："2006-01-02 MST"
// machineID 传入服务器/进程唯一编号
func NewSnowFlakeSetting(epoch string, machineID int64) *SnowFlake {
	st, err := time.Parse("2006-01-02 MST", epoch)
	if err != nil {
		zap.L().Fatal("解析epoch失败:", zap.Error(err))
	}
	sf.Epoch = int64(st.Nanosecond() / 1e6)
	sf.NodeBits = 5
	sf.StepBits = 3
	node, err := sf.NewNode(machineID)
	if err != nil {
		zap.L().Fatal("生成雪花ID服务器失败:", zap.Error(err))
	}
	return &SnowFlake{
		node: node,
	}
}

// GenerateID 生产雪花ID

func (snow *SnowFlake) GenerateID() int64 {
	return snow.node.Generate().Int64()
}
