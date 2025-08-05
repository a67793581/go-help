package redis_help

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
)

// 清理测试数据
func cleanupTestData(client redis.UniversalClient, key string) {
	ctx := context.Background()
	pattern := key + ":*"
	keys, err := client.Keys(ctx, pattern).Result()
	if err == nil {
		for _, k := range keys {
			client.Del(ctx, k)
		}
	}
}

func TestLeakyBucketRateLimiter(t *testing.T) {
	// 创建测试用的Redis服务
	s, err := miniredis.Run()
	if err != nil {
		t.Fatalf("Failed to start miniredis: %v", err)
	}
	defer s.Close()

	// 创建测试用的Redis客户端
	client := redis.NewClient(&redis.Options{
		Addr: s.Addr(),
	})
	defer client.Close()

	ctx := context.Background()

	t.Run("Test Leaky Bucket Initialization", func(t *testing.T) {
		// 测试正常初始化
		config := LeakyBucketConfig{
			Key:      "test:leaky:bucket:init",
			Rate:     2,
			Capacity: 10,
		}

		limiter, err := NewLeakyBucketRateLimiter(client, config)
		if err != nil {
			t.Errorf("NewLeakyBucketRateLimiter() error = %v", err)
			return
		}

		if limiter == nil {
			t.Fatal("Rate limiter should not be nil")
		}

		// 验证配置
		key, rate, capacity := limiter.GetConfig()
		if key != config.Key {
			t.Errorf("Expected key %s, got %s", config.Key, key)
		}
		if rate != config.Rate {
			t.Errorf("Expected rate %d, got %d", config.Rate, rate)
		}
		if capacity != config.Capacity {
			t.Errorf("Expected capacity %d, got %d", config.Capacity, capacity)
		}

		// 清理测试数据
		defer cleanupTestData(client, config.Key)
	})

	t.Run("Test Invalid Initialization", func(t *testing.T) {
		// 测试nil client
		config := LeakyBucketConfig{
			Key:      "test:leaky:bucket:invalid",
			Rate:     2,
			Capacity: 10,
		}
		_, err := NewLeakyBucketRateLimiter(nil, config)
		if err == nil {
			t.Error("Expected error when client is nil")
		}

		// 测试空key
		_, err = NewLeakyBucketRateLimiter(client, LeakyBucketConfig{})
		if err == nil {
			t.Error("Expected error when key is empty")
		}

		// 测试无效的Rate
		_, err = NewLeakyBucketRateLimiter(client, LeakyBucketConfig{
			Key:      "test:leaky:bucket:invalid",
			Rate:     0,
			Capacity: 10,
		})
		if err == nil {
			t.Error("Expected error when rate is 0")
		}

		// 测试无效的Capacity
		_, err = NewLeakyBucketRateLimiter(client, LeakyBucketConfig{
			Key:      "test:leaky:bucket:invalid",
			Rate:     2,
			Capacity: 0,
		})
		if err == nil {
			t.Error("Expected error when capacity is 0")
		}
	})

	t.Run("Test Basic Leaky Bucket Functionality", func(t *testing.T) {
		config := LeakyBucketConfig{
			Key:      "test:leaky:bucket:basic",
			Rate:     1, // 每秒漏出1个请求
			Capacity: 5, // 桶容量为5
		}

		limiter, err := NewLeakyBucketRateLimiter(client, config)
		if err != nil {
			t.Errorf("NewLeakyBucketRateLimiter() error = %v", err)
			return
		}

		userId := "user123"

		// 清理测试数据
		defer cleanupTestData(client, config.Key)

		// 初始状态应该允许5次请求（桶是满的）
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

		// 第6次请求应该被拒绝（桶空了）
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

	t.Run("Test Token Leakage", func(t *testing.T) {
		config := LeakyBucketConfig{
			Key:      "test:leaky:bucket:leak",
			Rate:     2, // 每秒漏出2个请求
			Capacity: 5, // 桶容量为5
		}

		limiter, err := NewLeakyBucketRateLimiter(client, config)
		if err != nil {
			t.Errorf("NewLeakyBucketRateLimiter() error = %v", err)
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

		// 等待1秒，应该漏出2个令牌
		time.Sleep(time.Second * 1)

		// 应该允许2次请求
		for i := 0; i < 2; i++ {
			allowed, tokens, err := limiter.IsAllowed(ctx, userId)
			if err != nil {
				t.Errorf("IsAllowed() error = %v", err)
				return
			}
			if !allowed {
				t.Errorf("Request after leak %d should be allowed, but was blocked. Tokens: %d", i+1, tokens)
			}
		}

		// 第3次请求应该被拒绝
		allowed, tokens, err := limiter.IsAllowed(ctx, userId)
		if err != nil {
			t.Errorf("IsAllowed() error = %v", err)
			return
		}
		if allowed {
			t.Errorf("Request after leak 3 should be blocked, but was allowed. Tokens: %d", tokens)
		}
	})

	t.Run("Test GetCurrentTokens", func(t *testing.T) {
		config := LeakyBucketConfig{
			Key:      "test:leaky:bucket:current",
			Rate:     1,
			Capacity: 10,
		}

		limiter, err := NewLeakyBucketRateLimiter(client, config)
		if err != nil {
			t.Errorf("NewLeakyBucketRateLimiter() error = %v", err)
			return
		}

		userId := "user123"

		// 清理测试数据
		defer cleanupTestData(client, config.Key)

		// 初始状态应该返回最大容量
		tokens, err := limiter.GetCurrentTokens(ctx, userId)
		if err != nil {
			t.Errorf("GetCurrentTokens() error = %v", err)
			return
		}
		if tokens != config.Capacity {
			t.Errorf("Expected %d tokens, got %d", config.Capacity, tokens)
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

	t.Run("Test ResetBucket", func(t *testing.T) {
		config := LeakyBucketConfig{
			Key:      "test:leaky:bucket:reset",
			Rate:     1,
			Capacity: 5,
		}

		limiter, err := NewLeakyBucketRateLimiter(client, config)
		if err != nil {
			t.Errorf("NewLeakyBucketRateLimiter() error = %v", err)
			return
		}

		userId := "user123"

		// 清理测试数据
		defer cleanupTestData(client, config.Key)

		// 消耗一些令牌
		limiter.IsAllowed(ctx, userId)
		limiter.IsAllowed(ctx, userId)

		// 重置桶
		err = limiter.ResetBucket(ctx, userId)
		if err != nil {
			t.Errorf("ResetBucket() error = %v", err)
			return
		}

		// 重置后应该返回最大容量
		tokens, err := limiter.GetCurrentTokens(ctx, userId)
		if err != nil {
			t.Errorf("GetCurrentTokens() error = %v", err)
			return
		}
		if tokens != config.Capacity {
			t.Errorf("Expected %d tokens after reset, got %d", config.Capacity, tokens)
		}
	})

	t.Run("Test AddTokens", func(t *testing.T) {
		config := LeakyBucketConfig{
			Key:      "test:leaky:bucket:add",
			Rate:     1,
			Capacity: 10,
		}

		limiter, err := NewLeakyBucketRateLimiter(client, config)
		if err != nil {
			t.Errorf("NewLeakyBucketRateLimiter() error = %v", err)
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

		// 测试添加超过容量的令牌
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
		if tokens != config.Capacity {
			t.Errorf("Expected %d tokens, got %d", config.Capacity, tokens)
		}
	})

	t.Run("Test SetTokens", func(t *testing.T) {
		config := LeakyBucketConfig{
			Key:      "test:leaky:bucket:set",
			Rate:     1,
			Capacity: 10,
		}

		limiter, err := NewLeakyBucketRateLimiter(client, config)
		if err != nil {
			t.Errorf("NewLeakyBucketRateLimiter() error = %v", err)
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
		config := LeakyBucketConfig{
			Key:      "test:leaky:bucket:errors",
			Rate:     1,
			Capacity: 10,
		}

		limiter, err := NewLeakyBucketRateLimiter(client, config)
		if err != nil {
			t.Errorf("NewLeakyBucketRateLimiter() error = %v", err)
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

		err = limiter.ResetBucket(ctx, "")
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
			t.Error("Expected error when tokens exceeds capacity")
		}
	})

	t.Run("Test Concurrent Access", func(t *testing.T) {
		config := LeakyBucketConfig{
			Key:      "test:leaky:bucket:concurrent",
			Rate:     5,
			Capacity: 10,
		}

		limiter, err := NewLeakyBucketRateLimiter(client, config)
		if err != nil {
			t.Errorf("NewLeakyBucketRateLimiter() error = %v", err)
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

		// 验证结果：应该只有10个请求被允许（Capacity）
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
		config := LeakyBucketConfig{
			Key:      "test:leaky:bucket:multi",
			Rate:     1,
			Capacity: 5,
		}

		limiter, err := NewLeakyBucketRateLimiter(client, config)
		if err != nil {
			t.Errorf("NewLeakyBucketRateLimiter() error = %v", err)
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

	t.Run("Test Token Leakage Logic", func(t *testing.T) {
		config := LeakyBucketConfig{
			Key:      "test:leaky:bucket:leak_logic",
			Rate:     3, // 每秒漏出3个请求
			Capacity: 10,
		}

		limiter, err := NewLeakyBucketRateLimiter(client, config)
		if err != nil {
			t.Errorf("NewLeakyBucketRateLimiter() error = %v", err)
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

		// 等待1秒，应该漏出3个令牌
		time.Sleep(time.Second * 1)

		// 应该补充了3个令牌
		tokens, err := limiter.GetCurrentTokens(ctx, userId)
		if err != nil {
			t.Errorf("GetCurrentTokens() error = %v", err)
			return
		}
		if tokens != 3 {
			t.Errorf("Expected 3 tokens after leak, got %d", tokens)
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

		// 等待1秒，应该再漏出3个令牌，但不超过容量10
		time.Sleep(time.Second * 1)

		// 应该再补充3个令牌，但不超过容量
		tokens, err = limiter.GetCurrentTokens(ctx, userId)
		if err != nil {
			t.Errorf("GetCurrentTokens() error = %v", err)
			return
		}
		if tokens != 4 {
			t.Errorf("Expected 4 tokens after second leak, got %d", tokens)
		}
	})

	t.Run("Test Rate Limiting Behavior", func(t *testing.T) {
		config := LeakyBucketConfig{
			Key:      "test:leaky:bucket:rate_limit",
			Rate:     1, // 每秒漏出1个请求
			Capacity: 3, // 桶容量为3
		}

		limiter, err := NewLeakyBucketRateLimiter(client, config)
		if err != nil {
			t.Errorf("NewLeakyBucketRateLimiter() error = %v", err)
			return
		}

		userId := "user123"

		// 清理测试数据
		defer cleanupTestData(client, config.Key)

		// 快速发送5个请求
		allowedCount := 0
		for i := 0; i < 5; i++ {
			allowed, _, err := limiter.IsAllowed(ctx, userId)
			if err != nil {
				t.Errorf("IsAllowed() error = %v", err)
				return
			}
			if allowed {
				allowedCount++
			}
		}

		// 应该只有3个请求被允许（桶容量）
		if allowedCount != 3 {
			t.Errorf("Expected 3 allowed requests, got %d", allowedCount)
		}

		// 等待1秒，应该漏出1个令牌
		time.Sleep(time.Second * 1)

		// 应该允许1个请求
		allowed, _, err := limiter.IsAllowed(ctx, userId)
		if err != nil {
			t.Errorf("IsAllowed() error = %v", err)
			return
		}
		if !allowed {
			t.Error("Request after 1 second should be allowed")
		}

		// 第2个请求应该被拒绝
		allowed, _, err = limiter.IsAllowed(ctx, userId)
		if err != nil {
			t.Errorf("IsAllowed() error = %v", err)
			return
		}
		if allowed {
			t.Error("Second request after 1 second should be blocked")
		}
	})
}
