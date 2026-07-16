package async

import (
	"context"
	"sync"
	"time"
	"webookApp/internal/async/repository"
	"webookApp/internal/code"
	"webookApp/internal/code/domain"
	"webookApp/pkg/retry"

	"go.uber.org/zap"
)

type AsyncServicer interface {
	Send(ctx context.Context, args []string, phoneNumber string) error
	Add(now time.Time, err error)
	needAsync() bool
	asyncSend()
	retry(ctx context.Context, ad domain.SmsEntity) bool
	advance(now time.Time)
	run()
	Stop()
}

// Async 同步转异步发送
type Async struct {
	cfg           *AsyncSMSWindowConfig
	mux           sync.Mutex
	SDK           code.SDKService
	wg            sync.WaitGroup
	stopOnce      sync.Once
	parentCtx     context.Context
	parentCancel  context.CancelFunc
	asyRepository *repository.AsyncRepository
	ctxParent     context.Context
}

type AsyncSMSWindowConfig struct {
	Threshold float64       // 错误率
	Interval  time.Duration // 子窗口大小
	Size      int           // 子窗口个数
}

var ErrorWaitingNotFoundSMS = repository.ErrorWaitingNotFoundSMS
var buckets []error    // 子窗口的error情况
var writeIndex int     // 当前写入的子窗口下标
var lastTime time.Time // 上次写入时间

func NewAsync(config *AsyncSMSWindowConfig, ar *repository.AsyncRepository, sdk code.SDKService) *Async {
	ctx, cancel := context.WithCancel(context.Background())
	c := &Async{
		cfg:           config,
		asyRepository: ar,
		ctxParent:     ctx,
		parentCancel:  cancel,
		SDK:           sdk,
	}
	// 开启异步监控
	c.wg.Add(1)
	go c.run()
	return c
}

// advace 用来更新写入writeIndex值
func (ac *Async) advance(now time.Time) {
	// 找到当前时间窗口打开边界
	nowTicke := now.Truncate(time.Duration(ac.cfg.Interval))

	if lastTime.IsZero() {
		lastTime = now
		return
	}
	// 判断窗口起始位置和上一次写入经过了多少子窗口
	elapsed := int(nowTicke.Sub(lastTime) / time.Duration(ac.cfg.Interval))
	// 说明没有超过1个子窗口
	if elapsed <= 0 {
		return
	}
	// 重置窗口
	if elapsed > int(ac.cfg.Size) {
		elapsed = int(ac.cfg.Size)
	}
	// 重置上一次写入时间
	lastTime = now

	// 说明超过了>=1个子窗口
	for i := 0; i < elapsed; i++ {
		// 循环回到上一个有旧数据的窗口
		writeIndex = (writeIndex + 1) % int(ac.cfg.Size)
		buckets[writeIndex] = nil
	}

}

// send 调用SDK发送短信
func (ac *Async) send(ctx context.Context, args []string, phoneNumber string) error {
	start := time.Now
	err := ac.SDK.Send(ctx, args, phoneNumber)
	ac.add(start(), err)
	if ac.needAsync() {
		// 操作数据库
		err := ac.asyRepository.Add(ctx, &domain.SmsEntity{
			PhoneNumber: phoneNumber,
			TplID:       args[2],
			Code:        args[0],
		})
		return err
	}
	// 如果出错了但是没触发阈值就让用户在1分钟后重新申请验证码
	return err
}

// add 更新子窗口Bucket[]error出错情况
func (ac *Async) add(now time.Time, err error) {
	// sync.Lock()不支持重入，advance()再次尝试获得锁会触发死锁
	ac.mux.Lock()
	defer ac.mux.Unlock()
	ac.advance(now)
	buckets[writeIndex] = err
}

// needAsync 查看窗口滑动是否触发阈值
func (ac *Async) needAsync() bool { // 触发阈值
	ac.mux.Lock()
	defer ac.mux.Unlock()

	ac.advance(time.Now()) // 先更新桶的情况
	counter := 0
	for _, val := range buckets {
		if val != nil {
			counter++
		}
	}
	if float64(counter) >= float64(ac.cfg.Threshold) {
		return true
	}
	return false
}

// asyncSend 异步发送并更新数据库
func (ac *Async) asyncSend() {
	ctx, cancel := context.WithTimeout(ac.ctxParent, 20*time.Second)
	defer cancel()
	// 底层数据库使用监听ctx.Done()通道完成超时断开连接释放资源
	as, err := ac.asyRepository.PreemptWaitingSMS(ctx)
	switch err {
	case nil: // 说明拿到过期短信正常,正常异步
		success := ac.retry(ctx, as) // 在这里不需要担心重试并发问题
		// 反馈结果
		err = ac.asyRepository.ReportScheduleResult(ctx, as.ID, success)
		if err != nil {
			zap.L().Error("写入异步发送数据库情况出错:", zap.Bool("异步发送结果:", success), zap.Error(err), zap.Int64("id", as.ID))
		}
	case ErrorWaitingNotFoundSMS:
		// 可以睡一秒
		time.Sleep(time.Second)
	case context.DeadlineExceeded:
		zap.L().Info("本次抢占因超时中断，自动重试")
	default:
		// 正常来说应该是数据库那边出了问题，
		// 但是为了尽量运行，还是要继续的
		// 你可以稍微睡眠，也可以不睡眠
		// 睡眠的话可以帮你规避掉短时间的网络抖动问题
		zap.L().Error("抢占异步发送短信任务失败", zap.Error(err))
		time.Sleep(time.Millisecond * 1000)
	}
}

// retry 重试调用SDK发送
func (ac *Async) retry(ctx context.Context, ad *domain.SmsEntity) bool {
	fn := func() (error, bool) {
		err := ac.SDK.Send(ctx, []string{ad.Code, "5分钟", ad.TplID}, ad.PhoneNumber)
		if err != nil {
			return err, false
		}
		return nil, true
	}
	err, res := retry.Retry(ctx, fn, &retry.BackoffOpts{MaxRetry: 3, InitElapse: time.Second, MaxElapse: 20 * time.Second}, false)
	if err != nil {
		zap.L().Error("重试失败:", zap.Error(err))
	}
	return res
}

// run 开启监控
func (ac *Async) run() {
	defer ac.wg.Done()
	for {
		select {
		case <-ac.ctxParent.Done():
			return
		default:
			ac.asyncSend()
		}
	}
}

// Stop 关闭监控
func (ac *Async) Stop() {
	ac.stopOnce.Do(func() {
		ac.parentCancel() // 监控所有的contextParent.Done channel
		done := make(chan struct{})

		go func() {
			ac.wg.Wait() // 优雅退出防止wait()卡死退不出
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
