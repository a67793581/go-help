package redis_help

import (
	"context"
	"testing"
	"time"

	"github.com/agiledragon/gomonkey/v2"
	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
)

func TestRateLimiter_NewRateLimiter(t *testing.T) {
	// 确保没有gomonkey的patches影响此测试
	patches := gomonkey.NewPatches()
	patches.Reset()

	// 启动一个模拟的Redis服务器
	s, err := miniredis.Run()
	assert.NoError(t, err)
	defer s.Close()

	// 创建Redis客户端
	client := redis.NewClient(&redis.Options{
		Addr: s.Addr(),
	})

	tests := []struct {
		name        string
		client      redis.UniversalClient
		config      RateLimitConfig
		expectError bool
	}{
		{
			name:   "valid config",
			client: client,
			config: RateLimitConfig{
				Key:      "test_key",
				MaxCount: 100,
				TimeUnit: time.Hour,
			},
			expectError: false,
		},
		{
			name:   "nil client",
			client: nil,
			config: RateLimitConfig{
				Key:      "test_key",
				MaxCount: 100,
				TimeUnit: time.Hour,
			},
			expectError: true,
		},
		{
			name:   "empty key",
			client: client,
			config: RateLimitConfig{
				Key:      "",
				MaxCount: 100,
				TimeUnit: time.Hour,
			},
			expectError: true,
		},
		{
			name:   "zero max count",
			client: client,
			config: RateLimitConfig{
				Key:      "test_key",
				MaxCount: 0,
				TimeUnit: time.Hour,
			},
			expectError: true,
		},
		{
			name:   "negative max count",
			client: client,
			config: RateLimitConfig{
				Key:      "test_key",
				MaxCount: -1,
				TimeUnit: time.Hour,
			},
			expectError: true,
		},
		{
			name:   "zero time window",
			client: client,
			config: RateLimitConfig{
				Key:      "test_key",
				MaxCount: 100,
				TimeUnit: 0,
			},
			expectError: true,
		},
		{
			name:   "negative time window",
			client: client,
			config: RateLimitConfig{
				Key:      "test_key",
				MaxCount: 100,
				TimeUnit: -time.Hour,
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := NewRateLimiter(tt.client, tt.config)
			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestRateLimiter_IsAllowed(t *testing.T) {
	s, err := miniredis.Run()
	assert.NoError(t, err)
	defer s.Close()

	client := redis.NewClient(&redis.Options{
		Addr: s.Addr(),
	})

	// 使用gomonkey mock time.Now函数
	patches := gomonkey.NewPatches()
	defer patches.Reset()

	// 固定一个时间点
	fixedTime := time.Date(2023, 1, 1, 12, 0, 0, 0, time.UTC)
	patches.ApplyFunc(time.Now, func() time.Time {
		return fixedTime
	})

	// 修改为使用RateLimitConfig结构体
	rl, err := NewRateLimiter(client, RateLimitConfig{
		Key:      "test_limit",
		MaxCount: 3,
		TimeUnit: time.Hour,
	})
	assert.NoError(t, err)

	ctx := context.Background()

	// 第一次请求，应该允许
	allowed, remaining, err := rl.IsAllowed(ctx)
	assert.NoError(t, err)
	assert.True(t, allowed)
	assert.Equal(t, int64(2), remaining)

	// 第二次请求，应该允许
	allowed, remaining, err = rl.IsAllowed(ctx)
	assert.NoError(t, err)
	assert.True(t, allowed)
	assert.Equal(t, int64(1), remaining)

	// 第三次请求，应该允许
	allowed, remaining, err = rl.IsAllowed(ctx)
	assert.NoError(t, err)
	assert.True(t, allowed)
	assert.Equal(t, int64(0), remaining)

	// 第四次请求，应该拒绝
	allowed, remaining, err = rl.IsAllowed(ctx)
	assert.NoError(t, err)
	assert.False(t, allowed)
	assert.Equal(t, int64(0), remaining)
}

func TestRateLimiter_GetCurrentCount(t *testing.T) {
	s, err := miniredis.Run()
	assert.NoError(t, err)
	defer s.Close()

	client := redis.NewClient(&redis.Options{
		Addr: s.Addr(),
	})

	// 使用gomonkey mock time.Now函数
	patches := gomonkey.NewPatches()
	defer patches.Reset()

	// 固定一个时间点
	fixedTime := time.Date(2023, 1, 1, 12, 0, 0, 0, time.UTC)
	patches.ApplyFunc(time.Now, func() time.Time {
		return fixedTime
	})
	config := RateLimitConfig{
		Key:      "test_current",
		MaxCount: 10,
		TimeUnit: time.Hour,
	}
	// 修改为使用RateLimitConfig结构体
	rl, err := NewRateLimiter(client, config)
	assert.NoError(t, err)

	ctx := context.Background()

	// 确保测试开始前清理键
	err = rl.ResetRateLimit(ctx)
	assert.NoError(t, err)

	// 初始计数应该为MaxCount（因为GetCurrentCount返回的是剩余次数）
	count, err := rl.GetCurrentCount(ctx)
	assert.NoError(t, err)
	assert.Equal(t, int64(10), count)

	// 增加一次计数（使用一次限流）
	allowed, _, err := rl.IsAllowed(ctx)
	assert.NoError(t, err)
	assert.True(t, allowed)

	// 检查当前计数（剩余次数）
	count, err = rl.GetCurrentCount(ctx)
	assert.NoError(t, err)
	assert.Equal(t, int64(9), count)

	// 计算已使用次数
	used := config.MaxCount - count
	assert.Equal(t, int64(1), used)
}

func TestRateLimiter_IncreaseCount(t *testing.T) {
	s, err := miniredis.Run()
	assert.NoError(t, err)
	defer s.Close()

	client := redis.NewClient(&redis.Options{
		Addr: s.Addr(),
	})

	// 使用gomonkey mock time.Now函数
	patches := gomonkey.NewPatches()
	defer patches.Reset()

	// 固定一个时间点
	fixedTime := time.Date(2023, 1, 1, 12, 0, 0, 0, time.UTC)
	patches.ApplyFunc(time.Now, func() time.Time {
		return fixedTime
	})

	// 修改为使用RateLimitConfig结构体
	rl, err := NewRateLimiter(client, RateLimitConfig{
		Key:      "test_increase",
		MaxCount: 10,
		TimeUnit: time.Hour,
	})
	assert.NoError(t, err)

	// 确保测试开始前清理键
	err = rl.ResetRateLimit(context.Background())
	assert.NoError(t, err)

	ctx := context.Background()

	// 增加计数
	err = rl.IncreaseCount(ctx, 3)
	assert.NoError(t, err)

	// 检查当前计数
	count, err := rl.GetCurrentCount(ctx)
	assert.NoError(t, err)
	assert.Equal(t, int64(3), count)

	// 再次增加
	err = rl.IncreaseCount(ctx, 2)
	assert.NoError(t, err)

	// 检查当前计数
	count, err = rl.GetCurrentCount(ctx)
	assert.NoError(t, err)
	assert.Equal(t, int64(5), count)

	// 测试无效增量 - 0值现在会返回错误
	err = rl.IncreaseCount(ctx, 0)
	assert.Error(t, err) // 0值应该返回错误

	// 确保计数没有变化
	count, err = rl.GetCurrentCount(ctx)
	assert.NoError(t, err)
	assert.Equal(t, int64(5), count)

	err = rl.IncreaseCount(ctx, -1)
	assert.NoError(t, err)
}

func TestRateLimiter_SetCount(t *testing.T) {
	s, err := miniredis.Run()
	assert.NoError(t, err)
	defer s.Close()

	client := redis.NewClient(&redis.Options{
		Addr: s.Addr(),
	})

	// 使用gomonkey mock time.Now函数
	patches := gomonkey.NewPatches()
	defer patches.Reset()

	// 固定一个时间点
	fixedTime := time.Date(2023, 1, 1, 12, 0, 0, 0, time.UTC)
	patches.ApplyFunc(time.Now, func() time.Time {
		return fixedTime
	})

	// 确保 TimeUnit 不为0，避免除零错误
	rl, err := NewRateLimiter(client, RateLimitConfig{
		Key:      "test_set",
		MaxCount: 10,
		TimeUnit: time.Hour, // 不能为0
	})
	assert.NoError(t, err)
	err = rl.ResetRateLimit(context.Background())
	assert.NoError(t, err)

	ctx := context.Background()

	// 设置计数
	err = rl.SetCount(ctx, 7)
	assert.NoError(t, err)

	// 检查当前计数
	count, err := rl.GetCurrentCount(ctx)
	assert.NoError(t, err)
	assert.Equal(t, int64(7), count)

	// 设置为0
	err = rl.SetCount(ctx, 0)
	assert.NoError(t, err)

	// 检查当前计数
	count, err = rl.GetCurrentCount(ctx)
	assert.NoError(t, err)
	assert.Equal(t, int64(0), count)

	// 测试负数
	err = rl.SetCount(ctx, -1)
	assert.Error(t, err)
}

func TestRateLimiter_ResetRateLimit(t *testing.T) {
	s, err := miniredis.Run()
	assert.NoError(t, err)
	defer s.Close()

	client := redis.NewClient(&redis.Options{
		Addr: s.Addr(),
	})

	// 使用gomonkey mock time.Now函数
	patches := gomonkey.NewPatches()
	defer patches.Reset()

	// 固定一个时间点
	fixedTime := time.Date(2023, 1, 1, 12, 0, 0, 0, time.UTC)
	patches.ApplyFunc(time.Now, func() time.Time {
		return fixedTime
	})

	// 修改为使用RateLimitConfig结构体
	rl, err := NewRateLimiter(client, RateLimitConfig{
		Key:      "test_reset",
		MaxCount: 3,
		TimeUnit: time.Hour,
	})
	assert.NoError(t, err)

	ctx := context.Background()

	// 确保测试开始前清理键
	err = rl.ResetRateLimit(ctx)
	assert.NoError(t, err)

	// 使用几次限流器
	for i := 0; i < 3; i++ {
		allowed, _, err := rl.IsAllowed(ctx)
		assert.NoError(t, err)
		assert.True(t, allowed)
	}

	// 检查当前计数
	count, err := rl.GetCurrentCount(ctx)
	assert.NoError(t, err)
	assert.Equal(t, int64(0), count)

	// 重置
	err = rl.ResetRateLimit(ctx)
	assert.NoError(t, err)

	// 检查计数是否重置
	count, err = rl.GetCurrentCount(ctx)
	assert.NoError(t, err)
	assert.Equal(t, int64(3), count)

	// 应该再次允许请求
	allowed, remaining, err := rl.IsAllowed(ctx)
	assert.NoError(t, err)
	assert.True(t, allowed)
	assert.Equal(t, int64(2), remaining)
}

func TestRateLimiter_GetConfig(t *testing.T) {
	s, err := miniredis.Run()
	assert.NoError(t, err)
	defer s.Close()

	client := redis.NewClient(&redis.Options{
		Addr: s.Addr(),
	})

	// 修改为使用RateLimitConfig结构体
	rl, err := NewRateLimiter(client, RateLimitConfig{
		Key:      "test_config",
		MaxCount: 100,
		TimeUnit: time.Hour,
	})
	assert.NoError(t, err)

	key, maxCount, timeWindow := rl.GetConfig()
	assert.Equal(t, "test_config", key)
	assert.Equal(t, int64(100), maxCount)
	assert.Equal(t, time.Hour, timeWindow)
}

func TestRateLimiter_WindowReset(t *testing.T) {
	s, err := miniredis.Run()
	assert.NoError(t, err)
	defer s.Close()

	client := redis.NewClient(&redis.Options{
		Addr: s.Addr(),
	})

	ctx := context.Background()

	// 使用gomonkey mock time.Now函数
	patches := gomonkey.NewPatches()
	defer patches.Reset()

	// 固定一个时间点
	fixedTime := time.Date(2023, 1, 1, 12, 0, 0, 0, time.UTC)
	patches.ApplyFunc(time.Now, func() time.Time {
		return fixedTime
	})

	// 修改为使用RateLimitConfig结构体
	rl, err := NewRateLimiter(client, RateLimitConfig{
		Key:      "test_window",
		MaxCount: 2,
		TimeUnit: time.Second,
	})
	assert.NoError(t, err)

	// 确保测试开始前清理键
	err = rl.ResetRateLimit(ctx)
	assert.NoError(t, err)

	// 使用所有配额
	for i := 0; i < 2; i++ {
		allowed, _, err := rl.IsAllowed(ctx)
		assert.NoError(t, err)
		assert.True(t, allowed)
	}

	// 应该被拒绝
	allowed, _, err := rl.IsAllowed(ctx)
	assert.NoError(t, err)
	assert.False(t, allowed)

	// 通过修改mock的时间来模拟时间窗口重置
	newTime := fixedTime.Add(time.Second)
	patches.ApplyFunc(time.Now, func() time.Time {
		return newTime
	})

	// 应该再次允许
	allowed, _, err = rl.IsAllowed(ctx)
	assert.NoError(t, err)
	assert.True(t, allowed)
}
