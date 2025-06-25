package redis_help

import (
	"context"
	"errors"
	"fmt"
	"time"

	redis "github.com/redis/go-redis/v9"
)

const tokenBucketExpireSeconds = 86400 // 24小时

// TokenBucketRateLimiter 令牌桶限流器结构体
type TokenBucketRateLimiter struct {
	client          redis.UniversalClient
	key             string        // Redis key前缀
	maxTokens       int64         // 最大令牌数
	refillInterval  time.Duration // 令牌补充间隔
	tokensPerRefill int64         // 每次补充的令牌数
}

// TokenBucketConfig 令牌桶配置
type TokenBucketConfig struct {
	Key             string        // Redis key前缀
	MaxTokens       int64         // 最大令牌数
	RefillInterval  time.Duration // 令牌补充间隔
	TokensPerRefill int64         // 每次补充的令牌数（可选，默认等于MaxTokens）
}

// NewTokenBucketRateLimiter 创建新的令牌桶限流器
func NewTokenBucketRateLimiter(client redis.UniversalClient, config TokenBucketConfig) (*TokenBucketRateLimiter, error) {
	// 参数验证
	if client == nil {
		return nil, errors.New("redis client cannot be nil")
	}
	if config.MaxTokens <= 0 {
		return nil, errors.New("max tokens must be greater than 0")
	}
	if config.RefillInterval <= 0 {
		return nil, errors.New("refill interval must be greater than 0")
	}
	if config.Key == "" {
		return nil, errors.New("key cannot be empty")
	}

	// 如果未指定每次补充的令牌数，默认等于最大令牌数
	tokensPerRefill := config.TokensPerRefill
	if tokensPerRefill <= 0 {
		tokensPerRefill = config.MaxTokens
	}

	// 检查配置合理性：确保过期时间不超过24小时
	// 过期时间 = 最大令牌数 * 补充间隔 / 每次补充的令牌数
	expireTime := int64(config.MaxTokens) * int64(config.RefillInterval.Seconds()) / tokensPerRefill
	if expireTime > 86400 { // 24小时 = 86400秒
		return nil, fmt.Errorf("configuration would result in expire time of %d seconds (>24h), please adjust max tokens, refill interval, or tokens per refill", expireTime)
	}

	return &TokenBucketRateLimiter{
		client:          client,
		key:             config.Key,
		maxTokens:       config.MaxTokens,
		refillInterval:  config.RefillInterval,
		tokensPerRefill: tokensPerRefill,
	}, nil
}

// generateKeys 生成Redis key
func (tbrl *TokenBucketRateLimiter) generateKeys(userId string) (string, string) {
	tokenKey := fmt.Sprintf("%s:tokens:%s", tbrl.key, userId)
	timeKey := fmt.Sprintf("%s:time:%s", tbrl.key, userId)
	return tokenKey, timeKey
}

// IsAllowed 检查是否允许请求通过限流
// 返回是否允许，当前令牌数，以及错误信息
func (tbrl *TokenBucketRateLimiter) IsAllowed(ctx context.Context, userId string) (bool, int64, error) {
	if userId == "" {
		return false, 0, errors.New("user id cannot be empty")
	}

	tokenKey, timeKey := tbrl.generateKeys(userId)
	currentTime := time.Now().Unix()

	// Lua脚本，过期时间直接用常量
	script := `
		local token_key = KEYS[1]
		local time_key = KEYS[2]
		local max_tokens = tonumber(ARGV[1])
		local refill_interval = tonumber(ARGV[2])
		local tokens_per_refill = tonumber(ARGV[3])
		local current_time = tonumber(ARGV[4])
		local expire_time = tonumber(ARGV[5])
		
		local current_tokens = redis.call('GET', token_key)
		local last_refill_time = redis.call('GET', time_key)
		if not current_tokens then
			current_tokens = max_tokens
		else
			current_tokens = tonumber(current_tokens)
		end
		if not last_refill_time then
			last_refill_time = current_time
		else
			last_refill_time = tonumber(last_refill_time)
		end
		local time_passed = current_time - last_refill_time
		local refill_cycles = math.floor(time_passed / refill_interval)
		local tokens_to_add = refill_cycles * tokens_per_refill
		if tokens_to_add > 0 then
			current_tokens = math.min(max_tokens, current_tokens + tokens_to_add)
			last_refill_time = current_time - (time_passed % refill_interval)
		end
		if current_tokens > 0 then
			current_tokens = current_tokens - 1
			redis.call('SETEX', token_key, expire_time, current_tokens)
			redis.call('SETEX', time_key, expire_time, last_refill_time)
			return {1, current_tokens}
		else
			redis.call('SETEX', time_key, expire_time, last_refill_time)
			return {0, current_tokens}
		end
	`

	expireTime := tokenBucketExpireSeconds

	result, err := tbrl.client.Eval(ctx, script, []string{tokenKey, timeKey},
		tbrl.maxTokens, int(tbrl.refillInterval.Seconds()), tbrl.tokensPerRefill, currentTime, expireTime).Result()
	if err != nil {
		return false, 0, fmt.Errorf("failed to execute token bucket script: %w", err)
	}

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

// GetCurrentTokens 获取当前令牌数
func (tbrl *TokenBucketRateLimiter) GetCurrentTokens(ctx context.Context, userId string) (int64, error) {
	if userId == "" {
		return 0, errors.New("user id cannot be empty")
	}

	tokenKey, timeKey := tbrl.generateKeys(userId)
	currentTime := time.Now().Unix()

	script := `
		local token_key = KEYS[1]
		local time_key = KEYS[2]
		local max_tokens = tonumber(ARGV[1])
		local refill_interval = tonumber(ARGV[2])
		local tokens_per_refill = tonumber(ARGV[3])
		local current_time = tonumber(ARGV[4])
		local expire_time = tonumber(ARGV[5])
		local current_tokens = redis.call('GET', token_key)
		local last_refill_time = redis.call('GET', time_key)
		if not current_tokens then
			current_tokens = max_tokens
		else
			current_tokens = tonumber(current_tokens)
		end
		if not last_refill_time then
			last_refill_time = current_time
		else
			last_refill_time = tonumber(last_refill_time)
		end
		local time_passed = current_time - last_refill_time
		local refill_cycles = math.floor(time_passed / refill_interval)
		local tokens_to_add = refill_cycles * tokens_per_refill
		if tokens_to_add > 0 then
			current_tokens = math.min(max_tokens, current_tokens + tokens_to_add)
			last_refill_time = current_time - (time_passed % refill_interval)
			redis.call('SETEX', token_key, expire_time, current_tokens)
			redis.call('SETEX', time_key, expire_time, last_refill_time)
		end
		return current_tokens
	`

	expireTime := tokenBucketExpireSeconds

	result, err := tbrl.client.Eval(ctx, script, []string{tokenKey, timeKey},
		tbrl.maxTokens, int(tbrl.refillInterval.Seconds()), tbrl.tokensPerRefill, currentTime, expireTime).Result()
	if err != nil {
		return 0, fmt.Errorf("failed to get current tokens: %w", err)
	}
	tokens, ok := result.(int64)
	if !ok {
		return 0, fmt.Errorf("failed to parse tokens result")
	}
	return tokens, nil
}

// ResetTokens 重置用户令牌桶
func (tbrl *TokenBucketRateLimiter) ResetTokens(ctx context.Context, userId string) error {
	if userId == "" {
		return errors.New("user id cannot be empty")
	}

	tokenKey, timeKey := tbrl.generateKeys(userId)

	// 删除现有的令牌和时间记录
	_, err := tbrl.client.Del(ctx, tokenKey, timeKey).Result()
	if err != nil {
		return fmt.Errorf("failed to reset tokens: %w", err)
	}

	return nil
}

// AddTokens 手动添加令牌
func (tbrl *TokenBucketRateLimiter) AddTokens(ctx context.Context, userId string, tokens int64) error {
	if userId == "" {
		return errors.New("user id cannot be empty")
	}
	if tokens <= 0 {
		return errors.New("tokens must be greater than 0")
	}

	tokenKey, _ := tbrl.generateKeys(userId)

	script := `
		local token_key = KEYS[1]
		local max_tokens = tonumber(ARGV[1])
		local tokens_to_add = tonumber(ARGV[2])
		local expire_time = tonumber(ARGV[3])
		local current_tokens = redis.call('GET', token_key)
		if not current_tokens then
			current_tokens = max_tokens
		else
			current_tokens = tonumber(current_tokens)
		end
		local new_tokens = math.min(max_tokens, current_tokens + tokens_to_add)
		redis.call('SETEX', token_key, expire_time, new_tokens)
		return new_tokens
	`

	expireTime := tokenBucketExpireSeconds

	_, err := tbrl.client.Eval(ctx, script, []string{tokenKey}, tbrl.maxTokens, tokens, expireTime).Result()
	if err != nil {
		return fmt.Errorf("failed to add tokens: %w", err)
	}
	return nil
}

// SetTokens 直接设置令牌数
func (tbrl *TokenBucketRateLimiter) SetTokens(ctx context.Context, userId string, tokens int64) error {
	if userId == "" {
		return errors.New("user id cannot be empty")
	}
	if tokens < 0 {
		return errors.New("tokens cannot be negative")
	}
	if tokens > tbrl.maxTokens {
		return fmt.Errorf("tokens cannot exceed max tokens (%d)", tbrl.maxTokens)
	}

	tokenKey, _ := tbrl.generateKeys(userId)

	expireTime := tokenBucketExpireSeconds

	err := tbrl.client.SetEx(ctx, tokenKey, tokens, time.Duration(expireTime)*time.Second).Err()
	if err != nil {
		return fmt.Errorf("failed to set tokens: %w", err)
	}
	return nil
}

// GetConfig 获取当前配置（只读）
func (tbrl *TokenBucketRateLimiter) GetConfig() (string, int64, time.Duration, int64) {
	return tbrl.key, tbrl.maxTokens, tbrl.refillInterval, tbrl.tokensPerRefill
}
