package redis_help

import (
	"context"
	"errors"
	"fmt"
	"time"

	redis "github.com/redis/go-redis/v9"
)

// RateLimiterV2 限流器结构体v2版本
type RateLimiterV2 struct {
	client   redis.UniversalClient
	key      string         // Redis key（不包含时间单位）
	maxCount int64          // 最大允许的请求数量
	timeUnit time.Duration  // 时间单位（如1天、1小时等）
	timezone *time.Location // 时区
}

// RateLimitConfigV2 限流配置v2版本
type RateLimitConfigV2 struct {
	Key      string         // Redis key（不包含时间单位）
	MaxCount int64          // 最大允许的请求数量
	TimeUnit time.Duration  // 时间单位（如1天、1小时等）
	Timezone *time.Location // 时区，默认为UTC
}

// NewRateLimiterV2 创建新的限流器v2版本
func NewRateLimiterV2(client redis.UniversalClient, config RateLimitConfigV2) (*RateLimiterV2, error) {
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
		return nil, fmt.Errorf("time unit cannot exceed 24 days")
	}

	// 设置默认时区为UTC
	tz := config.Timezone
	if tz == nil {
		tz = time.UTC
	}

	return &RateLimiterV2{
		client:   client,
		key:      config.Key,
		maxCount: config.MaxCount,
		timeUnit: config.TimeUnit,
		timezone: tz,
	}, nil
}

// generateTimeKey 生成包含时间单位的key
func (rl *RateLimiterV2) generateTimeKey() string {
	// 使用指定时区的时间
	now := time.Now().In(rl.timezone)
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

// calculateExpireTime 计算过期时间（时间单位的2倍，确保足够长）
func (rl *RateLimiterV2) calculateExpireTime() time.Duration {
	// 过期时间设置为时间单位的2倍，确保在时间单位结束后key还能存在一段时间
	return rl.timeUnit * 2
}

// IsAllowed 检查是否允许请求通过限流（使用增量计数）
// 返回是否允许，剩余次数，以及错误信息
func (rl *RateLimiterV2) IsAllowed(ctx context.Context) (bool, int64, error) {
	// 生成包含时间单位的key
	timeKey := rl.generateTimeKey()

	// 计算过期时间
	expireTime := rl.calculateExpireTime()

	// 使用Lua脚本确保原子性操作（改为增量计数模式）
	script := `
		local key = KEYS[1]
		local max_count = tonumber(ARGV[1])
		local expire_time = tonumber(ARGV[2])
		
		-- 增加计数
		local new_count = redis.call('INCRBY', key, 1)
		
		-- 如果是第一次请求，设置过期时间
		if new_count == 1 then
			redis.call('EXPIRE', key, expire_time)
		end
		
		-- 检查是否超过限制
		if new_count > max_count then
			-- 恢复计数（减1）
			redis.call('DECRBY', key, 1)
			return {0, max_count - (new_count - 1)}
		end
		
		return {1, max_count - new_count}
	`

	// 执行Lua脚本
	result, err := rl.client.Eval(ctx, script, []string{timeKey}, rl.maxCount, int(expireTime.Seconds())).Result()
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

// GetCurrentCount 获取当前已使用次数
func (rl *RateLimiterV2) GetCurrentCount(ctx context.Context) (int64, error) {
	// 生成包含时间单位的key
	timeKey := rl.generateTimeKey()

	count, err := rl.client.Get(ctx, timeKey).Int64()
	if err != nil {
		if err == redis.Nil {
			// key不存在，返回0（表示还未初始化）
			return 0, nil
		}
		return 0, fmt.Errorf("failed to get current count: %w", err)
	}

	return count, nil
}

// GetRemainingCount 获取剩余次数
func (rl *RateLimiterV2) GetRemainingCount(ctx context.Context) (int64, error) {
	current, err := rl.GetCurrentCount(ctx)
	if err != nil {
		return 0, err
	}

	remaining := rl.maxCount - current
	if remaining < 0 {
		return 0, nil
	}
	return remaining, nil
}

// IncreaseCount 增加已使用次数（用于补偿或重置）
func (rl *RateLimiterV2) IncreaseCount(ctx context.Context, increment int64) error {
	if increment <= 0 {
		return errors.New("increment must be greater than 0")
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

	// 使用Lua脚本确保原子性操作
	script := `
		local key = KEYS[1]
		local increment = tonumber(ARGV[1])
		local expire_time = tonumber(ARGV[2])
		
		-- 增加计数
		local new_count = redis.call('INCRBY', key, increment)
		
		-- 设置过期时间
		redis.call('EXPIRE', key, expire_time)
		
		return new_count
	`

	// 执行Lua脚本
	_, err := rl.client.Eval(ctx, script, []string{timeKey}, increment, expireSeconds).Result()
	if err != nil {
		return fmt.Errorf("failed to increase count: %w", err)
	}

	return nil
}

// SetCount 直接设置已使用次数
func (rl *RateLimiterV2) SetCount(ctx context.Context, count int64) error {
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

// ResetRateLimit 重置限流计数器
func (rl *RateLimiterV2) ResetRateLimit(ctx context.Context) error {
	// 生成包含时间单位的key
	timeKey := rl.generateTimeKey()

	_, err := rl.client.Del(ctx, timeKey).Result()
	if err != nil {
		return fmt.Errorf("failed to reset rate limit: %w", err)
	}

	return nil
}

// GetConfig 获取当前配置（只读）
func (rl *RateLimiterV2) GetConfig() (string, int64, time.Duration, *time.Location) {
	return rl.key, rl.maxCount, rl.timeUnit, rl.timezone
}
