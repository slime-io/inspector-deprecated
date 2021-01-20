package utils

import "time"

const (
	REDIS_KEY_PREFFIX = "inspector_"
)

var (
	CUR_SVC_ENV = "dev" //当前环境
	env         = map[string]ENV_INFORMATION{
		"online": {SVC_NS: "online", CAESAR_URI: "http://127.0.0.1:8550/proxy/online.yanxuan-dependence-api-web.service.mailsaas/service/downDependService?timestamp=", ADDRS: []string{""}, PASSWORD: "",
			DIAL_TIMEOUT: time.Second * 5, READ_TIMEOUT: time.Second * 3, WRITE_TIMEOUT: time.Second * 3,
			POOL_TIMEOUT: time.Second * 4, IDLE_TIMEOUT: time.Minute * 5,
			POOL_SIZE: 100, MIN_IDLE_CONNS: 20, SCAN_COUNT: 500, HSCAN_COUNT: 50,
			CRON_SCHEDULE: "@daily"},
		"test": {SVC_NS: "test", CAESAR_URI: "http://127.0.0.1:8550/proxy/test.yanxuan-dependence-api-web.service.mailsaas/service/downDependService?timestamp=", ADDRS: []string{"10.200.178.233:16398"}, PASSWORD: "",
			DIAL_TIMEOUT: time.Second * 5, READ_TIMEOUT: time.Second * 3, WRITE_TIMEOUT: time.Second * 3,
			POOL_TIMEOUT: time.Second * 4, IDLE_TIMEOUT: time.Minute * 5,
			POOL_SIZE: 100, MIN_IDLE_CONNS: 20, SCAN_COUNT: 500, HSCAN_COUNT: 50,
			CRON_SCHEDULE: "@every 10m"},
		"dev": {SVC_NS: "dev", CAESAR_URI: "http://localhost:8080/service/downDependService?timestamp=", ADDRS: []string{"10.178.7.11:16379"}, PASSWORD: "",
			DIAL_TIMEOUT: time.Second * 5, READ_TIMEOUT: time.Second * 3, WRITE_TIMEOUT: time.Second * 3,
			POOL_TIMEOUT: time.Second * 4, IDLE_TIMEOUT: time.Minute * 5,
			POOL_SIZE: 100, MIN_IDLE_CONNS: 20, SCAN_COUNT: 500, HSCAN_COUNT: 50,
			CRON_SCHEDULE: "@every 30s"},
	}
)

type ENV_INFORMATION struct {
	SVC_NS        string   //从稳定性平台拉取的数据，默认使用的ns
	CAESAR_URI    string   //稳定性平台uri
	ADDRS         []string //redis地址
	PASSWORD      string   //redis密码
	DIAL_TIMEOUT  time.Duration
	READ_TIMEOUT  time.Duration
	WRITE_TIMEOUT time.Duration
	POOL_TIMEOUT  time.Duration
	IDLE_TIMEOUT  time.Duration

	POOL_SIZE      int   //连接池大小
	MIN_IDLE_CONNS int   //最小空闲连接
	SCAN_COUNT     int64 //redis scan前缀搜索时一次返回的记录数
	HSCAN_COUNT    int64 //redis hscan前缀搜索时一次返回的记录数

	CRON_SCHEDULE string //定时任务周期
}
