// 分布式锁，修复防误删 + 自动续期
package redis

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/Caodongying/dongdong-IM/utils/config"
	"github.com/Caodongying/dongdong-IM/utils/logger"
	"go.uber.org/zap"
)

var (
	ErrLockAcquire = errors.New("获取锁失败")
	ErrLockRelease = errors.New("释放锁失败")
	ErrLockExpire  = errors.New("锁过期或不属于当前客户端")
)

// Redis分布式锁结构
type RedisLock struct {
	key     string        // 锁key
	expire  time.Duration // 过期时间
	renew   time.Duration // 自动续期间隔
	client  *redis.Client // Redis客户端
	nonce   string        // 随机值（防误删/误续其他客户端的锁）
	ctx     context.Context
	cancel  context.CancelFunc
}

// generateNonce 生成唯一随机nonce（处理错误）
func generateNonce() (string, error) {
	b := make([]byte, 16)
	n, err := rand.Read(b)
	if err != nil {
		return "", err
	}
	if n != len(b) {
		return "", errors.New("生成随机nonce不完整")
	}
	return base64.URLEncoding.EncodeToString(b), nil
}

// 新建分布式锁
// 仅初始化基础参数，不提前生成nonce/创建上下文
func NewRedisLock(key string) *RedisLock {
	cfg := config.Viper.GetConfig().Redis
	return &RedisLock{
		key:    key,
		expire: time.Duration(cfg.LockExpire) * time.Second,
		renew:  time.Duration(cfg.LockRenew) * time.Second,
		client: RDB,
	}
}

// 获取分布式锁
func (l *RedisLock) TryLock(ctx context.Context) (bool, error) {
	// 生成唯一nonce
	nonce, err := generateNonce()
	if err != nil {
		logger.Sugar.Error("生成nonce失败", zap.Error(err))
		return false, err
	}

	l.ctx, l.cancel = context.WithCancel(ctx)
	l.nonce = nonce

	ok, err := l.client.SetNX(
		l.ctx,
		l.key,
		l.nonce,
		l.expire,
	).Result()
	if err != nil {
		logger.Sugar.Error("Redis SetNX获取分布式锁失败", zap.String("key", l.key), zap.Error(err))
		return false, err
	}

	// 抢锁成功，启动自动续期协程
	if ok {
		go l.renewLock()
		logger.Sugar.Debug("获取锁成功", zap.String("key", l.key), zap.String("nonce", l.nonce))
		return true, nil
	}

	// 抢锁失败
	logger.Sugar.Debug("获取锁失败，锁已被占用", zap.String("key", l.key))
	return false, ErrLockAcquire
}

// renewLock 自动续期（优化：补充日志字段，逻辑更健壮）
func (l *RedisLock) renewLock() {
	if l.ctx == nil || l.client == nil || l.nonce == "" {
		logger.Sugar.Warn("续期条件不满足，跳过续期", zap.String("key", l.key))
		return
	}

	ticker := time.NewTicker(l.renew)
	defer ticker.Stop() // 函数退出时停止ticker，防止泄漏

	// Lua脚本：原子验证nonce + 续期
	renewLua := `
		if redis.call('GET', KEYS[1]) == ARGV[1] then
			return redis.call('EXPIRE', KEYS[1], ARGV[2])
		else
			return 0
		end
	`

	for {
		select {
		case <-ticker.C: // 到续期时间
			res, err := l.client.Eval(
				l.ctx,
				renewLua,
				[]string{l.key},
				l.nonce,
				int64(l.expire.Seconds()),
			).Result()

			if err != nil {
				logger.Sugar.Warn("续期脚本执行失败", zap.String("key", l.key), zap.String("nonce", l.nonce), zap.Error(err))
				return
			}

			// res=0说明nonce不匹配，锁已被抢占/过期
			if res.(int64) == 0 {
				logger.Sugar.Warn("续期失败，nonce不匹配", zap.String("key", l.key), zap.String("nonce", l.nonce))
				return
			}

			logger.Sugar.Debug("锁续期成功", zap.String("key", l.key), zap.String("nonce", l.nonce), zap.Duration("expire", l.expire))

		case <-l.ctx.Done(): // 上下文取消（释放锁/外部终止）
			logger.Sugar.Info("续期协程退出", zap.String("key", l.key), zap.String("nonce", l.nonce))
			return
		}
	}
}

// Unlock 释放锁（Lua脚本保证原子性，防误删）
func (l *RedisLock) Unlock() error {
	if l.cancel == nil {
		logger.Sugar.Warn("释放锁失败，锁未获取", zap.String("key", l.key))
		return ErrLockRelease
	}

	// 先停止续期
	defer l.cancel()

	// Lua脚本：验证nonce匹配才删除，原子操作
	unlockScript := redis.NewScript(`
		if redis.call('GET', KEYS[1]) == ARGV[1] then
			return redis.call('DEL', KEYS[1])
		else
			return 0
		end
	`)

	res, err := unlockScript.Run(l.ctx, l.client, []string{l.key}, l.nonce).Int64()
	if err != nil {
		logger.Sugar.Error("执行释放锁脚本失败", zap.String("key", l.key), zap.String("nonce", l.nonce), zap.Error(err))
		return ErrLockRelease
	}

	if res == 0 {
		logger.Sugar.Warn("释放锁失败，nonce不匹配或锁已过期", zap.String("key", l.key), zap.String("nonce", l.nonce))
		return ErrLockExpire
	}

	logger.Sugar.Debug("释放锁成功", zap.String("key", l.key), zap.String("nonce", l.nonce))
	return nil
}