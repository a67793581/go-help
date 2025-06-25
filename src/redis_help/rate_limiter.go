package redis_help

import (
	"context"
	"errors"
	"fmt"
	"time"

	redis "github.com/redis/go-redis/v9"
)

// RateLimiter 限流器结构体
type RateLimiter struct {
	client   redis.UniversalClient
	key      string        // 私有配置：Redis key（不包含时间单位）
	maxCount int64         // 私有配置：最大允许的请求数量
	timeUnit time.Duration // 私有配置：时间单位（如1天、1小时等）
}

// RateLimitConfig 限流配置（仅用于初始化）
type RateLimitConfig struct {
	Key      string        // Redis key（不包含时间单位）
	MaxCount int64         // 最大允许的请求数量
	TimeUnit time.Duration // 时间单位（如1天、1小时等）
}

// NewRateLimiter 创建新的限流器（在初始化时完成所有检查）
func NewRateLimiter(client redis.UniversalClient, config RateLimitConfig) (*RateLimiter, error) {
	// 在初始化时完成所有检查
	if client == nil {
		return nil, errors.New("redis client cannot be nil")
	}
	if config.MaxCount <= 0 {
		return nil, errors.New("max count must be greater than 0")
	}
	if config.TimeUnit <= 0 {
		return nil, errors.New("time unit must be greater than 0")
	}
	if config.Key == "" {
		return nil, errors.New("key cannot be empty")
	}

	// 检查配置合理性：确保时间单位在合理范围内
	if config.TimeUnit < time.Second {
		return nil, fmt.Errorf("time unit cannot be less than 1 second")
	}
	if config.TimeUnit > 24*time.Hour {
		return nil, fmt.Errorf("time unit cannot exceed 32 days")
	}

	// 检查配置合理性：确保请求密度合理
	// 计算每秒请求数
	requestsPerSecond := float64(config.MaxCount) / config.TimeUnit.Seconds()

	// 最小密度：每秒0.1个请求（避免过于宽松）
	minRequestsPerSecond := 0.1
	// 最大密度：每秒10000个请求（避免过于严格）
	maxRequestsPerSecond := 10000.0

	if requestsPerSecond < minRequestsPerSecond {
		return nil, fmt.Errorf("request density too low: %.2f requests/second (<%.1f), please increase max count or decrease time unit", requestsPerSecond, minRequestsPerSecond)
	}
	if requestsPerSecond > maxRequestsPerSecond {
		return nil, fmt.Errorf("request density too high: %.2f requests/second (>%.0f), please decrease max count or increase time unit", requestsPerSecond, maxRequestsPerSecond)
	}

	return &RateLimiter{
		client:   client,
		key:      config.Key,
		maxCount: config.MaxCount,
		timeUnit: config.TimeUnit,
	}, nil
}

// generateTimeKey 生成包含时间单位的key
func (rl *RateLimiter) generateTimeKey() string {
	now := time.Now()
	var timeKey string

	switch rl.timeUnit {
	case 24 * time.Hour: // 按天
		timeKey = now.Format("20060102")
	case time.Hour: // 按小时
		timeKey = now.Format("2006010215")
	case time.Minute: // 按分钟
		timeKey = now.Format("200601021504")
	case time.Second: // 按秒
		timeKey = now.Format("20060102150405")
	default: // 按毫秒或其他时间单位
		// 对于毫秒级别的时间单位，使用毫秒时间戳除以时间单位来生成key
		// 确保精度不会丢失
		if rl.timeUnit < time.Second {
			// 毫秒级别：使用毫秒时间戳
			timeKey = fmt.Sprintf("%d", now.UnixMilli()/int64(rl.timeUnit/time.Millisecond))
		} else {
			// 其他时间单位：使用秒时间戳
			timeKey = fmt.Sprintf("%d", now.Unix()/int64(rl.timeUnit/time.Second))
		}
	}

	return fmt.Sprintf("%s:%s", rl.key, timeKey)
}

// calculateExpireTime 计算过期时间（简单地在当前时间基础上加上时间单位，再额外加几秒缓冲）
func (rl *RateLimiter) calculateExpireTime() time.Duration {
	// 基础过期时间 = 时间单位 + 额外缓冲时间
	return rl.timeUnit + time.Second
}

// IsAllowed 检查是否允许请求通过限流（懒加载，无需重复检查）
// 返回是否允许，剩余次数，以及错误信息
func (rl *RateLimiter) IsAllowed(ctx context.Context) (bool, int64, error) {
	// 生成包含时间单位的key
	timeKey := rl.generateTimeKey()

	// 计算过期时间
	expireTime := rl.calculateExpireTime()
	expireSeconds := int(expireTime.Seconds())

	// 确保过期时间至少为1秒
	if expireSeconds <= 0 {
		expireSeconds = 1
	}

	// 使用Lua脚本确保原子性操作
	script := `
		local key = KEYS[1]
		local max_count = tonumber(ARGV[1])
		local expire_time = tonumber(ARGV[2])
		
		-- 获取当前剩余次数
		local current_count = redis.call('GET', key)
		
		-- 如果key不存在，初始化为最大值
		if not current_count then
			current_count = max_count
		else
			current_count = tonumber(current_count)
		end
		
		-- 如果剩余次数小于等于0，返回false
		if current_count <= 0 then
			return {0, current_count}
		end
		
		-- 减少剩余次数
		local new_count = current_count - 1
		
		-- 设置新值和过期时间
		redis.call('SETEX', key, expire_time, new_count)
		
		return {1, new_count}
	`

	// 执行Lua脚本
	result, err := rl.client.Eval(ctx, script, []string{timeKey}, rl.maxCount, expireSeconds).Result()
	if err != nil {
		return false, 0, fmt.Errorf("failed to execute rate limit script: %w", err)
	}

	// 解析结果
	results, ok := result.([]interface{})
	if !ok || len(results) != 2 {
		return false, 0, fmt.Errorf("unexpected script result format")
	}

	allowed, ok := results[0].(int64)
	if !ok {
		return false, 0, fmt.Errorf("failed to parse allowed result")
	}

	count, ok := results[1].(int64)
	if !ok {
		return false, 0, fmt.Errorf("failed to parse count result")
	}

	return allowed == 1, count, nil
}

// GetCurrentCount 获取当前剩余次数
func (rl *RateLimiter) GetCurrentCount(ctx context.Context) (int64, error) {
	// 生成包含时间单位的key
	timeKey := rl.generateTimeKey()

	count, err := rl.client.Get(ctx, timeKey).Int64()
	if err != nil {
		if err == redis.Nil {
			// key不存在，返回最大值（表示还未初始化）
			return rl.maxCount, nil
		}
		return 0, fmt.Errorf("failed to get current count: %w", err)
	}

	return count, nil
}

// ResetRateLimit 重置限流计数器
func (rl *RateLimiter) ResetRateLimit(ctx context.Context) error {
	// 生成包含时间单位的key
	timeKey := rl.generateTimeKey()

	_, err := rl.client.Del(ctx, timeKey).Result()
	if err != nil {
		return fmt.Errorf("failed to reset rate limit: %w", err)
	}

	return nil
}

// IncreaseCount 增加剩余次数（用于补偿或重置）
func (rl *RateLimiter) IncreaseCount(ctx context.Context, increment int64) error {
	if increment <= 0 {
		return errors.New("increment must be greater than 0")
	}

	// 生成包含时间单位的key
	timeKey := rl.generateTimeKey()

	// 使用INCRBY命令增加计数
	_, err := rl.client.IncrBy(ctx, timeKey, increment).Result()
	if err != nil {
		return fmt.Errorf("failed to increase count: %w", err)
	}

	return nil
}

// SetCount 直接设置剩余次数
func (rl *RateLimiter) SetCount(ctx context.Context, count int64) error {
	if count < 0 {
		return errors.New("count cannot be negative")
	}

	// 生成包含时间单位的key
	timeKey := rl.generateTimeKey()

	// 计算过期时间
	expireTime := rl.calculateExpireTime()
	expireSeconds := int(expireTime.Seconds())

	// 确保过期时间至少为1秒
	if expireSeconds <= 0 {
		expireSeconds = 1
	}

	err := rl.client.SetEx(ctx, timeKey, count, time.Duration(expireSeconds)*time.Second).Err()
	if err != nil {
		return fmt.Errorf("failed to set count: %w", err)
	}

	return nil
}

// GetConfig 获取当前配置（只读）
func (rl *RateLimiter) GetConfig() (string, int64, time.Duration) {
	return rl.key, rl.maxCount, rl.timeUnit
}
