package bgService

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"webookApp/internal/async/repository"
	dao "webookApp/internal/async/repository/mysql"
	async "webookApp/internal/async/service"
	"webookApp/internal/code"
	followdomain "webookApp/internal/follow/domain"
	followdao "webookApp/internal/follow/repository/mysql"
	likedomain "webookApp/internal/like/domain"
	likedao "webookApp/internal/like/repository/mysql"
	"webookApp/internal/mq"
	redmq "webookApp/internal/mq/redmq"

	"github.com/redis/go-redis/v9"
	"github.com/spf13/viper"
)

type BackgroundServicer interface {
	Stop()
}

// LikeSyncer 包装点赞同步器
type LikeSyncer struct {
	syncer *mq.TargetSyncer
}

func (l *LikeSyncer) Stop() {
	l.syncer.Stop()
}

// FollowSyncer 包装关注同步器
type FollowSyncer struct {
	fanSyncer    *mq.TargetSyncer // 粉丝数同步
	followSyncer *mq.TargetSyncer // 关注数同步
}

func (f *FollowSyncer) Stop() {
	f.fanSyncer.Stop()
	f.followSyncer.Stop()
}

type BackgroundService []BackgroundServicer

// 提供点赞同步器
func ProvideLikeSyncer(db *sql.DB, rds *redis.Client) *LikeSyncer {
	c := mq.NewTargetSyncer(mq.TargetSyncerConfig{
		FetchTargetIDFunc: func(ctx context.Context, rds *redis.Client) ([]string, error) {
			return rds.SMembers(ctx, "dirty:article").Result()
		},
		CountKeyFunc: func(id string) string {
			return "article:count:" + id
		},
		CountNumberFunc: func(ctx context.Context, rds *redis.Client, keys []string) ([]string, error) {
			res, err := rds.MGet(ctx, keys...).Result()
			if err != nil {
				return nil, err
			}

			rescount := make([]string, 0, len(keys))
			for _, val_ := range res {
				val, ok := val_.(string)
				if !ok {
					// 类型不匹配时，返回明确错误
					if val_ == nil {
						val = "0"
						rescount = append(rescount, val)
						continue
					}
					err := fmt.Errorf("like的计数器数值不是string, got %T", val_)
					return nil, err
				}
				rescount = append(rescount, val)
			}
			return rescount, nil // 这里闭合了 for 循环和闭包本身
		},

		UpdateFunc: func(ctx context.Context, db *sql.DB, parames map[string]string) (reError error) {
			values := make([]string, 0, len(parames))
			args := make([]interface{}, 0, len(parames)*2)

			for id, value := range parames {
				values = append(values, "?")
				args = append(args, value, id)
			}

			// 开启事务文章在发表库和制作库都存在
			tx, err := db.BeginTx(ctx, &sql.TxOptions{
				Isolation: sql.LevelRepeatableRead,
			})
			if err != nil {
				reError = fmt.Errorf("like开启事务失败:%w", err)
				return reError
			}

			statement1 := fmt.Sprintf("UPDATE articles SET cnt=? WHERE artical_id IN (%s)", strings.Join(values, ","))
			statement2 := fmt.Sprintf("UPDATE draft SET cnt=? WHERE draft_id IN (%s)", strings.Join(values, ","))
			_, reError = tx.ExecContext(ctx, statement1, args...)
			_, reError = tx.ExecContext(ctx, statement2, args...)

			defer func() {
				if err != nil {
					if rbError := tx.Rollback(); rbError != nil {
						reError = fmt.Errorf("like更新失败,原错误和回滚错误分别是:%w", errors.Join(err, rbError))
					}
				} else {
					reError = tx.Commit()
				}
			}()
			return
		},

		DeleteFunc: func(ctx context.Context, rds *redis.Client, ids []string) error {
			return rds.SRem(ctx, "dirty:article", ids).Err()
		},
	}, db, rds) // 三个参数：config, db, rds
	return &LikeSyncer{syncer: c}
}

// 提供粉丝数同步器
func provideFollowFansSyncer(db *sql.DB, rds *redis.Client) *mq.TargetSyncer {
	return mq.NewTargetSyncer(mq.TargetSyncerConfig{
		FetchTargetIDFunc: func(ctx context.Context, rds *redis.Client) ([]string, error) {
			return rds.SMembers(ctx, "dirty:following").Result()
		},
		CountKeyFunc: func(id string) string {
			return "follow:following:count:" + id
		},
		CountNumberFunc: func(ctx context.Context, rds *redis.Client, keys []string) ([]string, error) {
			res, err := rds.MGet(ctx, keys...).Result()
			if err != nil {
				return nil, err
			}

			rescount := make([]string, 0, len(keys))
			for _, val_ := range res {
				val, ok := val_.(string)
				if !ok {
					// 类型不匹配时，返回明确错误
					err := fmt.Errorf("follow的粉丝计数器数值不是string, got %T", val_)
					return nil, err
				}
				rescount = append(rescount, val)
			}
			return rescount, nil // 这里闭合了 for 循环和闭包本身
		},

		UpdateFunc: func(ctx context.Context, db *sql.DB, parames map[string]string) (reError error) {
			values := make([]string, 0, len(parames))
			args := make([]interface{}, 0, len(parames)*2)

			for id, value := range parames {
				values = append(values, "?")
				args = append(args, value, id)
			}

			statement1 := fmt.Sprintf("UPDATE users SET fan_count=? WHERE id IN (%s)", strings.Join(values, ","))
			_, reError = db.ExecContext(ctx, statement1, args...)

			if reError != nil {
				reError = fmt.Errorf("follow粉丝数更新失败:%w", reError)
				return reError
			}
			return nil
		},

		DeleteFunc: func(ctx context.Context, rds *redis.Client, ids []string) error {
			return rds.SRem(ctx, "dirty:following", ids).Err()
		},
	}, db, rds) // 三个参数：config, db, rds
}

// 提供关注数同步器
func provideFollowingSyncer(db *sql.DB, rds *redis.Client) *mq.TargetSyncer {
	return mq.NewTargetSyncer(mq.TargetSyncerConfig{
		FetchTargetIDFunc: func(ctx context.Context, rds *redis.Client) ([]string, error) {
			return rds.SMembers(ctx, "dirty:followed").Result()
		},
		CountKeyFunc: func(id string) string {
			return "follow:followed:count:" + id
		},
		CountNumberFunc: func(ctx context.Context, rds *redis.Client, keys []string) ([]string, error) {
			res, err := rds.MGet(ctx, keys...).Result()
			if err != nil {
				return nil, err
			}

			rescount := make([]string, 0, len(keys))
			for _, val_ := range res {
				val, ok := val_.(string)
				if !ok {
					// 类型不匹配时，返回明确错误
					err := fmt.Errorf("follow的关注数计数器数值不是string, got %T", val_)
					return nil, err
				}
				rescount = append(rescount, val)
			}
			return rescount, nil // 这里闭合了 for 循环和闭包本身
		},

		UpdateFunc: func(ctx context.Context, db *sql.DB, parames map[string]string) (reError error) {
			values := make([]string, 0, len(parames))
			args := make([]interface{}, 0, len(parames)*2)

			for id, value := range parames {
				values = append(values, "?")
				args = append(args, value, id)
			}

			statement1 := fmt.Sprintf("UPDATE users SET following_count=? WHERE id IN (%s)", strings.Join(values, ","))
			_, reError = db.ExecContext(ctx, statement1, args...)

			if reError != nil {
				reError = fmt.Errorf("follow关注数更新失败:%w", reError)
				return reError
			}
			return nil
		},

		DeleteFunc: func(ctx context.Context, rds *redis.Client, ids []string) error {
			return rds.SRem(ctx, "dirty:followed", ids).Err()
		},
	}, db, rds) // 三个参数：config, db, rds
}

// 提供用户关系异步任务同步器
func ProvideFollowSyncer(db *sql.DB, rds *redis.Client) *FollowSyncer {
	return &FollowSyncer{
		fanSyncer:    provideFollowFansSyncer(db, rds),
		followSyncer: provideFollowingSyncer(db, rds),
	}
}

// 提供点赞的消费者
func ProvideLikeConsumer(dataRepo *likedao.LikeDao, likeConsumerAllConfig *likedomain.LikeMqConsumerAllConfig) *likedomain.LikeConsumer {
	c := redmq.NewMqConsumer(dataRepo, likeConsumerAllConfig.NormalConfig, likeConsumerAllConfig.RetryConfig)
	return &likedomain.LikeConsumer{
		Consumer: c,
	}
}

// 提供关注的消费者
func ProvideFollowConsumer(dataRepo *followdao.FollowDao, followConsumerConfig *followdomain.FollowConsumerAllConfig) *followdomain.FollowConsumer {
	c := redmq.NewMqConsumer(dataRepo, followConsumerConfig.NormalConfig, followConsumerConfig.RetryConfig)
	return &followdomain.FollowConsumer{
		Consumer: c,
	}

}

// 开启异步发送SMS
func ProviderSMSSyncer(v *viper.Viper, db *sql.DB, sdk code.SDKService) *async.Async {
	cfg := &async.AsyncSMSWindowConfig{
		Threshold: v.GetFloat64("mq.smsSync.threshold"),
		Interval:  v.GetDuration("mq.smsSync.interval"),
		Size:      v.GetInt("mq.smsSync.windowSize"),
	}
	arMysql := dao.NewAsync(db)
	ar := repository.NewAsyncRepository(arMysql)
	return async.NewAsync(cfg, ar, sdk)
}

// 将所有后台服务收集到一起
func ProvideBackgroundServices(like *LikeSyncer, follow *FollowSyncer, likeconsumer *likedomain.LikeConsumer, followconsumer *followdomain.FollowConsumer, sms *async.Async) BackgroundService {
	return []BackgroundServicer{like, follow, sms, likeconsumer, followconsumer}
}
