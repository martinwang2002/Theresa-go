package akAbFs

import (
	"context"
	"time"

	"github.com/allegro/bigcache/v3"
	"github.com/go-redis/redis/v9"
	"github.com/rs/zerolog/log"

	"theresa-go/internal/config"
)

const akAbFsBigCacheDefaultTimeout = 2 * time.Minute
const akAbFsRedisDefaultTimeout = time.Hour

type CacheClient struct {
	bigCache     *bigcache.BigCache
	cacheContext context.Context
	redisClient  *redis.Client
}

func NewCacheClient(conf *config.Config) *CacheClient {
	bigCache, err := bigcache.NewBigCache(bigcache.Config{
		// number of shards (must be a power of 2)
		Shards: 32,

		// time after which entry can be evicted
		LifeWindow: akAbFsBigCacheDefaultTimeout,

		// Interval between removing expired entries (clean up).
		// If set to <= 0 then no action is performed.
		// Setting to < 1 second is counterproductive — bigcache has a one second resolution.
		CleanWindow: 1 * time.Minute,

		// rps * lifeWindow, used only in initial memory allocation
		MaxEntriesInWindow: 1000,

		// max entry size in bytes, used only in initial memory allocation
		// 20MB
		MaxEntrySize: 20 * 1024 * 1024,

		// prints information about additional memory allocation
		Verbose: false,

		// cache will not allocate more memory than this limit, value in MB
		// if value is reached then the oldest entries can be overridden for the new ones
		// 0 value means no size limit
		HardMaxCacheSize: 256,

		// callback fired when the oldest entry is removed because of its expiration time or no space left
		// for the new entry, or because delete was called. A bitmask representing the reason will be returned.
		// Default value is nil which means no callback and it prevents from unwrapping the oldest entry.
		OnRemove: nil,

		// OnRemoveWithReason is a callback fired when the oldest entry is removed because of its expiration time or no space left
		// for the new entry, or because delete was called. A constant representing the reason will be passed through.
		// Default value is nil which means no callback and it prevents from unwrapping the oldest entry.
		// Ignored if OnRemove is specified.
		OnRemoveWithReason: nil,
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
		bigCache:     bigCache,
		cacheContext: context.Background(),
		redisClient:  redisClient,
	}
}

func (cacheClient *CacheClient) GetBytes(key string) ([]byte, error) {
	value, err := cacheClient.bigCache.Get(key)
	if err != nil {
		value, err := cacheClient.redisClient.Get(cacheClient.cacheContext, key).Bytes()
		if err == nil {
			err := cacheClient.bigCache.Set(key, value)
			if err != nil {
				log.Error().Err(err).Int("length", len(value)).Str("key", key).Bytes("value", value).Msg("failed to set cache (bigcache)")
			}
		}
		return value, err
	}

	return value, nil
}

func (cacheClient *CacheClient) SetBytes(key string, value []byte) {
	err := cacheClient.bigCache.Set(key, value)
	if err != nil {
		log.Error().Err(err).Int("length", len(value)).Str("key", key).Bytes("value", value).Msg("failed to set cache (bigcache)")
	}

	defer func() {
		err = cacheClient.redisClient.Set(cacheClient.cacheContext, key, value, akAbFsRedisDefaultTimeout).Err()
		log.Error().Err(err).Int("length", len(value)).Str("key", key).Bytes("value", value).Msg("failed to set cache (redis)")
	}()
}

func (cacheClient *CacheClient) SetBytesWithTimeout(key string, value []byte, timeout time.Duration) {
	if timeout < akAbFsBigCacheDefaultTimeout {
		log.Warn().Msg("timeout is shorter than `akAbFsBigCacheDefaultTimeout`")
	}
	err := cacheClient.bigCache.Set(key, value)
	if err != nil {
		log.Error().Err(err).Int("length", len(value)).Str("key", key).Bytes("value", value).Msg("failed to set cache (bigcache)")
	}
	err = cacheClient.redisClient.Set(cacheClient.cacheContext, key, value, timeout).Err()
	if err != nil {
		log.Error().Err(err).Int("length", len(value)).Str("key", key).Bytes("value", value).Msg("failed to set cache (redis)")
	}
}

func (cacheClient *CacheClient) Flush() {
	err := cacheClient.bigCache.Reset()
	if err != nil {
		log.Error().Err(err).Msg("failed to purge cache (bigcache)")
	}
	err = cacheClient.redisClient.FlushDB(cacheClient.cacheContext).Err()
	log.Error().Err(err).Msg("failed to purge cache (redis)")
}
