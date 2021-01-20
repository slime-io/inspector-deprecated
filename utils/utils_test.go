package utils_test

import (
	"fmt"
	"reflect"
	"time"

	"github.com/go-redis/redis"
	. "github.com/onsi/ginkgo"

	"yun.netease.com/inspector/utils"
)

var _ = Describe("Utils", func() {

	BeforeEach(func() {
		By("BeforeEach")
	})

	JustBeforeEach(func() {
		By("JustBeforeEach")
	})

	AfterEach(func() {
		By("AfterEach")
	})

	//开始建立redis连接,开启接口服务器
	BeforeSuite(func() {
		utils.RedisInit("dev")
		go func() { utils.MockHTTPServer() }()
	})
	Describe("redis测试", func() {
		Context("redis写与查", func() {
			var (
				key   = "czl-hmset-origin.dev"
				field = []string{"czl-hmset-des-1.dev", "czl-hmset-des-2.dev", "czl-hmset-des-3.dev", "czl-hmset-des-4.dev", "czl-hmset-des-5.dev", "czl-hmset-des-6.dev"}
			)
			It("hset", func() {
				fmt.Println("hset:", utils.HSET("czl-hset.dev", "czl-field", time.Now().Unix()))
			})
			It("hget", func() {
				result, ok := utils.HGET_INT64("czl-hset.dev", "czl-field")
				fmt.Println("hget:rt type=", reflect.TypeOf(result), " rt=", result, " ok=", ok)
			})

			It("HMSET", func() {
				data := utils.Data{From: key, To: field}

				key, fields, ok := utils.BuildEntityByHMSET(data)
				if !ok {
					Fail("BuildEntityByHMSET Failed")
				}
				if utils.HMSET(key, fields) == false {
					Fail("BuildEntityByHMSET Failed")
				}
			})

			It("hmget", func() {
				result, ok := utils.HMGET_INT64(key, field)
				fmt.Println("hmget:rt=", result, " ok=", ok)
			})

			It("scan", func() {
				_ = utils.RedisClient.ForEachMaster(func(client *redis.Client) error {
					fmt.Println("client=", client)
					cursor := uint64(0)
					for {
						keys, nextCursor, ok := utils.SCAN(client, cursor, "*", 2)
						if ok {
							cursor = nextCursor
							if len(keys) != 0 {
								fmt.Println("scan:key={} nextCursor={} client={}", keys, nextCursor, client)
							}

							if nextCursor == 0 {
								fmt.Println("nextCursor=0 exit")
								break
							}
						}
					}
					return nil
				})
			})
			It("hscan", func() {
				cursor := uint64(0)
				for {
					fields, nextCursor, ok := utils.HSCAN("czl-yanxuan3-a.dev", cursor, "*", 2)
					if ok {
						cursor = nextCursor
						if len(fields) != 0 {
							fmt.Println("hscan:fields=", fields)
						}
						if nextCursor == 0 {
							break
						}
					}

				}
			})

			It("hexists-before", func() {
				ok := utils.HEXISTS(key, field[0])
				fmt.Println("hexists=", ok)
			})
			It("hdel", func() {
				ok := utils.HDEL(key, field)
				fmt.Println("hdel=", ok)
			})
			It("hexists-after", func() {
				ok := utils.HEXISTS(key, field[0])
				fmt.Println("hexists=", ok)
			})
		})
	})

	Describe("cron和httpclient测试", func() {
		Context("httpclient测试", func() {
			It("httpclient测试", func() {
				utils.GetDataFromCaesar(nil, "", 0)
			})
		})
		Context("cron测试", func() {
			It("cron测试", func() {
				utils.StartCron()
			})
		})
	})
})
