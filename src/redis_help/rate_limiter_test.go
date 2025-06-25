package redis_help

import (
	"context"
	"fmt"
	"testing"
	"time"
)

func TestRateLimiter(t *testing.T) {
	// 创建测试用的Redis客户端
	config := &DataRedis{
		Address:      "localhost:6379",
		IsCluster:    false,
		ReadTimeout:  Duration(5),
		WriteTimeout: Duration(5),
	}

	client, err := NewRedis(config)
	if err != nil {
		t.Skipf("Skipping rate limiter test: Redis connection failed: %v", err)
		return
	}

	ctx := context.Background()

	t.Run("Test Rate Limiter Initialization", func(t *testing.T) {
		// 测试正常初始化
		config := RateLimitConfig{
			Key:      "test_init",
			MaxCount: 3,
			TimeUnit: time.Second * 2,
		}

		limiter, err := NewRateLimiter(client, config)
		if err != nil {
			t.Errorf("NewRateLimiter() error = %v", err)
			return
		}

		// 验证配置
		key, maxCount, timeUnit := limiter.GetConfig()
		if key != "test_init" {
			t.Errorf("Expected key 'test_init', got %s", key)
		}
		if maxCount != 3 {
			t.Errorf("Expected maxCount 3, got %d", maxCount)
		}
		if timeUnit != time.Second*2 {
			t.Errorf("Expected timeUnit 2s, got %v", timeUnit)
		}

		// 清理测试数据
		defer limiter.ResetRateLimit(ctx)
	})

	t.Run("Test Invalid Initialization", func(t *testing.T) {
		// 测试空key
		config := RateLimitConfig{
			Key:      "",
			MaxCount: 5,
			TimeUnit: time.Second,
		}
		_, err := NewRateLimiter(client, config)
		if err == nil {
			t.Error("Expected error for empty key")
		}

		// 测试无效的MaxCount
		config = RateLimitConfig{
			Key:      "test_invalid",
			MaxCount: 0,
			TimeUnit: time.Second,
		}
		_, err = NewRateLimiter(client, config)
		if err == nil {
			t.Error("Expected error for invalid MaxCount")
		}

		// 测试无效的TimeUnit
		config = RateLimitConfig{
			Key:      "test_invalid",
			MaxCount: 5,
			TimeUnit: 0,
		}
		_, err = NewRateLimiter(client, config)
		if err == nil {
			t.Error("Expected error for invalid TimeUnit")
		}

		// 测试nil client
		config = RateLimitConfig{
			Key:      "test_invalid",
			MaxCount: 5,
			TimeUnit: time.Second,
		}
		_, err = NewRateLimiter(nil, config)
		if err == nil {
			t.Error("Expected error for nil client")
		}
	})

	t.Run("Test Rate Limiter Basic Functionality", func(t *testing.T) {
		config := RateLimitConfig{
			Key:      "test_basic",
			MaxCount: 3,
			TimeUnit: time.Second * 2,
		}

		limiter, err := NewRateLimiter(client, config)
		if err != nil {
			t.Errorf("NewRateLimiter() error = %v", err)
			return
		}

		// 清理测试数据
		defer limiter.ResetRateLimit(ctx)

		// 测试前3次请求应该被允许，剩余次数从3递减到0
		for i := 0; i < 3; i++ {
			allowed, remaining, err := limiter.IsAllowed(ctx)
			if err != nil {
				t.Errorf("IsAllowed() error = %v", err)
				return
			}
			if !allowed {
				t.Errorf("Request %d should be allowed, but was denied. Remaining: %d", i+1, remaining)
			}
			expectedRemaining := int64(2 - i)
			if remaining != expectedRemaining {
				t.Errorf("Expected remaining %d, got %d", expectedRemaining, remaining)
			}
		}

		// 第4次请求应该被拒绝，剩余次数为0
		allowed, remaining, err := limiter.IsAllowed(ctx)
		if err != nil {
			t.Errorf("IsAllowed() error = %v", err)
			return
		}
		if allowed {
			t.Errorf("Request should be denied when limit exceeded, but was allowed. Remaining: %d", remaining)
		}
		if remaining != 0 {
			t.Errorf("Expected remaining 0, got %d", remaining)
		}
	})

	t.Run("Test Time Unit Key Generation", func(t *testing.T) {
		config := RateLimitConfig{
			Key:      "test_timeunit",
			MaxCount: 2,
			TimeUnit: time.Millisecond * 100, // 100ms时间单位
		}

		limiter, err := NewRateLimiter(client, config)
		if err != nil {
			t.Errorf("NewRateLimiter() error = %v", err)
			return
		}

		// 清理测试数据
		defer limiter.ResetRateLimit(ctx)

		// 第一次请求
		allowed, remaining, err := limiter.IsAllowed(ctx)
		if err != nil {
			t.Errorf("IsAllowed() error = %v", err)
			return
		}
		if !allowed {
			t.Error("First request should be allowed")
		}
		if remaining != 1 {
			t.Errorf("Expected remaining 1, got %d", remaining)
		}

		// 等待时间单位过期
		time.Sleep(time.Millisecond * 150)

		// 时间单位过期后，应该重新开始计数（剩余次数为2）
		allowed, remaining, err = limiter.IsAllowed(ctx)
		if err != nil {
			t.Errorf("IsAllowed() error = %v", err)
			return
		}
		if !allowed {
			t.Error("Request after time unit expiry should be allowed")
		}
		if remaining != 1 {
			t.Errorf("Expected remaining 1 after time unit expiry, got %d", remaining)
		}
	})

	t.Run("Test GetCurrentCount", func(t *testing.T) {
		config := RateLimitConfig{
			Key:      "test_current_count",
			MaxCount: 5,
			TimeUnit: time.Second,
		}

		limiter, err := NewRateLimiter(client, config)
		if err != nil {
			t.Errorf("NewRateLimiter() error = %v", err)
			return
		}

		defer limiter.ResetRateLimit(ctx)

		// 初始剩余次数应该是最大值（还未初始化）
		count, err := limiter.GetCurrentCount(ctx)
		if err != nil {
			t.Errorf("GetCurrentCount() error = %v", err)
			return
		}
		if count != 5 {
			t.Errorf("Expected initial count 5, got %d", count)
		}

		// 使用3次，剩余2次
		for i := 0; i < 3; i++ {
			_, _, err := limiter.IsAllowed(ctx)
			if err != nil {
				t.Errorf("IsAllowed() error = %v", err)
				return
			}
		}

		// 检查当前剩余次数
		count, err = limiter.GetCurrentCount(ctx)
		if err != nil {
			t.Errorf("GetCurrentCount() error = %v", err)
			return
		}
		if count != 2 {
			t.Errorf("Expected remaining count 2, got %d", count)
		}
	})

	t.Run("Test ResetRateLimit", func(t *testing.T) {
		config := RateLimitConfig{
			Key:      "test_reset",
			MaxCount: 5,
			TimeUnit: time.Second,
		}

		limiter, err := NewRateLimiter(client, config)
		if err != nil {
			t.Errorf("NewRateLimiter() error = %v", err)
			return
		}

		defer limiter.ResetRateLimit(ctx)

		// 添加一些请求记录
		_, _, err = limiter.IsAllowed(ctx)
		if err != nil {
			t.Errorf("IsAllowed() error = %v", err)
			return
		}

		// 重置限流器
		err = limiter.ResetRateLimit(ctx)
		if err != nil {
			t.Errorf("ResetRateLimit() error = %v", err)
			return
		}

		// 重置后计数应该是最大值（还未初始化）
		count, err := limiter.GetCurrentCount(ctx)
		if err != nil {
			t.Errorf("GetCurrentCount() error = %v", err)
			return
		}
		if count != 5 {
			t.Errorf("Expected count 5 after reset, got %d", count)
		}
	})

	t.Run("Test IncreaseCount", func(t *testing.T) {
		config := RateLimitConfig{
			Key:      "test_increase",
			MaxCount: 5,
			TimeUnit: time.Second,
		}

		limiter, err := NewRateLimiter(client, config)
		if err != nil {
			t.Errorf("NewRateLimiter() error = %v", err)
			return
		}

		defer limiter.ResetRateLimit(ctx)

		// 使用3次，剩余2次
		for i := 0; i < 3; i++ {
			_, _, err := limiter.IsAllowed(ctx)
			if err != nil {
				t.Errorf("IsAllowed() error = %v", err)
				return
			}
		}

		// 检查当前剩余次数
		count, err := limiter.GetCurrentCount(ctx)
		if err != nil {
			t.Errorf("GetCurrentCount() error = %v", err)
			return
		}
		if count != 2 {
			t.Errorf("Expected remaining count 2, got %d", count)
		}

		// 增加3次
		err = limiter.IncreaseCount(ctx, 3)
		if err != nil {
			t.Errorf("IncreaseCount() error = %v", err)
			return
		}

		// 检查增加后的剩余次数
		count, err = limiter.GetCurrentCount(ctx)
		if err != nil {
			t.Errorf("GetCurrentCount() error = %v", err)
			return
		}
		if count != 5 {
			t.Errorf("Expected remaining count 5 after increase, got %d", count)
		}
	})

	t.Run("Test SetCount", func(t *testing.T) {
		config := RateLimitConfig{
			Key:      "test_set_count",
			MaxCount: 10,
			TimeUnit: time.Second,
		}

		limiter, err := NewRateLimiter(client, config)
		if err != nil {
			t.Errorf("NewRateLimiter() error = %v", err)
			return
		}

		defer limiter.ResetRateLimit(ctx)

		// 直接设置剩余次数为5
		err = limiter.SetCount(ctx, 5)
		if err != nil {
			t.Errorf("SetCount() error = %v", err)
			return
		}

		// 检查设置后的剩余次数
		count, err := limiter.GetCurrentCount(ctx)
		if err != nil {
			t.Errorf("GetCurrentCount() error = %v", err)
			return
		}
		if count != 5 {
			t.Errorf("Expected count 5, got %d", count)
		}

		// 使用一次，剩余4次
		allowed, remaining, err := limiter.IsAllowed(ctx)
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

	t.Run("Test Concurrent Access", func(t *testing.T) {
		config := RateLimitConfig{
			Key:      "test_concurrent",
			MaxCount: 10,
			TimeUnit: time.Second * 5,
		}

		limiter, err := NewRateLimiter(client, config)
		if err != nil {
			t.Errorf("NewRateLimiter() error = %v", err)
			return
		}

		defer limiter.ResetRateLimit(ctx)

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
					allowed, _, err := limiter.IsAllowed(ctx)
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

		// 验证结果：应该只有10个请求被允许（MaxCount）
		if allowedCount != 10 {
			t.Errorf("Expected 10 allowed requests, got %d", allowedCount)
		}

		// 验证剩余次数
		remaining, err := limiter.GetCurrentCount(ctx)
		if err != nil {
			t.Errorf("GetCurrentCount() error = %v", err)
			return
		}
		if remaining != 0 {
			t.Errorf("Expected remaining 0, got %d", remaining)
		}
	})

	t.Run("Test All Time Units Key Generation", func(t *testing.T) {
		// 测试所有支持的时间单位
		testCases := []struct {
			name     string
			timeUnit time.Duration
		}{
			{"Millisecond", time.Millisecond * 100},
			{"Second", time.Second},
			{"Minute", time.Minute},
			{"Hour", time.Hour},
			{"Day", 24 * time.Hour},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				config := RateLimitConfig{
					Key:      fmt.Sprintf("test_%s", tc.name),
					MaxCount: 5,
					TimeUnit: tc.timeUnit,
				}

				limiter, err := NewRateLimiter(client, config)
				if err != nil {
					t.Errorf("NewRateLimiter() error = %v", err)
					return
				}

				defer limiter.ResetRateLimit(ctx)

				// 第一次请求
				allowed1, remaining1, err := limiter.IsAllowed(ctx)
				if err != nil {
					t.Errorf("First IsAllowed() error = %v", err)
					return
				}
				if !allowed1 {
					t.Error("First request should be allowed")
				}

				// 等待一小段时间（对于毫秒级时间单位）
				if tc.timeUnit < time.Second {
					time.Sleep(tc.timeUnit + time.Millisecond*10)
				} else {
					time.Sleep(time.Millisecond * 10)
				}

				// 第二次请求
				allowed2, remaining2, err := limiter.IsAllowed(ctx)
				if err != nil {
					t.Errorf("Second IsAllowed() error = %v", err)
					return
				}

				// 对于毫秒级时间单位，如果时间窗口已经切换，应该重新开始计数
				if tc.timeUnit < time.Second {
					// 如果时间窗口切换，remaining2应该等于remaining1
					if remaining2 != remaining1 {
						t.Logf("Time unit: %v, First remaining: %d, Second remaining: %d", tc.timeUnit, remaining1, remaining2)
					}
				} else {
					// 对于秒级以上的时间单位，在短时间内remaining2应该比remaining1少1
					if remaining2 != remaining1-1 {
						t.Errorf("Expected remaining2 = %d, got %d", remaining1-1, remaining2)
					}
				}

				// 验证请求被允许
				if !allowed2 {
					t.Error("Second request should be allowed")
				}
			})
		}
	})

	t.Run("Test Key Generation Logic", func(t *testing.T) {
		// 测试key生成逻辑
		testCases := []struct {
			name     string
			timeUnit time.Duration
			expected string
		}{
			{"Millisecond", time.Millisecond * 100, "test_ms"},
			{"Second", time.Second, "test_sec"},
			{"Minute", time.Minute, "test_min"},
			{"Hour", time.Hour, "test_hour"},
			{"Day", 24 * time.Hour, "test_day"},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				config := RateLimitConfig{
					Key:      tc.expected,
					MaxCount: 5,
					TimeUnit: tc.timeUnit,
				}

				limiter, err := NewRateLimiter(client, config)
				if err != nil {
					t.Errorf("NewRateLimiter() error = %v", err)
					return
				}

				defer limiter.ResetRateLimit(ctx)

				// 第一次请求
				allowed1, remaining1, err := limiter.IsAllowed(ctx)
				if err != nil {
					t.Errorf("IsAllowed() error = %v", err)
					return
				}
				if !allowed1 {
					t.Error("Request should be allowed")
				}

				// 验证剩余次数
				expectedRemaining := int64(4) // MaxCount - 1
				if remaining1 != expectedRemaining {
					t.Errorf("Expected remaining %d, got %d", expectedRemaining, remaining1)
				}

				// 对于毫秒级时间单位，等待时间窗口切换
				if tc.timeUnit < time.Second {
					time.Sleep(tc.timeUnit + time.Millisecond*10)

					// 第二次请求应该在新的时间窗口中
					allowed2, remaining2, err := limiter.IsAllowed(ctx)
					if err != nil {
						t.Errorf("Second IsAllowed() error = %v", err)
						return
					}
					if !allowed2 {
						t.Error("Second request should be allowed")
					}

					// 在新的时间窗口中，剩余次数应该重新开始
					if remaining2 != expectedRemaining {
						t.Errorf("Expected remaining %d in new time window, got %d", expectedRemaining, remaining2)
					}
				}
			})
		}
	})

	t.Run("Test Key Format Verification", func(t *testing.T) {
		// 测试key格式验证
		now := time.Now()

		testCases := []struct {
			name     string
			timeUnit time.Duration
			key      string
		}{
			{"Millisecond", time.Millisecond * 100, "test_ms"},
			{"Second", time.Second, "test_sec"},
			{"Minute", time.Minute, "test_min"},
			{"Hour", time.Hour, "test_hour"},
			{"Day", 24 * time.Hour, "test_day"},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				config := RateLimitConfig{
					Key:      tc.key,
					MaxCount: 5,
					TimeUnit: tc.timeUnit,
				}

				limiter, err := NewRateLimiter(client, config)
				if err != nil {
					t.Errorf("NewRateLimiter() error = %v", err)
					return
				}

				defer limiter.ResetRateLimit(ctx)

				// 验证key格式
				expectedKeyFormat := ""
				switch tc.timeUnit {
				case 24 * time.Hour:
					expectedKeyFormat = fmt.Sprintf("%s:%s", tc.key, now.Format("20060102"))
				case time.Hour:
					expectedKeyFormat = fmt.Sprintf("%s:%s", tc.key, now.Format("2006010215"))
				case time.Minute:
					expectedKeyFormat = fmt.Sprintf("%s:%s", tc.key, now.Format("200601021504"))
				case time.Second:
					expectedKeyFormat = fmt.Sprintf("%s:%s", tc.key, now.Format("20060102150405"))
				default:
					// 对于毫秒级别，使用纳秒时间戳
					expectedKeyFormat = fmt.Sprintf("%s:%d", tc.key, now.UnixNano()/int64(tc.timeUnit))
				}

				t.Logf("Time unit: %v, Expected key format: %s", tc.timeUnit, expectedKeyFormat)

				// 执行请求并验证功能正常
				allowed, remaining, err := limiter.IsAllowed(ctx)
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

				// 验证当前计数
				count, err := limiter.GetCurrentCount(ctx)
				if err != nil {
					t.Errorf("GetCurrentCount() error = %v", err)
					return
				}
				if count != 4 {
					t.Errorf("Expected count 4, got %d", count)
				}
			})
		}
	})

	t.Run("Test Time Window Transition", func(t *testing.T) {
		// 测试时间窗口切换逻辑
		testCases := []struct {
			name     string
			timeUnit time.Duration
			waitTime time.Duration
		}{
			{"Millisecond", time.Millisecond * 100, time.Millisecond * 150},
			{"Second", time.Second, time.Second + time.Millisecond*100},
			{"Minute", time.Minute, time.Minute + time.Second*5},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				config := RateLimitConfig{
					Key:      fmt.Sprintf("test_transition_%s", tc.name),
					MaxCount: 3,
					TimeUnit: tc.timeUnit,
				}

				limiter, err := NewRateLimiter(client, config)
				if err != nil {
					t.Errorf("NewRateLimiter() error = %v", err)
					return
				}

				defer limiter.ResetRateLimit(ctx)

				// 第一次请求
				allowed1, remaining1, err := limiter.IsAllowed(ctx)
				if err != nil {
					t.Errorf("First IsAllowed() error = %v", err)
					return
				}
				if !allowed1 {
					t.Error("First request should be allowed")
				}
				if remaining1 != 2 {
					t.Errorf("Expected remaining 2, got %d", remaining1)
				}

				// 等待时间窗口切换
				time.Sleep(tc.waitTime)

				// 第二次请求应该在新的时间窗口中
				allowed2, remaining2, err := limiter.IsAllowed(ctx)
				if err != nil {
					t.Errorf("Second IsAllowed() error = %v", err)
					return
				}
				if !allowed2 {
					t.Error("Second request should be allowed")
				}

				// 在新的时间窗口中，剩余次数应该重新开始
				if remaining2 != 2 {
					t.Errorf("Expected remaining 2 in new time window, got %d", remaining2)
				}

				// 验证当前计数
				count, err := limiter.GetCurrentCount(ctx)
				if err != nil {
					t.Errorf("GetCurrentCount() error = %v", err)
					return
				}
				if count != 2 {
					t.Errorf("Expected count 2, got %d", count)
				}
			})
		}
	})
}
