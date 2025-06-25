package redis_help

import (
	"context"
	"errors"
	"fmt"
	"time"

	redis "github.com/redis/go-redis/v9"
)

// LeakyBucketRateLimiter 漏桶限流器结构体
type LeakyBucketRateLimiter struct {
	client   redis.UniversalClient
	key      string // Redis key前缀
	rate     int64  // 漏出速率（每秒漏出的请求数）
	capacity int64  // 桶的最大容量
}

// LeakyBucketConfig 漏桶配置
type LeakyBucketConfig struct {
	Key      string // Redis key前缀
	Rate     int64  // 漏出速率（每秒漏出的请求数）
	Capacity int64  // 桶的最大容量
}

// NewLeakyBucketRateLimiter 创建新的漏桶限流器
func NewLeakyBucketRateLimiter(client redis.UniversalClient, config LeakyBucketConfig) (*LeakyBucketRateLimiter, error) {
	// 参数验证
	if client == nil {
		return nil, errors.New("redis client cannot be nil")
	}
	if config.Rate <= 0 {
		return nil, errors.New("rate must be greater than 0")
	}
	if config.Capacity <= 0 {
		return nil, errors.New("capacity must be greater than 0")
	}
	if config.Key == "" {
		return nil, errors.New("key cannot be empty")
	}

	return &LeakyBucketRateLimiter{
		client:   client,
		key:      config.Key,
		rate:     config.Rate,
		capacity: config.Capacity,
	}, nil
}

// generateKey 生成Redis key
func (lbrl *LeakyBucketRateLimiter) generateKey(userId string) string {
	return fmt.Sprintf("%s:%s", lbrl.key, userId)
}

// IsAllowed 检查是否允许请求通过限流
// 返回是否允许，当前桶中水量，以及错误信息
func (lbrl *LeakyBucketRateLimiter) IsAllowed(ctx context.Context, userId string) (bool, int64, error) {
	if userId == "" {
		return false, 0, errors.New("user id cannot be empty")
	}

	key := lbrl.generateKey(userId)
	currentTime := time.Now().Unix()

	// 使用Lua脚本确保原子性操作
	script := `
		local key = KEYS[1]
		local rate = tonumber(ARGV[1])
		local capacity = tonumber(ARGV[2])
		local current_time = tonumber(ARGV[3])
		
		-- 获取桶的当前状态
		local tokens = redis.call('HGET', key, 'tokens')
		local last_time = redis.call('HGET', key, 'last_time')
		
		-- 如果桶为空，则初始化
		if not tokens then
			tokens = capacity
		else
			tokens = tonumber(tokens)
		end
		
		if not last_time then
			last_time = 0
		else
			last_time = tonumber(last_time)
		end
		
		-- 计算时间差，漏出令牌
		local elapsed = current_time - last_time
		local leaked_tokens = elapsed * rate
		tokens = math.min(capacity, tokens + leaked_tokens)
		
		-- 判断是否可以通过请求
		local allowed = 0
		if tokens >= 1 then
			tokens = tokens - 1
			allowed = 1
		end
		
		-- 更新桶的状态
		redis.call('HSET', key, 'tokens', tokens)
		redis.call('HSET', key, 'last_time', current_time)
		
		-- 设置过期时间（桶容量除以速率，确保数据不会永久存储）
		local expire_time = math.ceil(capacity / rate)
		if expire_time > 0 then
			redis.call('EXPIRE', key, expire_time)
		end
		
		return {allowed, tokens}
	`

	// 执行Lua脚本
	result, err := lbrl.client.Eval(ctx, script, []string{key}, lbrl.rate, lbrl.capacity, currentTime).Result()
	if err != nil {
		return false, 0, fmt.Errorf("failed to execute leaky bucket script: %w", err)
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

	tokens, ok := results[1].(int64)
	if !ok {
		return false, 0, fmt.Errorf("failed to parse tokens result")
	}

	return allowed == 1, tokens, nil
}

// GetCurrentTokens 获取当前桶中的水量
func (lbrl *LeakyBucketRateLimiter) GetCurrentTokens(ctx context.Context, userId string) (int64, error) {
	if userId == "" {
		return 0, errors.New("user id cannot be empty")
	}

	key := lbrl.generateKey(userId)
	currentTime := time.Now().Unix()

	// 使用Lua脚本计算当前水量（不消耗令牌）
	script := `
		local key = KEYS[1]
		local rate = tonumber(ARGV[1])
		local capacity = tonumber(ARGV[2])
		local current_time = tonumber(ARGV[3])
		
		-- 获取桶的当前状态
		local tokens = redis.call('HGET', key, 'tokens')
		local last_time = redis.call('HGET', key, 'last_time')
		
		-- 如果桶为空，则初始化
		if not tokens then
			tokens = capacity
		else
			tokens = tonumber(tokens)
		end
		
		if not last_time then
			last_time = 0
		else
			last_time = tonumber(last_time)
		end
		
		-- 计算时间差，漏出令牌
		local elapsed = current_time - last_time
		local leaked_tokens = elapsed * rate
		tokens = math.min(capacity, tokens + leaked_tokens)
		
		-- 更新桶的状态（不消耗令牌，只更新时间和水量）
		redis.call('HSET', key, 'tokens', tokens)
		redis.call('HSET', key, 'last_time', current_time)
		
		-- 设置过期时间
		local expire_time = math.ceil(capacity / rate)
		if expire_time > 0 then
			redis.call('EXPIRE', key, expire_time)
		end
		
		return tokens
	`

	// 执行Lua脚本
	result, err := lbrl.client.Eval(ctx, script, []string{key}, lbrl.rate, lbrl.capacity, currentTime).Result()
	if err != nil {
		return 0, fmt.Errorf("failed to get current tokens: %w", err)
	}

	tokens, ok := result.(int64)
	if !ok {
		return 0, fmt.Errorf("failed to parse tokens result")
	}

	return tokens, nil
}

// ResetBucket 重置用户漏桶
func (lbrl *LeakyBucketRateLimiter) ResetBucket(ctx context.Context, userId string) error {
	if userId == "" {
		return errors.New("user id cannot be empty")
	}

	key := lbrl.generateKey(userId)

	// 删除现有的桶记录
	_, err := lbrl.client.Del(ctx, key).Result()
	if err != nil {
		return fmt.Errorf("failed to reset bucket: %w", err)
	}

	return nil
}

// AddTokens 手动添加水量到桶中
func (lbrl *LeakyBucketRateLimiter) AddTokens(ctx context.Context, userId string, tokens int64) error {
	if userId == "" {
		return errors.New("user id cannot be empty")
	}
	if tokens <= 0 {
		return errors.New("tokens must be greater than 0")
	}

	key := lbrl.generateKey(userId)
	currentTime := time.Now().Unix()

	// 使用Lua脚本确保不超过桶容量
	script := `
		local key = KEYS[1]
		local rate = tonumber(ARGV[1])
		local capacity = tonumber(ARGV[2])
		local tokens_to_add = tonumber(ARGV[3])
		local current_time = tonumber(ARGV[4])
		
		-- 获取桶的当前状态
		local tokens = redis.call('HGET', key, 'tokens')
		local last_time = redis.call('HGET', key, 'last_time')
		
		-- 如果桶为空，则初始化
		if not tokens then
			tokens = capacity
		else
			tokens = tonumber(tokens)
		end
		
		if not last_time then
			last_time = 0
		else
			last_time = tonumber(last_time)
		end
		
		-- 计算时间差，漏出令牌
		local elapsed = current_time - last_time
		local leaked_tokens = elapsed * rate
		tokens = math.min(capacity, tokens + leaked_tokens)
		
		-- 添加令牌，但不超过容量
		tokens = math.min(capacity, tokens + tokens_to_add)
		
		-- 更新桶的状态
		redis.call('HSET', key, 'tokens', tokens)
		redis.call('HSET', key, 'last_time', current_time)
		
		-- 设置过期时间
		local expire_time = math.ceil(capacity / rate)
		if expire_time > 0 then
			redis.call('EXPIRE', key, expire_time)
		end
		
		return tokens
	`

	_, err := lbrl.client.Eval(ctx, script, []string{key}, lbrl.rate, lbrl.capacity, tokens, currentTime).Result()
	if err != nil {
		return fmt.Errorf("failed to add tokens: %w", err)
	}

	return nil
}

// SetTokens 直接设置桶中的水量
func (lbrl *LeakyBucketRateLimiter) SetTokens(ctx context.Context, userId string, tokens int64) error {
	if userId == "" {
		return errors.New("user id cannot be empty")
	}
	if tokens < 0 {
		return errors.New("tokens cannot be negative")
	}
	if tokens > lbrl.capacity {
		return fmt.Errorf("tokens cannot exceed capacity (%d)", lbrl.capacity)
	}

	key := lbrl.generateKey(userId)
	currentTime := time.Now().Unix()

	// 使用Lua脚本设置水量
	script := `
		local key = KEYS[1]
		local rate = tonumber(ARGV[1])
		local capacity = tonumber(ARGV[2])
		local tokens_to_set = tonumber(ARGV[3])
		local current_time = tonumber(ARGV[4])
		
		-- 设置桶的状态
		redis.call('HSET', key, 'tokens', tokens_to_set)
		redis.call('HSET', key, 'last_time', current_time)
		
		-- 设置过期时间
		local expire_time = math.ceil(capacity / rate)
		if expire_time > 0 then
			redis.call('EXPIRE', key, expire_time)
		end
		
		return tokens_to_set
	`

	_, err := lbrl.client.Eval(ctx, script, []string{key}, lbrl.rate, lbrl.capacity, tokens, currentTime).Result()
	if err != nil {
		return fmt.Errorf("failed to set tokens: %w", err)
	}

	return nil
}

// GetConfig 获取当前配置（只读）
func (lbrl *LeakyBucketRateLimiter) GetConfig() (string, int64, int64) {
	return lbrl.key, lbrl.rate, lbrl.capacity
}
