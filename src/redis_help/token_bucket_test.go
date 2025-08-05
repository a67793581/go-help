package redis_help

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	redis "github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
)

func TestTokenBucketRateLimiter(t *testing.T) {
	// 创建测试用的Redis服务
	s, err := miniredis.Run()
	if err != nil {
		t.Fatalf("Failed to start miniredis: %v", err)
	}
	defer s.Close()

	// 创建Redis客户端
	client := redis.NewClient(&redis.Options{
		Addr: s.Addr(),
	})
	defer client.Close()

	// 清理测试数据
	client.FlushDB(context.Background())

	// 测试配置合理性检查
	t.Run("配置合理性检查", func(t *testing.T) {
		// 测试过期时间超过24小时的配置
		invalidConfig := TokenBucketConfig{
			Key:             "test_invalid",
			MaxTokens:       1000000, // 100万令牌
			RefillInterval:  time.Hour,
			TokensPerRefill: 1, // 每次只补充1个令牌
		}

		// 计算过期时间：1000000 * 3600 / 1 = 3600000000秒 > 86400秒(24小时)
		_, err := NewTokenBucketRateLimiter(client, invalidConfig)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "expire time")
		assert.Contains(t, err.Error(), ">24h")

		// 测试有效的配置
		validConfig := TokenBucketConfig{
			Key:             "test_valid",
			MaxTokens:       100,
			RefillInterval:  time.Minute,
			TokensPerRefill: 10,
		}

		// 计算过期时间：100 * 60 / 10 = 600秒 < 86400秒
		limiter, err := NewTokenBucketRateLimiter(client, validConfig)
		assert.NoError(t, err)
		assert.NotNil(t, limiter)
	})

	// 测试基本功能
	t.Run("基本功能测试", func(t *testing.T) {
		config := TokenBucketConfig{
			Key:             "test_basic",
			MaxTokens:       10,
			RefillInterval:  time.Second,
			TokensPerRefill: 2,
		}

		limiter, err := NewTokenBucketRateLimiter(client, config)
		assert.NoError(t, err)
		assert.NotNil(t, limiter)

		// 测试初始令牌数
		tokens, err := limiter.GetCurrentTokens(context.Background(), "user1")
		assert.NoError(t, err)
		assert.Equal(t, int64(10), tokens)

		// 测试消耗令牌
		allowed, tokens, err := limiter.IsAllowed(context.Background(), "user1")
		assert.NoError(t, err)
		assert.True(t, allowed)
		assert.Equal(t, int64(9), tokens)

		// 测试令牌补充
		time.Sleep(2 * time.Second) // 等待2秒，应该补充4个令牌
		tokens, err = limiter.GetCurrentTokens(context.Background(), "user1")
		assert.NoError(t, err)
		assert.Equal(t, int64(10), tokens) // 应该回到最大值
	})

	// 测试动态过期时间
	t.Run("动态过期时间测试", func(t *testing.T) {
		config := TokenBucketConfig{
			Key:             "test_expire",
			MaxTokens:       5,
			RefillInterval:  time.Minute,
			TokensPerRefill: 1,
		}

		limiter, err := NewTokenBucketRateLimiter(client, config)
		assert.NoError(t, err)

		// 初始状态：满桶，应该使用较短的过期时间
		allowed, tokens, err := limiter.IsAllowed(context.Background(), "user2")
		assert.NoError(t, err)
		assert.True(t, allowed)
		assert.Equal(t, int64(4), tokens)

		// 消耗所有令牌
		for i := 0; i < 4; i++ {
			allowed, _, err := limiter.IsAllowed(context.Background(), "user2")
			assert.NoError(t, err)
			assert.True(t, allowed)
		}

		// 现在桶空了，应该使用较长的过期时间
		allowed, tokens, err = limiter.IsAllowed(context.Background(), "user2")
		assert.NoError(t, err)
		assert.False(t, allowed)
		assert.Equal(t, int64(0), tokens)
	})

	ctx := context.Background()

	t.Run("Test Token Bucket Initialization", func(t *testing.T) {
		// 测试正常初始化
		config := TokenBucketConfig{
			Key:             "test:token:bucket:init",
			MaxTokens:       10,
			RefillInterval:  time.Minute,
			TokensPerRefill: 5,
		}

		limiter, err := NewTokenBucketRateLimiter(client, config)
		if err != nil {
			t.Errorf("NewTokenBucketRateLimiter() error = %v", err)
			return
		}

		if limiter == nil {
			t.Fatal("Rate limiter should not be nil")
		}

		// 验证配置
		key, maxTokens, refillInterval, tokensPerRefill := limiter.GetConfig()
		if key != config.Key {
			t.Errorf("Expected key %s, got %s", config.Key, key)
		}
		if maxTokens != config.MaxTokens {
			t.Errorf("Expected max tokens %d, got %d", config.MaxTokens, maxTokens)
		}
		if refillInterval != config.RefillInterval {
			t.Errorf("Expected refill interval %v, got %v", config.RefillInterval, refillInterval)
		}
		if tokensPerRefill != config.TokensPerRefill {
			t.Errorf("Expected tokens per refill %d, got %d", config.TokensPerRefill, tokensPerRefill)
		}

		// 清理测试数据
		defer cleanupTestData(client, config.Key)
	})

	t.Run("Test Invalid Initialization", func(t *testing.T) {
		// 测试nil client
		config := TokenBucketConfig{
			Key:             "test:token:bucket:invalid",
			MaxTokens:       10,
			RefillInterval:  time.Minute,
			TokensPerRefill: 5,
		}
		_, err := NewTokenBucketRateLimiter(nil, config)
		if err == nil {
			t.Error("Expected error when client is nil")
		}

		// 测试空key
		_, err = NewTokenBucketRateLimiter(client, TokenBucketConfig{})
		if err == nil {
			t.Error("Expected error when key is empty")
		}

		// 测试无效的MaxTokens
		_, err = NewTokenBucketRateLimiter(client, TokenBucketConfig{
			Key:            "test:token:bucket:invalid",
			MaxTokens:      0,
			RefillInterval: time.Minute,
		})
		if err == nil {
			t.Error("Expected error when max tokens is 0")
		}

		// 测试无效的RefillInterval
		_, err = NewTokenBucketRateLimiter(client, TokenBucketConfig{
			Key:            "test:token:bucket:invalid",
			MaxTokens:      10,
			RefillInterval: 0,
		})
		if err == nil {
			t.Error("Expected error when refill interval is 0")
		}
	})

	t.Run("Test Default Tokens Per Refill", func(t *testing.T) {
		config := TokenBucketConfig{
			Key:            "test:token:bucket:default",
			MaxTokens:      10,
			RefillInterval: time.Minute,
			// 不设置TokensPerRefill，应该默认为MaxTokens
		}

		limiter, err := NewTokenBucketRateLimiter(client, config)
		if err != nil {
			t.Errorf("NewTokenBucketRateLimiter() error = %v", err)
			return
		}

		_, _, _, tokensPerRefill := limiter.GetConfig()
		if tokensPerRefill != config.MaxTokens {
			t.Errorf("Expected tokens per refill to default to %d, got %d", config.MaxTokens, tokensPerRefill)
		}

		// 清理测试数据
		defer cleanupTestData(client, config.Key)
	})

	t.Run("Test Basic Token Bucket Functionality", func(t *testing.T) {
		config := TokenBucketConfig{
			Key:             "test:token:bucket:basic",
			MaxTokens:       5,
			RefillInterval:  time.Second * 2,
			TokensPerRefill: 2,
		}

		limiter, err := NewTokenBucketRateLimiter(client, config)
		if err != nil {
			t.Errorf("NewTokenBucketRateLimiter() error = %v", err)
			return
		}

		userId := "user123"

		// 清理测试数据
		defer cleanupTestData(client, config.Key)

		// 测试初始状态 - 应该允许5次请求
		for i := 0; i < 5; i++ {
			allowed, tokens, err := limiter.IsAllowed(ctx, userId)
			if err != nil {
				t.Errorf("IsAllowed() error = %v", err)
				return
			}
			if !allowed {
				t.Errorf("Request %d should be allowed, but was blocked. Tokens: %d", i+1, tokens)
			}
			expectedTokens := int64(4 - i)
			if tokens != expectedTokens {
				t.Errorf("Expected %d tokens, got %d", expectedTokens, tokens)
			}
		}

		// 第6次请求应该被拒绝
		allowed, tokens, err := limiter.IsAllowed(ctx, userId)
		if err != nil {
			t.Errorf("IsAllowed() error = %v", err)
			return
		}
		if allowed {
			t.Errorf("Request 6 should be blocked, but was allowed. Tokens: %d", tokens)
		}
		if tokens != 0 {
			t.Errorf("Expected 0 tokens, got %d", tokens)
		}
	})

	t.Run("Test Token Refill", func(t *testing.T) {
		config := TokenBucketConfig{
			Key:             "test:token:bucket:refill",
			MaxTokens:       5,
			RefillInterval:  time.Second * 2,
			TokensPerRefill: 2,
		}

		limiter, err := NewTokenBucketRateLimiter(client, config)
		if err != nil {
			t.Errorf("NewTokenBucketRateLimiter() error = %v", err)
			return
		}

		userId := "user123"

		// 清理测试数据
		defer cleanupTestData(client, config.Key)

		// 消耗所有令牌
		for i := 0; i < 5; i++ {
			_, _, err := limiter.IsAllowed(ctx, userId)
			if err != nil {
				t.Errorf("IsAllowed() error = %v", err)
				return
			}
		}

		// 等待令牌补充
		time.Sleep(time.Second * 3)

		// 应该允许2次请求（补充了2个令牌）
		for i := 0; i < 2; i++ {
			allowed, tokens, err := limiter.IsAllowed(ctx, userId)
			if err != nil {
				t.Errorf("IsAllowed() error = %v", err)
				return
			}
			if !allowed {
				t.Errorf("Request after refill %d should be allowed, but was blocked. Tokens: %d", i+1, tokens)
			}
		}

		// 第3次请求应该被拒绝
		allowed, tokens, err := limiter.IsAllowed(ctx, userId)
		if err != nil {
			t.Errorf("IsAllowed() error = %v", err)
			return
		}
		if allowed {
			t.Errorf("Request after refill 3 should be blocked, but was allowed. Tokens: %d", tokens)
		}
	})

	t.Run("Test GetCurrentTokens", func(t *testing.T) {
		config := TokenBucketConfig{
			Key:             "test:token:bucket:current",
			MaxTokens:       10,
			RefillInterval:  time.Second * 2,
			TokensPerRefill: 3,
		}

		limiter, err := NewTokenBucketRateLimiter(client, config)
		if err != nil {
			t.Errorf("NewTokenBucketRateLimiter() error = %v", err)
			return
		}

		userId := "user123"

		// 清理测试数据
		defer cleanupTestData(client, config.Key)

		// 初始状态应该返回最大令牌数
		tokens, err := limiter.GetCurrentTokens(ctx, userId)
		if err != nil {
			t.Errorf("GetCurrentTokens() error = %v", err)
			return
		}
		if tokens != config.MaxTokens {
			t.Errorf("Expected %d tokens, got %d", config.MaxTokens, tokens)
		}

		// 消耗一些令牌
		limiter.IsAllowed(ctx, userId)
		limiter.IsAllowed(ctx, userId)

		// 检查剩余令牌数
		tokens, err = limiter.GetCurrentTokens(ctx, userId)
		if err != nil {
			t.Errorf("GetCurrentTokens() error = %v", err)
			return
		}
		if tokens != 8 {
			t.Errorf("Expected 8 tokens, got %d", tokens)
		}
	})

	t.Run("Test ResetTokens", func(t *testing.T) {
		config := TokenBucketConfig{
			Key:             "test:token:bucket:reset",
			MaxTokens:       5,
			RefillInterval:  time.Minute,
			TokensPerRefill: 2,
		}

		limiter, err := NewTokenBucketRateLimiter(client, config)
		if err != nil {
			t.Errorf("NewTokenBucketRateLimiter() error = %v", err)
			return
		}

		userId := "user123"

		// 清理测试数据
		defer cleanupTestData(client, config.Key)

		// 消耗一些令牌
		limiter.IsAllowed(ctx, userId)
		limiter.IsAllowed(ctx, userId)

		// 重置令牌
		err = limiter.ResetTokens(ctx, userId)
		if err != nil {
			t.Errorf("ResetTokens() error = %v", err)
			return
		}

		// 重置后应该返回最大令牌数
		tokens, err := limiter.GetCurrentTokens(ctx, userId)
		if err != nil {
			t.Errorf("GetCurrentTokens() error = %v", err)
			return
		}
		if tokens != config.MaxTokens {
			t.Errorf("Expected %d tokens after reset, got %d", config.MaxTokens, tokens)
		}
	})

	t.Run("Test AddTokens", func(t *testing.T) {
		config := TokenBucketConfig{
			Key:             "test:token:bucket:add",
			MaxTokens:       10,
			RefillInterval:  time.Minute,
			TokensPerRefill: 2,
		}

		limiter, err := NewTokenBucketRateLimiter(client, config)
		if err != nil {
			t.Errorf("NewTokenBucketRateLimiter() error = %v", err)
			return
		}

		userId := "user123"

		// 清理测试数据
		defer cleanupTestData(client, config.Key)

		// 消耗所有令牌
		for i := 0; i < 10; i++ {
			limiter.IsAllowed(ctx, userId)
		}

		// 添加3个令牌
		err = limiter.AddTokens(ctx, userId, 3)
		if err != nil {
			t.Errorf("AddTokens() error = %v", err)
			return
		}

		// 检查令牌数
		tokens, err := limiter.GetCurrentTokens(ctx, userId)
		if err != nil {
			t.Errorf("GetCurrentTokens() error = %v", err)
			return
		}
		if tokens != 3 {
			t.Errorf("Expected 3 tokens, got %d", tokens)
		}

		// 测试添加超过最大值的令牌
		err = limiter.AddTokens(ctx, userId, 10)
		if err != nil {
			t.Errorf("AddTokens() error = %v", err)
			return
		}

		tokens, err = limiter.GetCurrentTokens(ctx, userId)
		if err != nil {
			t.Errorf("GetCurrentTokens() error = %v", err)
			return
		}
		if tokens != config.MaxTokens {
			t.Errorf("Expected %d tokens, got %d", config.MaxTokens, tokens)
		}
	})

	t.Run("Test SetTokens", func(t *testing.T) {
		config := TokenBucketConfig{
			Key:             "test:token:bucket:set",
			MaxTokens:       10,
			RefillInterval:  time.Minute,
			TokensPerRefill: 2,
		}

		limiter, err := NewTokenBucketRateLimiter(client, config)
		if err != nil {
			t.Errorf("NewTokenBucketRateLimiter() error = %v", err)
			return
		}

		userId := "user123"

		// 清理测试数据
		defer cleanupTestData(client, config.Key)

		// 设置令牌数为5
		err = limiter.SetTokens(ctx, userId, 5)
		if err != nil {
			t.Errorf("SetTokens() error = %v", err)
			return
		}

		// 检查令牌数
		tokens, err := limiter.GetCurrentTokens(ctx, userId)
		if err != nil {
			t.Errorf("GetCurrentTokens() error = %v", err)
			return
		}
		if tokens != 5 {
			t.Errorf("Expected 5 tokens, got %d", tokens)
		}

		// 使用一次，剩余4次
		allowed, remaining, err := limiter.IsAllowed(ctx, userId)
		if err != nil {
			t.Errorf("IsAllowed() error = %v", err)
			return
		}
		if !allowed {
			t.Error("Request should be allowed")
		}
		if remaining != 4 {
			t.Errorf("Expected remaining 4, got %d", remaining)
		}
	})

	t.Run("Test Error Cases", func(t *testing.T) {
		config := TokenBucketConfig{
			Key:             "test:token:bucket:errors",
			MaxTokens:       10,
			RefillInterval:  time.Minute,
			TokensPerRefill: 2,
		}

		limiter, err := NewTokenBucketRateLimiter(client, config)
		if err != nil {
			t.Errorf("NewTokenBucketRateLimiter() error = %v", err)
			return
		}

		userId := "user123"

		// 清理测试数据
		defer cleanupTestData(client, config.Key)

		// 测试空用户ID
		_, _, err = limiter.IsAllowed(ctx, "")
		if err == nil {
			t.Error("Expected error when user ID is empty")
		}

		_, err = limiter.GetCurrentTokens(ctx, "")
		if err == nil {
			t.Error("Expected error when user ID is empty")
		}

		err = limiter.ResetTokens(ctx, "")
		if err == nil {
			t.Error("Expected error when user ID is empty")
		}

		err = limiter.AddTokens(ctx, "", 3)
		if err == nil {
			t.Error("Expected error when user ID is empty")
		}

		err = limiter.SetTokens(ctx, "", 5)
		if err == nil {
			t.Error("Expected error when user ID is empty")
		}

		// 测试无效的令牌数
		err = limiter.AddTokens(ctx, userId, 0)
		if err == nil {
			t.Error("Expected error when tokens is 0")
		}

		err = limiter.AddTokens(ctx, userId, -1)
		if err == nil {
			t.Error("Expected error when tokens is negative")
		}

		err = limiter.SetTokens(ctx, userId, -1)
		if err == nil {
			t.Error("Expected error when tokens is negative")
		}

		err = limiter.SetTokens(ctx, userId, 15)
		if err == nil {
			t.Error("Expected error when tokens exceeds max tokens")
		}
	})

	t.Run("Test Concurrent Access", func(t *testing.T) {
		config := TokenBucketConfig{
			Key:             "test:token:bucket:concurrent",
			MaxTokens:       10,
			RefillInterval:  time.Second * 5,
			TokensPerRefill: 2,
		}

		limiter, err := NewTokenBucketRateLimiter(client, config)
		if err != nil {
			t.Errorf("NewTokenBucketRateLimiter() error = %v", err)
			return
		}

		userId := "user123"

		// 清理测试数据
		defer cleanupTestData(client, config.Key)

		// 使用10个goroutine并发访问
		const numGoroutines = 10
		const requestsPerGoroutine = 2
		totalRequests := numGoroutines * requestsPerGoroutine

		results := make(chan bool, totalRequests)
		errors := make(chan error, totalRequests)

		// 启动并发goroutine
		for i := 0; i < numGoroutines; i++ {
			go func(id int) {
				for j := 0; j < requestsPerGoroutine; j++ {
					allowed, _, err := limiter.IsAllowed(ctx, userId)
					if err != nil {
						errors <- fmt.Errorf("goroutine %d request %d error: %w", id, j, err)
						return
					}
					results <- allowed
				}
			}(i)
		}

		// 收集结果
		allowedCount := 0
		for i := 0; i < totalRequests; i++ {
			select {
			case allowed := <-results:
				if allowed {
					allowedCount++
				}
			case err := <-errors:
				t.Errorf("Concurrent test error: %v", err)
				return
			}
		}

		// 验证结果：应该只有10个请求被允许（MaxTokens）
		if allowedCount != 10 {
			t.Errorf("Expected 10 allowed requests, got %d", allowedCount)
		}

		// 验证剩余令牌数
		tokens, err := limiter.GetCurrentTokens(ctx, userId)
		if err != nil {
			t.Errorf("GetCurrentTokens() error = %v", err)
			return
		}
		if tokens != 0 {
			t.Errorf("Expected 0 tokens, got %d", tokens)
		}
	})

	t.Run("Test Multiple Users", func(t *testing.T) {
		config := TokenBucketConfig{
			Key:             "test:token:bucket:multi",
			MaxTokens:       5,
			RefillInterval:  time.Minute,
			TokensPerRefill: 2,
		}

		limiter, err := NewTokenBucketRateLimiter(client, config)
		if err != nil {
			t.Errorf("NewTokenBucketRateLimiter() error = %v", err)
			return
		}

		user1 := "user1"
		user2 := "user2"

		// 清理测试数据
		defer cleanupTestData(client, config.Key)

		// 用户1消耗3个令牌
		for i := 0; i < 3; i++ {
			_, _, err := limiter.IsAllowed(ctx, user1)
			if err != nil {
				t.Errorf("User1 IsAllowed() error = %v", err)
				return
			}
		}

		// 用户2消耗4个令牌
		for i := 0; i < 4; i++ {
			_, _, err := limiter.IsAllowed(ctx, user2)
			if err != nil {
				t.Errorf("User2 IsAllowed() error = %v", err)
				return
			}
		}

		// 检查用户1的剩余令牌数
		tokens1, err := limiter.GetCurrentTokens(ctx, user1)
		if err != nil {
			t.Errorf("GetCurrentTokens() error = %v", err)
			return
		}
		if tokens1 != 2 {
			t.Errorf("Expected user1 to have 2 tokens, got %d", tokens1)
		}

		// 检查用户2的剩余令牌数
		tokens2, err := limiter.GetCurrentTokens(ctx, user2)
		if err != nil {
			t.Errorf("GetCurrentTokens() error = %v", err)
			return
		}
		if tokens2 != 1 {
			t.Errorf("Expected user2 to have 1 token, got %d", tokens2)
		}

		// 用户1应该还能使用2次
		for i := 0; i < 2; i++ {
			allowed, _, err := limiter.IsAllowed(ctx, user1)
			if err != nil {
				t.Errorf("User1 IsAllowed() error = %v", err)
				return
			}
			if !allowed {
				t.Errorf("User1 request %d should be allowed", i+1)
			}
		}

		// 用户1第3次请求应该被拒绝
		allowed, _, err := limiter.IsAllowed(ctx, user1)
		if err != nil {
			t.Errorf("User1 IsAllowed() error = %v", err)
			return
		}
		if allowed {
			t.Error("User1 should be blocked after using all tokens")
		}
	})

	t.Run("Test Token Refill Logic", func(t *testing.T) {
		config := TokenBucketConfig{
			Key:             "test:token:bucket:refill_logic",
			MaxTokens:       10,
			RefillInterval:  time.Second * 3,
			TokensPerRefill: 3,
		}

		limiter, err := NewTokenBucketRateLimiter(client, config)
		if err != nil {
			t.Errorf("NewTokenBucketRateLimiter() error = %v", err)
			return
		}

		userId := "user123"

		// 清理测试数据
		defer cleanupTestData(client, config.Key)

		// 消耗所有令牌
		for i := 0; i < 10; i++ {
			_, _, err := limiter.IsAllowed(ctx, userId)
			if err != nil {
				t.Errorf("IsAllowed() error = %v", err)
				return
			}
		}

		// 等待一个补充周期
		time.Sleep(time.Second * 4)

		// 应该补充了3个令牌
		tokens, err := limiter.GetCurrentTokens(ctx, userId)
		if err != nil {
			t.Errorf("GetCurrentTokens() error = %v", err)
			return
		}
		if tokens != 3 {
			t.Errorf("Expected 3 tokens after refill, got %d", tokens)
		}

		// 使用2个令牌
		for i := 0; i < 2; i++ {
			_, _, err := limiter.IsAllowed(ctx, userId)
			if err != nil {
				t.Errorf("IsAllowed() error = %v", err)
				return
			}
		}

		// 剩余1个令牌
		tokens, err = limiter.GetCurrentTokens(ctx, userId)
		if err != nil {
			t.Errorf("GetCurrentTokens() error = %v", err)
			return
		}
		if tokens != 1 {
			t.Errorf("Expected 1 token remaining, got %d", tokens)
		}

		// 等待另一个补充周期
		time.Sleep(time.Second * 4)

		// 应该再补充3个令牌，但不超过最大值10
		tokens, err = limiter.GetCurrentTokens(ctx, userId)
		if err != nil {
			t.Errorf("GetCurrentTokens() error = %v", err)
			return
		}
		if tokens != 4 {
			t.Errorf("Expected 4 tokens after second refill, got %d", tokens)
		}
	})
}
