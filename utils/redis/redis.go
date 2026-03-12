// Redis 初始化 + 缓存方法，连接池优化

package redis

import (
	"context"
	"time"
	"strings"

	"github.com/redis/go-redis/v9"
	"github.com/Caodongying/dongdong-IM/utils/config"
	"github.com/Caodongying/dongdong-IM/utils/logger"
	"go.uber.org/zap"
)

var RDB *redis.Client


// 初始化Redis
func Init() {
	cfg := config.Viper.GetConfig().Redis
	rdb := redis.NewClient(&redis.Options{
		Addr:         cfg.Addr,
		Password:     cfg.Password,
		DB:           cfg.DB,
		PoolSize:     cfg.PoolSize,
		MinIdleConns: cfg.MinIdleConns,
		ConnMaxLifetime: time.Duration(cfg.ConnMaxLifetime) * time.Second,
	})
	// 测试连接
	initCtx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if err := rdb.Ping(initCtx).Err(); err != nil {
		logger.Sugar.Panic("Redis连接失败", zap.Error(err))
	}
	RDB = rdb
	logger.Sugar.Info("Redis初始化成功", zap.String("addr", cfg.Addr))
}

// Close 优雅关闭Redis连接
func Close() {
	if err := RDB.Close(); err != nil {
		logger.Sugar.Error("Redis关闭失败", zap.Error(err))
		return
	}
	logger.Sugar.Info("Redis关闭成功")
}

// 两个Redis缓存的key模板
// 用{}作为占位符
const (
	// 邮箱→用户ID
	UserIDByEmailTemplate = "im:user:id:{}"
	// 用户ID→邮箱
	UserEmailByIDTemplate = "im:user:email:{}"
)


func SetUserCache(ctx context.Context, userID, email string) error {
	// 双缓存：email->ID + ID->email
	key1 := replaceKey(UserIDByEmailTemplate, email)
	key2 := replaceKey(UserEmailByIDTemplate, userID)

	// 创建管道，把多个 Redis 命令打包一起执行，减少网络往返次数
	pipe := RDB.Pipeline()
	pipe.Set(ctx, key1, userID, 24*time.Hour)
	pipe.Set(ctx, key2, email, 24*time.Hour)
	_, err := pipe.Exec(ctx)
	return err
}

func GetUserIdByUserEmailCache(ctx context.Context, email string) (string, error) {
	key := replaceKey(UserIDByEmailTemplate, email)
	return RDB.Get(ctx, key).Result()
}

// 替换key模板中的占位符
// 调用 replaceKey(KeyUserIDByEmail, "test@xxx.com")
// 会返回 "im:user:id:test@xxx.com"
func replaceKey(key, val string) string {
	return strings.ReplaceAll(key, "{}", val)
}