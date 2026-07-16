package dao

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"webookApp/internal/mq/domain"

	"go.uber.org/zap"
)

type FollowDao struct {
	db *sql.DB
}

func NewFollowDao(db *sql.DB) *FollowDao {
	return &FollowDao{
		db: db,
	}
}

func (fd *FollowDao) Insert(ctx context.Context, list []*domain.MqConsumerRecord) map[int][]string {
	// 分批写入考虑到数据库一次写入很多要爆炸

	bachSize := 300
	finalAckID := make(map[int][]string)
	j := 0
	for i := 0; i < len(list); i += bachSize {
		end := i + bachSize
		if end >= len(list) {
			end = len(list)
		}

		insertTarget := list[i:end]
		palceholder := make([]string, 0, end)
		args := make([]interface{}, 0, end*4)
		finalAckID[j] = make([]string, 0, 1000)

		for _, value := range insertTarget {
			palceholder = append(palceholder, "(?,?,?,?)")
			args = append(args, value.DrivingID, value.PassivityID, value.Status, value.TimeStamp)
			finalAckID[j] = append(finalAckID[j], value.MessagesID...)
		}

		// 一次网络RTT
		statment := fmt.Sprintf("INSERT INTO follow_log(follower_id,followee_id,status,op_time) VALUES %s ON DUPLICATE KEY UPDATE status = IF(VALUES(op_time) > op_time, VALUES(status),status), op_time= IF(VALUES(op_time) > op_time, VALUES(op_time), op_time)", strings.Join(palceholder, ","))
		_, err := fd.db.ExecContext(ctx, statment, args...)

		if err != nil { // 批量执行一次失败，则集体失败
			zap.L().Error("batchInsert 出错:", zap.Error(err))
			delete(finalAckID, j)
			continue
		}
		j++
	}

	return finalAckID
}

func (fd *FollowDao) Delete(ctx context.Context, list []*domain.MqConsumerRecord) map[int][]string {
	// 分批写入考虑到数据库一次写入很多要爆炸

	bachSize := 300
	finalAckID := make(map[int][]string)
	j := 0
	for i := 0; i < len(list); i += bachSize {
		end := i + bachSize
		if end >= len(list) {
			end = len(list)
		}

		insertTarget := list[i:end]
		palceholder := make([]string, 0, end)
		args := make([]interface{}, 0, end*4)
		finalAckID[j] = make([]string, 0, end-i)

		for _, value := range insertTarget {
			palceholder = append(palceholder, "(?,?,?,?)")
			args = append(args, value.DrivingID, value.PassivityID, value.Status, value.TimeStamp)
			finalAckID[j] = append(finalAckID[j], value.MessagesID...) // 收集这一批的历史消息ID
		}

		// 一次网络RTT
		statment := fmt.Sprintf("INSERT INTO follow_log(follower_id,followee_id,status,op_time) VALUES %s ON DUPLICATE KEY UPDATE status = IF(VALUES(op_time) > op_time, VALUES(status),status), op_time= IF(VALUES(op_time) > op_time, VALUES(op_time), op_time)", strings.Join(palceholder, ","))
		_, err := fd.db.ExecContext(ctx, statment, args...)
		if err != nil {
			zap.L().Error("batchDelete 出错:", zap.Error(err))
			delete(finalAckID, j)
			continue
		}
		j++
	}
	return finalAckID
}

// Batch 分流消息行为
func (fd *FollowDao) Batch(ctx context.Context, records []*domain.MqRecorde) []interface{} {
	totalMessage := make([]interface{}, 0, 2)
	likes := make([]*domain.MqConsumerRecord, 0, len(records))
	dislikes := make([]*domain.MqConsumerRecord, 0, len(records))
	// 分批执行
	for _, item := range records {
		switch item.Action {
		case "follow":
			likes = append(likes, &domain.MqConsumerRecord{DrivingID: item.DrivingID, PassivityID: item.PassivityID, Status: 1, TimeStamp: item.Ts, MessagesID: item.MessagesID})
		case "disFollow":
			dislikes = append(dislikes, &domain.MqConsumerRecord{DrivingID: item.DrivingID, PassivityID: item.PassivityID, Status: 0, TimeStamp: item.Ts, MessagesID: item.MessagesID})
		}
	}

	if len(likes) > 0 {
		insertSuccessMessage := fd.Insert(ctx, likes)
		totalMessage = append(totalMessage, insertSuccessMessage)
	}
	if len(dislikes) > 0 {
		deleteSuccessMessage := fd.Delete(ctx, dislikes)
		totalMessage = append(totalMessage, deleteSuccessMessage)
	}

	return totalMessage
}
