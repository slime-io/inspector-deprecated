package utils

import (
	"fmt"
	"time"

	"github.com/go-redis/redis"
	"github.com/robfig/cron/v3"
)

type keyAndField struct {
	key   string
	field string
	value string
}

func StartCron() {
	c := cron.New()
	_, _ = c.AddFunc(env[CUR_SVC_ENV].CRON_SCHEDULE, func() {
		fmt.Println("StartCron")
		cronJob()
	})
	c.Start()
	defer c.Stop()
	//czl
	time.Sleep(time.Duration(10 * time.Minute))
}

func cronJob() {
	fmt.Println("cronJob")
	keyChannel := make(chan []string, 10)
	fieldsChannel := make(chan keyAndField, 10)
	//开启一个协程查询所有的key,然后放入channel里
	go func() {
		_ = RedisClient.ForEachMaster(func(client *redis.Client) error {
			fmt.Println("client=", client)
			cursor := uint64(0)
			for {
				keys, nextCursor, ok := SCAN(client, cursor, "*", env[CUR_SVC_ENV].SCAN_COUNT)
				if ok {
					cursor = nextCursor
					if len(keys) != 0 {
						fmt.Println("scan:key={} nextCursor={} client={}", keys, nextCursor, client)
						keyChannel <- keys
					}
					if nextCursor == 0 {
						fmt.Println("nextCursor=0 exit")
						break
					}
				}
			}
			return nil
		})
		close(keyChannel)
		println("close keyChannel!")
	}()
	//开启一个协程查询 每一个key对应的fields
	go func() {
		fmt.Println("hscan")
		for {
			keys, ok := <-keyChannel
			if ok {
				for _, key := range keys {
					cursor := uint64(0)
					for {
						fields, nextCursor, ok := HSCAN(key, cursor, "*", env[CUR_SVC_ENV].HSCAN_COUNT)
						if ok {
							cursor = nextCursor
							//开始处理获取的field 和value
							if len(fields) != 0 {
								fmt.Println("[cronJob] key:", key, " hscan:", fields)
								for i, result := range fields {
									//result 此时是value，放入channel进行后续crd处理
									if i%2 == 1 {
										fieldsChannel <- keyAndField{key: key, field: fields[i-1], value: result}
									}
								}
							}
							if nextCursor == 0 {
								break
							}
						}
					}
				}
			} else {
				println("close fieldsChannel!")
				close(fieldsChannel)
				return
			}
		}
	}()

	//开启一个协程 删除或写crd
	go func() {
		fmt.Println("write")
		for {
			keyField, ok := <-fieldsChannel
			if ok {
				//判断value过期就删除，不过期就写crd
				//if IsExpired(keyField.value) {
				//删除
				//fmt.Println("[cronJob] delete:key=" + keyField.key + " field=" + keyField.field + " " + keyField.value)
				//HDEL(keyField.key, []string{keyField.field})
				//} else {
				//写crd
				fmt.Println("[cronJob] write:key=" + keyField.key + " field=" + keyField.field + " " + keyField.value)

				//}
			} else {
				println("close write!")
				return
			}
		}
	}()
}
