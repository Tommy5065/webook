package mq

// 消息队列深挖点:
// 2.优化共享内存的锁机制--实现无锁设计
// 3.优化Redis set键个数，压缩内存使用

import (
	"context"
	"fmt"
	"slices"
	"strconv"
	"sync"
	"time"
	"webookApp/internal/mq"
	"webookApp/internal/mq/domain"

	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

//消息队列两极生产者-消费者模型

type MqConsumer struct {
	redisCmd     *redis.Client
	mqRepository mq.MqRepository
	ctxParent    context.Context
	cancelParent context.CancelFunc
	mu           sync.Mutex
	stopOnce     sync.Once
	buff         map[int64]map[int64]*domain.MqRecorde // 第一个key是发起动作者ID,第二个key是被动接受者ID
	wg           sync.WaitGroup
	opts         *MqConsumerConfig
	retryOpts    *MqRetryConsumerConfig
}

type MqConsumerConfig struct {
	RedisClient *redis.Client
	BuffSize    int
	StreamName  string
	GroupName   string
	Department  string
	Count       int64
	Block       time.Duration
	Timer       time.Duration
}

type MqRetryConsumerConfig struct {
	RetryTimer time.Duration
	Idle       time.Duration
	RetryTime  int64
	Count      int64
}

func NewMqConsumer(mqRepo mq.MqRepository, config *MqConsumerConfig, configRetry *MqRetryConsumerConfig) *MqConsumer {
	ctx, cancel := context.WithCancel(context.Background())
	c := &MqConsumer{
		redisCmd:     config.RedisClient,
		mqRepository: mqRepo,
		ctxParent:    ctx,
		cancelParent: cancel,
		buff:         make(map[int64]map[int64]*domain.MqRecorde, config.BuffSize),
		opts:         config,
		retryOpts:    configRetry,
	}
	c.wg.Add(3)
	go c.consumerLoop()
	go c.flushLoop()
	go c.claimAndRetry()
	return c
}

// 一级消费者
func (ll *MqConsumer) consumerLoop() {
	defer ll.wg.Done()
	// 第一次先创建创建好消费者
	consumerName := fmt.Sprintf("%s-%v", ll.opts.Department, time.Now().UnixMilli())
	err := ll.redisCmd.XGroupCreate(ll.ctxParent, ll.opts.StreamName, ll.opts.GroupName, "0").Err()
	if err != nil && err.Error() != "BUSYGROUP Consumer Group name already exists" {
		zap.L().Error("like consumer group create fail:%s", zap.Error(err))
		return
	}
	for {
		select {
		case <-ll.ctxParent.Done():
			return
		default:
			// 拉取redis消息
			streams, err := ll.redisCmd.XReadGroup(ll.ctxParent, &redis.XReadGroupArgs{
				Group:    ll.opts.GroupName,
				Consumer: consumerName,
				Streams:  []string{ll.opts.StreamName, ">"}, // 只读取新消息
				NoAck:    false,
				Count:    ll.opts.Count, // 一次拉多少条
				Block:    ll.opts.Block, // 一直阻塞多久
			}).Result()

			if err != nil {
				if err == redis.Nil {
					continue
				} else {
					zap.L().Error("Stream拉取消息失败:", zap.Error(err))
				}
			}

			for _, stream := range streams {
				for _, message := range stream.Messages {
					likeRecorde := ll.xstreamToLikeRecorde(message)
					if likeRecorde == nil {
						continue
					}
					ll.Add(likeRecorde)
				}
			}
		}
	}
}

// Add 一级消费者回调函数—-只记录用户一系列行为的最终状态
func (ll *MqConsumer) Add(li *domain.MqRecorde) {
	// // 加锁多线程共享资源
	ll.mu.Lock()
	defer ll.mu.Unlock()

	// 解决第一层
	if _, ok := ll.buff[li.PassivityID]; !ok { // 说明这是本帖第一个赞
		ll.buff[li.PassivityID] = make(map[int64]*domain.MqRecorde)
		ll.buff[li.PassivityID][li.DrivingID] = li
		return
	}
	pre, exists := ll.buff[li.PassivityID][li.DrivingID]

	if exists { // 说明已存在用户行为更新
		// 保存从最大时间戳截止的该用户行为消息ID
		if pre.Ts < li.Ts {
			pre.Action = li.Action
			pre.Ts = li.Ts
		}
		pre.MessagesID = append(pre.MessagesID, li.MessagesID...)
	} else { // 说明帖子点赞新人
		ll.buff[li.PassivityID][li.DrivingID] = li
	}

	if len(ll.buff[li.PassivityID]) >= 500 { // 这个帖子/人已经有500名用户的最终行为了
		normalCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		go func() {
			defer cancel()
			ll.flush(normalCtx) // 正常刷盘，不要和父context耦合否则会在刚满500的时候Done丢数据
		}()
	}

}

// flushLoop 定时拉取消费,二级消费者
func (ll *MqConsumer) flushLoop() {
	defer ll.wg.Done()
	// 二级消费者
	timer := time.NewTicker(ll.opts.Timer) // 创建一个定时任务器
	defer timer.Stop()                     // 及时关闭时间任务器

	for {
		select {
		case <-ll.ctxParent.Done():
			// 直接使用全新的 Background，只保留 5 秒超时,避免剩下的数据无法刷盘
			emergencyCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			totalAckMessage := ll.flush(emergencyCtx)
			zap.L().Info("flushloop 父协程取消")
			cancel()
			if len(totalAckMessage) != 0 {
				ll.xack(totalAckMessage)
			}
			return

		case <-timer.C:
			normalCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			totalAckMessage := ll.flush(normalCtx) // 正常刷盘，不要和父context耦合否则在3分01秒的时候会丢数据
			cancel()
			if len(totalAckMessage) != 0 {
				ll.xack(totalAckMessage)
			}
		}
	}
}

// flush 二级消费者回调函数
func (ll *MqConsumer) flush(ctx context.Context) []interface{} {
	ll.mu.Lock()
	record := make([]*domain.MqRecorde, 0, 300)
	if len(ll.buff) == 0 { // 还没消息不要刷盘
		ll.mu.Unlock()
		return nil
	}
	// 生成动态的存储池的中间变量,因为buf要清空继续收第一级生产者消息
	for _, items := range ll.buff {
		for _, item := range items {
			record = append(record, item)
		}
	}

	ll.buff = make(map[int64]map[int64]*domain.MqRecorde)
	ll.mu.Unlock()
	totalAckMessage := ll.mqRepository.Batch(ctx, record)
	return totalAckMessage
}

// stop 停止消费者
func (ll *MqConsumer) Stop() {
	ll.stopOnce.Do(func() {
		ll.cancelParent() // 监控所有的contextParent.Done channel
		done := make(chan struct{})

		go func() {
			ll.wg.Wait() // 优雅退出防止wait()卡死退不出
			close(done)
		}()
		// 在这里阻塞查看是超时先到还是done的channel消息先到
		select {
		case <-done:
			zap.L().Info("所有协程退出")
		case <-time.After(5 * time.Second):
			zap.L().Info("强制退出协程")
		}
	},
	)
}

// xack 重复都需要ack部分抽象出来
func (ll *MqConsumer) xack(total []interface{}) {
	for _, itemsMap := range total {
		items, _ := itemsMap.(map[int][]string)
		for _, item := range items {

			err := ll.redisCmd.XAck(context.Background(), ll.opts.StreamName, ll.opts.GroupName, item...).Err()
			if err != nil {
				zap.L().Error("xack err:", zap.Error(err))
				return
			} else {
				// 要手动删除stream才能释放不然只是消费者的PEL被删了
				ll.redisCmd.XDel(context.Background(), ll.opts.StreamName, item...).Err()
			}
		}
	}
}

// claimAndRetry重试消费者重试整个链路
func (ll *MqConsumer) claimAndRetry() {
	defer ll.wg.Done()
	consumerName := "retryConsumer"
	currentStart := "0-0"
	timer := time.NewTimer(ll.retryOpts.RetryTimer)
	defer timer.Stop()

	for {
		select {
		case <-ll.ctxParent.Done():
			return
		case <-timer.C:
			// 避免递归栈溢出使用for循环，扫完满足条件没有ack的消息
			asynClaimCtx, claimCancel := context.WithTimeout(context.Background(), time.Second)
			for {
				messages, nextStart, err := ll.redisCmd.XAutoClaim(asynClaimCtx, &redis.XAutoClaimArgs{
					Stream:   ll.opts.StreamName,
					Group:    ll.opts.GroupName,
					MinIdle:  time.Minute,
					Consumer: consumerName,
					Start:    currentStart,
					Count:    ll.retryOpts.Count,
				}).Result()

				if err != nil {
					zap.L().Error("pending 消息ack失败", zap.Error(err))
					break
				}

				// 一批一批处理
				if len(messages) > 0 {
					// 同步处理
					ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
					ll.processAndRetry(ctx, messages)
					cancel()
				}

				// 说明pending消息已经全部扫描完了
				if nextStart == "0-0" {
					break
				}
				// 断点继续
				currentStart = nextStart
			}
			claimCancel()
		}
	}
}

// processAndRetry 一批一批重试ack
func (ll *MqConsumer) processAndRetry(ctx context.Context, messages []redis.XMessage) {

	recorde := make([]*domain.MqRecorde, 0, len(messages))
	msgID := make([]string, 0, len(messages))

	for _, message := range messages {
		likeRecord := ll.xstreamToLikeRecorde(message)
		if likeRecord == nil {
			continue
		}
		recorde = append(recorde, likeRecord)
		msgID = append(msgID, message.ID)
	}

	// 查出消息投递超过3次的，直接死信
	deadID := ll.deadMsgToDeadBeter(ctx, msgID, 3)
	if len(deadID) > 0 {
		_, err := ll.redisCmd.XAck(context.Background(), ll.opts.StreamName, ll.opts.GroupName, deadID...).Result()
		if err != nil {
			zap.L().Error("死信ack失败", zap.Strings("ids:", deadID))
		} else {
			ll.redisCmd.XDel(context.Background(), ll.opts.StreamName, deadID...)
			zap.L().Warn("消息已传为死信,请人工检查:", zap.Strings("ids:", deadID))
		}
	}

	// 把已经投递到死信队列里的消息挑出
	deadSet := make(map[string]bool)
	for _, m := range deadID {
		deadSet[m] = true
	}

	avalidRecord := recorde[:0]
	for _, r := range recorde {
		if !deadSet[r.MessagesID[0]] {
			avalidRecord = append(avalidRecord, r)
		}
	}

	for _, likeRecord := range avalidRecord {
		ll.Add(likeRecord)
	}

	totalMessageID := ll.flush(ctx)

	for _, itemsMap := range totalMessageID {
		items, _ := itemsMap.(map[int][]string)
		for _, item := range items {
			err := ll.redisCmd.XAck(context.Background(), ll.opts.StreamName, ll.opts.GroupName, item...).Err()
			if err != nil {
				zap.L().Error("积压消息 ACK 失败", zap.Error(err))
			} else {
				err = ll.redisCmd.XDel(context.Background(), ll.opts.StreamName, item...).Err()
				if err != nil {
					zap.L().Error("like xdel err:", zap.Error(err))
				}
			}
		}
	}
}

// xstreamToLikeRecorde 解析xstream
func (ll *MqConsumer) xstreamToLikeRecorde(message redis.XMessage) *domain.MqRecorde {
	drivingIDString, _ := message.Values["driving_id"].(string)
	passivityIDString, _ := message.Values["passivity_id"].(string)
	tsString, _ := message.Values["timestamp"].(string)
	action, _ := message.Values["action"].(string)

	drivingID, err := strconv.ParseInt(drivingIDString, 10, 64)
	if err != nil {
		zap.L().Error("drivingID解析失败:", zap.Error(err))
		return nil
	}

	passivityID, err := strconv.ParseInt(passivityIDString, 10, 64)
	if err != nil {
		zap.L().Error("passivityID解析失败:", zap.Error(err))
		return nil
	}

	ts, err := strconv.ParseInt(tsString, 10, 64)
	if err != nil {
		zap.L().Error("timestamp解析失败:", zap.Error(err))
		return nil
	}
	return &domain.MqRecorde{
		Action:      action,
		DrivingID:   drivingID,
		PassivityID: passivityID,
		Ts:          ts,
		MessagesID:  []string{message.ID},
	}
}

// deadMsgToDeadBeter 查找出投递达到3次的消息
func (ll *MqConsumer) deadMsgToDeadBeter(ctx context.Context, msgID []string, retryCount int64) []string {
	var deadID []string
	msgID = ll.stringsSort(msgID)
	length := len(msgID)

	for i := 0; i < length; i += 100 {
		end := i + 100
		if end > length {
			end = length
		}
		result, err := ll.redisCmd.XPendingExt(ctx, &redis.XPendingExtArgs{
			Stream: ll.opts.StreamName,
			Group:  ll.opts.GroupName,
			Idle:   time.Minute,
			Start:  msgID[i],
			End:    msgID[end-1],
			Count:  100,
		}).Result()

		if err != nil {
			zap.L().Warn("查询投递次数失败,保守不丢弃", zap.Strings("id:", msgID[i:end]), zap.Error(err))
		}

		if len(result) > 0 {
			for _, info := range result {
				if info.RetryCount != retryCount {
					continue
				}
				deadID = append(deadID, info.ID)
			}
		}
	}
	return deadID
}

// stringsSort 排序string类型的ID
func (ll *MqConsumer) stringsSort(msgID []string) []string {
	idNumber := make([]int64, 0, len(msgID))
	finalID := make([]string, 0, len(msgID))
	for _, i := range msgID {
		number, _ := strconv.ParseInt(i, 10, 64)
		idNumber = append(idNumber, number)
	}
	slices.Sort(idNumber)
	for _, j := range idNumber {
		finalID = append(finalID, strconv.FormatInt(j, 10))
	}
	return finalID
}
