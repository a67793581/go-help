package redis_help

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	redis "github.com/redis/go-redis/v9"
)

type DataRedis struct {
	Alias        string   `json:"alias,omitempty"`
	Address      string   `json:"address,omitempty"`
	IsCluster    bool     `json:"is_cluster,omitempty"`
	ReadTimeout  Duration `json:"read_timeout,omitempty"`
	WriteTimeout Duration `json:"write_timeout,omitempty"`
}
type Duration time.Duration

// NewRedis Initialize redis connection.
func NewRedis(config *DataRedis) (redis.UniversalClient, error) {
	if len(config.Address) == 0 {
		return nil, errors.New("redis address is empty")
	}
	var rdb redis.UniversalClient
	maxRetry, minIdleConns, maxIdleConns, poolSize := 3, 30, 50, 100
	Address := strings.Split(config.Address, ",")
	if len(Address) == 0 {
		return nil, errors.New("redis address is empty")
	}

	if config.IsCluster {
		rdb = redis.NewClusterClient(&redis.ClusterOptions{
			Addrs:        Address,
			PoolSize:     poolSize,
			MaxIdleConns: maxIdleConns,
			MinIdleConns: minIdleConns,
			MaxRetries:   maxRetry,
			ReadTimeout:  time.Second * time.Duration(config.ReadTimeout),
			WriteTimeout: time.Second * time.Duration(config.ReadTimeout),
		})
	} else {
		rdb = redis.NewClient(&redis.Options{
			Addr:         Address[0],
			DB:           0,        // use default DB
			PoolSize:     poolSize, // connection pool size
			MaxIdleConns: maxIdleConns,
			MinIdleConns: minIdleConns,
			MaxRetries:   maxRetry,
			ReadTimeout:  time.Second * time.Duration(config.ReadTimeout),
			WriteTimeout: time.Second * time.Duration(config.ReadTimeout),
		})
	}

	var err error = nil
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*10)
	defer cancel()
	err = rdb.Ping(ctx).Err()
	if err != nil {
		return nil, fmt.Errorf("redis ping %s", err)
	}
	return rdb, err
}

// RegisterCache ...
func RegisterCache(configs []DataRedis) (map[string]redis.UniversalClient, error) {

	Handlers := map[string]redis.UniversalClient{}
	for _, v := range configs {
		if len(v.Address) == 0 || v.Alias == "" {
			return nil, fmt.Errorf("the Address or alias of %s not exist", v.Alias)
		}

		h, err := NewRedis(&v)
		if err != nil {
			return nil, fmt.Errorf("connect to Redis %s failed: %v", v.Alias, err)
		}

		Handlers[v.Alias] = h
	}
	return Handlers, nil
}
