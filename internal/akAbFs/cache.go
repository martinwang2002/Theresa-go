package akAbFs

import (
	"context"
	"time"

	"github.com/dgraph-io/ristretto"
	"github.com/redis/go-redis/v9"
	"github.com/rs/zerolog/log"
	"github.com/tidwall/gjson"

	"theresa-go/internal/config"
)

const akAbFsGoCacheDefaultTimeout = 30 * time.Second
const akAbFsRedisDefaultTimeout = time.Hour

type CacheClient struct {
	ristrettoCache *ristretto.Cache
	redisClient    *redis.Client
}

func NewCacheClient(conf *config.Config) *CacheClient {
	ristrettoCache, err := ristretto.NewCache(&ristretto.Config{
		NumCounters: 1 * 1 << 20,  // number of keys to track frequency of (10M).
		MaxCost:     50 * 1 << 20, // maximum cost of cache (50MB).
		BufferItems: 64,           // number of keys per Get buffer.
	})

	if err != nil {
		panic(err)
	}

	redisOptions, err := redis.ParseURL(conf.RedisDsn)

	if err != nil {
		panic(err)
	}

	redisClient := redis.NewClient(redisOptions)

	return &CacheClient{
		ristrettoCache: ristrettoCache,
		redisClient:    redisClient,
	}
}

func (cacheClient *CacheClient) GetBytes(ctx context.Context, key string) ([]byte, error) {
	value, found := cacheClient.ristrettoCache.Get(key)
	if found {
		return value.([]byte), nil
	} else {
		value, err := cacheClient.redisClient.Get(ctx, key).Bytes()
		if err == nil {
			cacheClient.ristrettoCache.SetWithTTL(key, value, 0, akAbFsGoCacheDefaultTimeout)
		}
		return value, err
	}
}

func (cacheClient *CacheClient) SetBytes(ctx context.Context, key string, value []byte) {
	cacheClient.ristrettoCache.SetWithTTL(key, value, 0, akAbFsGoCacheDefaultTimeout)

	defer func() {
		err := cacheClient.redisClient.Set(ctx, key, value, akAbFsRedisDefaultTimeout).Err()
		if err != nil {
			log.Error().Err(err).Int("length", len(value)).Str("key", key).Bytes("value", value).Msg("failed to set cache (redis)")
		}
	}()
}

func (cacheClient *CacheClient) SetBytesWithTimeout(ctx context.Context, key string, value []byte, timeout time.Duration) {
	if timeout < akAbFsGoCacheDefaultTimeout {
		log.Warn().Msg("timeout is shorter than `akAbFsGoCacheDefaultTimeout`")
	}
	cacheClient.ristrettoCache.SetWithTTL(key, value, 0, akAbFsGoCacheDefaultTimeout)

	err := cacheClient.redisClient.Set(ctx, key, value, timeout).Err()
	if err != nil {
		log.Error().Err(err).Int("length", len(value)).Str("key", key).Bytes("value", value).Msg("failed to set cache (redis)")
	}
}

func (cacheClient *CacheClient) GetGjsonResult(ctx context.Context, key string) (*gjson.Result, error) {
	value, found := cacheClient.ristrettoCache.Get(key)
	if found {
		return value.(*gjson.Result), nil
	} else {
		redisCmd := cacheClient.redisClient.Get(ctx, key)
		if redisCmd.Err() != nil {
			return nil, redisCmd.Err()
		}
		resultBytes, err := redisCmd.Bytes()
		if err != nil {
			return nil, err
		}
		value := gjson.ParseBytes(resultBytes)
		cacheClient.ristrettoCache.SetWithTTL(key, &value, 0, akAbFsGoCacheDefaultTimeout)
		return &value, nil
	}
}

func (cacheClient *CacheClient) SetGjsonResult(ctx context.Context, key string, gjsonBytes []byte, gjsonValue *gjson.Result) {
	cacheClient.ristrettoCache.SetWithTTL(key, gjsonValue, 0, akAbFsGoCacheDefaultTimeout)

	defer func() {
		err := cacheClient.redisClient.Set(ctx, key, gjsonBytes, akAbFsRedisDefaultTimeout).Err()
		if err != nil {
			log.Error().Err(err).Int("length", len(gjsonBytes)).Str("key", key).Bytes("value", gjsonBytes).Msg("failed to set cache (redis)")
		}
	}()
}

func (cacheClient *CacheClient) Flush(ctx context.Context) {
	cacheClient.ristrettoCache.Clear()

	err := cacheClient.redisClient.FlushDB(ctx).Err()
	if err != nil {
		log.Error().Err(err).Msg("failed to flush cache (redis)")
	}
}
