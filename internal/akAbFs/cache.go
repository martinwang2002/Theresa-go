package akAbFs

import (
	"context"
	"time"

	"github.com/go-redis/redis/v9"
	goCacheLib "github.com/patrickmn/go-cache"
	"github.com/rs/zerolog/log"
	"github.com/tidwall/gjson"

	"theresa-go/internal/config"
)

const akAbFsGoCacheDefaultTimeout = 30 * time.Second
const akAbFsRedisDefaultTimeout = time.Hour

type CacheClient struct {
	goCache      *goCacheLib.Cache
	cacheContext context.Context
	redisClient  *redis.Client
}

func NewCacheClient(conf *config.Config) *CacheClient {
	goCache := goCacheLib.New(akAbFsGoCacheDefaultTimeout, 2*akAbFsGoCacheDefaultTimeout)

	redisOptions, err := redis.ParseURL(conf.RedisDsn)

	if err != nil {
		panic(err)
	}

	redisClient := redis.NewClient(redisOptions)

	return &CacheClient{
		goCache:      goCache,
		cacheContext: context.Background(),
		redisClient:  redisClient,
	}
}

func (cacheClient *CacheClient) GetBytes(key string) ([]byte, error) {
	value, found := cacheClient.goCache.Get(key)
	if found {
		return value.([]byte), nil
	} else {
		value, err := cacheClient.redisClient.Get(cacheClient.cacheContext, key).Bytes()
		if err == nil {
			cacheClient.goCache.Set(key, value, goCacheLib.DefaultExpiration)
		}
		return value, err
	}
}

func (cacheClient *CacheClient) SetBytes(key string, value []byte) {
	cacheClient.goCache.Set(key, value, goCacheLib.DefaultExpiration)

	defer func() {
		err := cacheClient.redisClient.Set(cacheClient.cacheContext, key, value, akAbFsRedisDefaultTimeout).Err()
		if err != nil {
			log.Error().Err(err).Int("length", len(value)).Str("key", key).Bytes("value", value).Msg("failed to set cache (redis)")
		}
	}()
}

func (cacheClient *CacheClient) SetBytesWithTimeout(key string, value []byte, timeout time.Duration) {
	if timeout < akAbFsGoCacheDefaultTimeout {
		log.Warn().Msg("timeout is shorter than `akAbFsGoCacheDefaultTimeout`")
	}
	cacheClient.goCache.Set(key, value, goCacheLib.DefaultExpiration)

	err := cacheClient.redisClient.Set(cacheClient.cacheContext, key, value, timeout).Err()
	if err != nil {
		log.Error().Err(err).Int("length", len(value)).Str("key", key).Bytes("value", value).Msg("failed to set cache (redis)")
	}
}

func (cacheClient *CacheClient) GetGjsonResult(key string) (*gjson.Result, error) {
	value, found := cacheClient.goCache.Get(key)
	if found {
		return value.(*gjson.Result), nil
	} else {
		bytesValue, err := cacheClient.redisClient.Get(cacheClient.cacheContext, key).Bytes()
		value := gjson.ParseBytes(bytesValue)
		if err == nil {
			cacheClient.goCache.Set(key, &value, goCacheLib.DefaultExpiration)
		}
		return &value, err
	}
}

func (cacheClient *CacheClient) SetGjsonResult(key string, gjsonBytes []byte, gjsonValue *gjson.Result) {
	cacheClient.goCache.Set(key, gjsonValue, goCacheLib.DefaultExpiration)

	defer func() {
		err := cacheClient.redisClient.Set(cacheClient.cacheContext, key, gjsonBytes, akAbFsRedisDefaultTimeout).Err()
		if err != nil {
			log.Error().Err(err).Int("length", len(gjsonBytes)).Str("key", key).Bytes("value", gjsonBytes).Msg("failed to set cache (redis)")
		}
	}()
}

func (cacheClient *CacheClient) Flush() {
	cacheClient.goCache.Flush()

	err := cacheClient.redisClient.FlushDB(cacheClient.cacheContext).Err()
	if err != nil {
		log.Error().Err(err).Msg("failed to flush cache (redis)")
	}
}
