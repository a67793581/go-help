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

func TestRateLimiterV2_NewRateLimiterV2(t *testing.T) {
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
		config      RateLimitConfigV2
		expectError bool
	}{
		{
			name:   "valid config",
			client: client,
			config: RateLimitConfigV2{
				Key:      "test_key",
				MaxCount: 100,
				TimeUnit: time.Hour,
				Timezone: time.UTC,
			},
			expectError: false,
		},
		{
			name:   "nil client",
			client: nil,
			config: RateLimitConfigV2{
				Key:      "test_key",
				MaxCount: 100,
				TimeUnit: time.Hour,
			},
			expectError: true,
		},
		{
			name:   "empty key",
			client: client,
			config: RateLimitConfigV2{
				Key:      "",
				MaxCount: 100,
				TimeUnit: time.Hour,
			},
			expectError: true,
		},
		{
			name:   "zero max count",
			client: client,
			config: RateLimitConfigV2{
				Key:      "test_key",
				MaxCount: 0,
				TimeUnit: time.Hour,
			},
			expectError: true,
		},
		{
			name:   "negative time unit",
			client: client,
			config: RateLimitConfigV2{
				Key:      "test_key",
				MaxCount: 100,
				TimeUnit: -time.Hour,
			},
			expectError: true,
		},
		{
			name:   "time unit less than second",
			client: client,
			config: RateLimitConfigV2{
				Key:      "test_key",
				MaxCount: 100,
				TimeUnit: time.Millisecond,
			},
			expectError: true,
		},
		{
			name:   "time unit exceeds 24 hours",
			client: client,
			config: RateLimitConfigV2{
				Key:      "test_key",
				MaxCount: 100,
				TimeUnit: 25 * 24 * time.Hour,
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := NewRateLimiterV2(tt.client, tt.config)
			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestRateLimiterV2_IsAllowed(t *testing.T) {
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

	config := RateLimitConfigV2{
		Key:      "test_limit",
		MaxCount: 3,
		TimeUnit: time.Hour,
		Timezone: time.UTC,
	}

	rl, err := NewRateLimiterV2(client, config)
	assert.NoError(t, err)

	ctx := context.Background()

	// 循环测试允许的请求
	for i := 0; i < 3; i++ {
		allowed, remaining, err := rl.IsAllowed(ctx)
		assert.NoError(t, err)
		assert.True(t, allowed)
		assert.Equal(t, int64(2-i), remaining)
	}

	// 测试超出限制的请求
	allowed, remaining, err := rl.IsAllowed(ctx)
	assert.NoError(t, err)
	assert.False(t, allowed)
	assert.Equal(t, int64(0), remaining)
}

func TestRateLimiterV2_GetCurrentCount(t *testing.T) {
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

	config := RateLimitConfigV2{
		Key:      "test_current",
		MaxCount: 10,
		TimeUnit: time.Hour,
		Timezone: time.UTC,
	}

	rl, err := NewRateLimiterV2(client, config)
	assert.NoError(t, err)

	ctx := context.Background()

	// 初始计数应该为0
	count, err := rl.GetCurrentCount(ctx)
	assert.NoError(t, err)
	assert.Equal(t, int64(0), count)

	// 增加一次计数
	allowed, _, err := rl.IsAllowed(ctx)
	assert.NoError(t, err)
	assert.True(t, allowed)

	// 检查当前计数
	count, err = rl.GetCurrentCount(ctx)
	assert.NoError(t, err)
	assert.Equal(t, int64(1), count)
}

func TestRateLimiterV2_GetRemainingCount(t *testing.T) {
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

	config := RateLimitConfigV2{
		Key:      "test_remaining",
		MaxCount: 5,
		TimeUnit: time.Hour,
		Timezone: time.UTC,
	}

	rl, err := NewRateLimiterV2(client, config)
	assert.NoError(t, err)

	ctx := context.Background()

	// 初始剩余次数应该为最大值
	remaining, err := rl.GetRemainingCount(ctx)
	assert.NoError(t, err)
	assert.Equal(t, int64(5), remaining)

	// 使用一次
	_, _, err = rl.IsAllowed(ctx)
	assert.NoError(t, err)

	// 剩余次数应该减少1
	remaining, err = rl.GetRemainingCount(ctx)
	assert.NoError(t, err)
	assert.Equal(t, int64(4), remaining)
}

func TestRateLimiterV2_IncreaseCount(t *testing.T) {
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

	config := RateLimitConfigV2{
		Key:      "test_increase",
		MaxCount: 10,
		TimeUnit: time.Hour,
		Timezone: time.UTC,
	}

	rl, err := NewRateLimiterV2(client, config)
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

	// 测试无效增量
	err = rl.IncreaseCount(ctx, 0)
	assert.Error(t, err)

	err = rl.IncreaseCount(ctx, -1)
	assert.NoError(t, err)
	// 检查当前计数
	count, err = rl.GetCurrentCount(ctx)
	assert.NoError(t, err)
	assert.Equal(t, int64(4), count)
}

func TestRateLimiterV2_SetCount(t *testing.T) {
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

	config := RateLimitConfigV2{
		Key:      "test_set",
		MaxCount: 10,
		TimeUnit: time.Hour,
		Timezone: time.UTC,
	}

	rl, err := NewRateLimiterV2(client, config)
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

func TestRateLimiterV2_ResetRateLimit(t *testing.T) {
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

	config := RateLimitConfigV2{
		Key:      "test_reset",
		MaxCount: 3,
		TimeUnit: time.Hour,
		Timezone: time.UTC,
	}

	rl, err := NewRateLimiterV2(client, config)
	assert.NoError(t, err)

	ctx := context.Background()

	// 使用几次限流器
	for i := 0; i < 3; i++ {
		allowed, _, err := rl.IsAllowed(ctx)
		assert.NoError(t, err)
		assert.True(t, allowed)
	}

	// 检查当前计数
	count, err := rl.GetCurrentCount(ctx)
	assert.NoError(t, err)
	assert.Equal(t, int64(3), count)

	// 重置
	err = rl.ResetRateLimit(ctx)
	assert.NoError(t, err)

	// 检查计数是否重置
	count, err = rl.GetCurrentCount(ctx)
	assert.NoError(t, err)
	assert.Equal(t, int64(0), count)

	// 应该再次允许请求
	allowed, remaining, err := rl.IsAllowed(ctx)
	assert.NoError(t, err)
	assert.True(t, allowed)
	assert.Equal(t, int64(2), remaining)
}

func TestRateLimiterV2_GetConfig(t *testing.T) {
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

	tz := time.FixedZone("TestZone", 8*3600) // UTC+8

	config := RateLimitConfigV2{
		Key:      "test_config",
		MaxCount: 100,
		TimeUnit: time.Hour,
		Timezone: tz,
	}

	rl, err := NewRateLimiterV2(client, config)
	assert.NoError(t, err)

	key, maxCount, timeUnit, timezone := rl.GetConfig()
	assert.Equal(t, "test_config", key)
	assert.Equal(t, int64(100), maxCount)
	assert.Equal(t, time.Hour, timeUnit)
	assert.Equal(t, tz, timezone)
}

func TestRateLimiterV2_TimezoneHandling(t *testing.T) {
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

	// 使用UTC时区
	utcConfig := RateLimitConfigV2{
		Key:      "test_utc",
		MaxCount: 10,
		TimeUnit: time.Hour,
		Timezone: time.UTC,
	}

	utcLimiter, err := NewRateLimiterV2(client, utcConfig)
	assert.NoError(t, err)

	// 使用东八区时区
	cst, _ := time.LoadLocation("Asia/Shanghai")
	cstConfig := RateLimitConfigV2{
		Key:      "test_cst",
		MaxCount: 10,
		TimeUnit: time.Hour,
		Timezone: cst,
	}

	cstLimiter, err := NewRateLimiterV2(client, cstConfig)
	assert.NoError(t, err)

	// 验证两个限流器使用不同的key（因为时区不同）
	utcKey := utcLimiter.GenerateTimeKey()
	cstKey := cstLimiter.GenerateTimeKey()

	// 如果当前时间在不同时区属于不同小时，则key应该不同
	assert.NotEqual(t, utcKey, cstKey)
}

func TestRateLimiterV2_DifferentTimeUnits(t *testing.T) {
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

	// 测试按秒限流
	secondConfig := RateLimitConfigV2{
		Key:      "test_second",
		MaxCount: 2,
		TimeUnit: time.Second,
		Timezone: time.UTC,
	}

	secondLimiter, err := NewRateLimiterV2(client, secondConfig)
	assert.NoError(t, err)

	// 使用所有配额
	for i := 0; i < 2; i++ {
		allowed, _, err := secondLimiter.IsAllowed(ctx)
		assert.NoError(t, err)
		assert.True(t, allowed)
	}

	// 应该被拒绝
	allowed, _, err := secondLimiter.IsAllowed(ctx)
	assert.NoError(t, err)
	assert.False(t, allowed)

	// 通过修改mock的时间来模拟时间流逝
	newTime := fixedTime.Add(time.Second)
	patches.ApplyFunc(time.Now, func() time.Time {
		return newTime
	})

	// 应该再次允许
	allowed, _, err = secondLimiter.IsAllowed(ctx)
	assert.NoError(t, err)
	assert.True(t, allowed)
}

func TestRateLimiterV2_CrossDayHandling(t *testing.T) {
	s, err := miniredis.Run()
	assert.NoError(t, err)
	defer s.Close()

	client := redis.NewClient(&redis.Options{
		Addr: s.Addr(),
	})

	// 使用gomonkey mock time.Now函数
	patches := gomonkey.NewPatches()
	defer patches.Reset()

	// 加载中国时区
	cst, _ := time.LoadLocation("Asia/Shanghai")

	config := RateLimitConfigV2{
		Key:      "test_cross_day",
		MaxCount: 10,
		TimeUnit: time.Hour * 24, // 按天限流
		Timezone: cst,
	}

	rl, err := NewRateLimiterV2(client, config)
	assert.NoError(t, err)

	// 模拟中国时间23点（第一天）
	time23 := time.Date(2023, 1, 1, 23, 30, 0, 0, cst)
	patches.ApplyFunc(time.Now, func() time.Time {
		return time23
	})

	// 在23点发起请求
	key23 := rl.GenerateTimeKey()

	// 模拟中国时间第二天1点
	time1 := time.Date(2023, 1, 2, 1, 30, 0, 0, cst)
	patches.ApplyFunc(time.Now, func() time.Time {
		return time1
	})

	// 在1点发起请求
	key1 := rl.GenerateTimeKey()

	// 验证两个时间点使用的是不同的key
	assert.NotEqual(t, key23, key1, "不同日期应该生成不同的key")

	// 验证23点的key包含第一天的日期
	assert.Contains(t, key23, "test_cross_day:20230101")

	// 验证1点的key包含第二天的日期
	assert.Contains(t, key1, "test_cross_day:20230102")
}
