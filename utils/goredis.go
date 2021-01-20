package utils

import (
	"fmt"
	"strings"

	"github.com/go-redis/redis"
)

var (
	RedisClient *redis.ClusterClient //redis的客户端
)

/**
redis client 初始化
*/
func RedisInit(currentCluster string) {
	CUR_SVC_ENV = currentCluster
	RedisClient = redis.NewClusterClient(&redis.ClusterOptions{
		Addrs:        env[CUR_SVC_ENV].ADDRS,
		Password:     env[CUR_SVC_ENV].PASSWORD,
		DialTimeout:  env[CUR_SVC_ENV].DIAL_TIMEOUT,
		ReadTimeout:  env[CUR_SVC_ENV].READ_TIMEOUT,
		WriteTimeout: env[CUR_SVC_ENV].WRITE_TIMEOUT,
		PoolSize:     env[CUR_SVC_ENV].POOL_SIZE,
		PoolTimeout:  env[CUR_SVC_ENV].POOL_TIMEOUT,
		IdleTimeout:  env[CUR_SVC_ENV].IDLE_TIMEOUT,
	})
	err := RedisClient.Ping().Err()
	if err != nil {
		fmt.Println("[redis] RedisPoolInit failed, error=", err)
	}
}

/**
根据pattern查询key
返回：key
*/
func KEYS(pattern string) ([]string, bool) {
	stringSliceCmd := RedisClient.Keys(AddKeyPreffix(pattern))
	if stringSliceCmd.Err() != nil {
		fmt.Println("[redis] keys failed, error=", stringSliceCmd.Err())
		return nil, false
	}
	return stringSliceCmd.Val(), true
}

/**
根据pattern查询key
client:master节点的client客户端
cursor：表示查询的起点
count：表示一次查询返回的记录数
返回：string[]：表示数据
uint64：表示下一次查询的游标，0表示查询结束
*/
func SCAN(client *redis.Client, cursor uint64, pattern string, count int64) ([]string, uint64, bool) {
	scanCmd := client.Scan(cursor, AddKeyPreffix(pattern), count)
	if scanCmd.Err() != nil {
		fmt.Println("[redis] scan failed, error=", scanCmd.Err())
		return nil, 0, false
	}
	keys, nextCursor := scanCmd.Val()
	for i, _ := range keys {
		keys[i] = DeleteKeyPreffix(keys[i])
	}
	return keys, nextCursor, true
}

/**
向名称为key的hash中添加元素field和value
返回：true添加成功；false添加失败
*/
func HSET(key string, field string, value interface{}) bool {
	boolCmd := RedisClient.HSet(AddKeyPreffix(key), field, value)
	if boolCmd.Err() != nil {
		fmt.Println("[redis] hset failed, error=", boolCmd.Err())
		return false
	}
	return true
}

/**
向名称为key的hash中批量添加元素field
返回：true添加成功；false添加失败
*/
func HMSET(key string, fields map[string]interface{}) bool {
	statusCmd := RedisClient.HMSet(AddKeyPreffix(key), fields)
	if statusCmd.Err() != nil {
		fmt.Println("[redis] hmset failed, error=", statusCmd.Err())
		return false
	}
	if statusCmd.Val() == "OK" {
		return true
	}
	return false
}

func HGET_INT64(key string, field string) (int64, bool) {
	result, ok := HGET(key, field)
	if !ok {
		return 0, false
	}
	return InterfaceToInt64(result)
}

/**
返回名称为key的hash中field对应的values
返回：value；true获取成功；false获取失败
*/
func HGET(key string, field string) (interface{}, bool) {
	stringCmd := RedisClient.HGet(AddKeyPreffix(key), field)
	if stringCmd.Err() != nil {
		fmt.Println("redis hget failed, error=", stringCmd.Err())
		return nil, false
	}
	return stringCmd.Val(), true
}
func HMGET_INT64(key string, fields []string) ([]int64, bool) {
	result, ok := HMGET(key, fields)
	if !ok {
		return nil, false
	}
	var rt []int64
	if len(result) > 0 {
		if rt = InterfacesToInt64s(result); rt == nil {
			return nil, false
		}
	}
	return rt, true
}

/**
获取名称为key的hash中fields对应的values
返回：所有的value；true获取成功；false获取失败
*/
func HMGET(key string, fields []string) ([]interface{}, bool) {
	sliceCmd := RedisClient.HMGet(AddKeyPreffix(key), fields...)
	if sliceCmd.Err() != nil {
		fmt.Println("redis hmget failed, error", sliceCmd.Err())
		return nil, false
	}
	return sliceCmd.Val(), true
}

/**
获取名称为key的hash中所有的field和value
返回：map的key是field，value是value,而且value必须是string类型
true获取成功；false获取失败
*/
func HGETALL(key string) (map[string]string, bool) {
	stringStringMapCmd := RedisClient.HGetAll(AddKeyPreffix(key))
	if stringStringMapCmd.Err() != nil {
		fmt.Println("[redis] hgetall failed, error=", stringStringMapCmd.Err())
		return nil, false
	}
	return stringStringMapCmd.Val(), true
}

func HSCAN(key string, cursor uint64, pattern string, count int64) ([]string, uint64, bool) {
	scanCmd := RedisClient.HScan(AddKeyPreffix(key), cursor, pattern, count)
	if scanCmd.Err() != nil {
		fmt.Println("[redis] scan failed, error=", scanCmd.Err())
		return nil, 0, false
	}
	field, nextCursor := scanCmd.Val()
	return field, nextCursor, true
}

/**
删除名称为key的hash中键为field的域
返回：true删除成功；false删除失败或未能删除成功所有的field
*/
func HDEL(key string, field ...string) bool {
	intCmd := RedisClient.HDel(AddKeyPreffix(key), field...)
	if intCmd.Err() != nil {
		fmt.Println("[redis] hdel failed, error=", intCmd.Err())
		return false
	}
	if intCmd.Val() == int64(len(field)) {
		return true
	}
	return false
}

/**
判断名称为key的hash中是否存在键为field的域
true存在；false不存在
*/
func HEXISTS(key string, field string) bool {
	boolCmd := RedisClient.HExists(AddKeyPreffix(key), field)
	if boolCmd.Err() != nil {
		fmt.Println("[redis] hexists failed, error=", boolCmd.Err())
		return false
	}
	return boolCmd.Val()
}

/**
删除key前缀
*/
func DeleteKeyPreffix(key string) string {
	return strings.TrimPrefix(key, REDIS_KEY_PREFFIX)
}

/**
给key加上前缀，方便redis全局搜素
*/
func AddKeyPreffix(key string) string {
	return REDIS_KEY_PREFFIX + key
}
