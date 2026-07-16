package retry

import (
	"context"
	"errors"
	"math"
	"math/rand"
	"time"
)

type InitElapse time.Duration
type MaxElapse time.Duration
type MaxRetry int
type Factor uint

// 带指数退避+随机抖动的重试
type Backoff struct {
	// 重试次数
	maxRetry MaxRetry
	// 初始时间
	initElapse InitElapse
	// 最大间隔
	maxElapse MaxElapse
	// 指数因子
	factor Factor
}

var (
	ErrCtxDeadline = context.DeadlineExceeded
)

// 带指数退避+随机抖动的重试
type BackoffOpts struct {
	// 重试次数
	MaxRetry int
	// 初始时间
	InitElapse time.Duration
	// 最大间隔
	MaxElapse time.Duration
}

func NewBackoff(maxRetry MaxRetry, initElapse_ InitElapse, maxElapse_ MaxElapse, baseNumber Factor, typeParame any) (*Backoff, error) {
	if maxRetry == 0 || initElapse_ == 0 || baseNumber == 0 {
		return nil, errors.New("maxRetry,initElapse,baseNumber is empty.")
	}
	return &Backoff{
		maxRetry:   maxRetry,
		initElapse: initElapse_,
		maxElapse:  maxElapse_,
		factor:     baseNumber,
	}, nil
}

// func (bo *Backoff) Retry(ctx context.Context, fn func() (error T), arguments ...any) (bool, error) {
// 	for attempt := 0; attempt < int(bo.maxRetry); attempt++ {
// 		err,T := fn()
// 		if err == nil {
// 			return true, nil
// 		}

// 		rand.Seed(time.Now().UTC().UnixMilli())
// 		randTime := rand.Intn(2)
// 		backoffTime_ := bo.computeDelay(attempt)

// 		// 计算要退避的时间理论时间:初始时间*(基数^幂)+随机抖动
// 		// backoffTime := backoffTime_ * bo.initElapse
// 		backoffTime := backoffTime_*time.Duration(bo.initElapse) + time.Duration(randTime)
// 		if backoffTime > time.Duration(bo.maxElapse) {
// 			backoffTime = time.Duration(bo.maxElapse)
// 		}
// 		// 设置超时通道和阻塞间隔通道
// 		select {
// 		case <-ctx.Done():
// 			return false, ErrCtxDeadline
// 		case <-time.After(backoffTime):
// 		}
// 	}
// 	return false, nil
// }

func Retry[T any](ctx context.Context, fn func() (error, T), opts *BackoffOpts, defalult T) (error, T) {
	var err error
	for attempt := 0; attempt < opts.MaxRetry; attempt++ {
		err_, T := fn()
		if err_ == nil {
			return err_, T
		}

		rand.Seed(time.Now().UTC().UnixMilli())
		randTime := rand.Intn(2)

		backoffTime := time.Duration(math.Exp2(float64(attempt)))*opts.InitElapse + time.Duration(randTime)
		if backoffTime > opts.MaxElapse {
			backoffTime = opts.MaxElapse
		}
		err = err_
		// 设置超时通道和阻塞间隔通道
		select {
		case <-ctx.Done():
			return ErrCtxDeadline, T
		case <-time.After(backoffTime):
		}
	}
	return err, defalult
}

// func (bo *Backoff) computeDelay(attempt int) time.Duration {
// 	var result Factor = 1
// 	for i := 0; i < attempt; i++ {
// 		result *= bo.factor
// 	}
// 	return time.Duration(result)
// }
