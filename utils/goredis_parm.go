package utils

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

/**
构建redis MSET的参数
输入：data类型
输出：key、field与value组成的map、构建结果
*/
func BuildEntityByHMSET(data Data) (string, map[string]interface{}, bool) {
	if data.From == "" {
		return "", nil, false
	}
	addServiceEnv(&data.From)
	rt := make(map[string]interface{})
	for _, to := range data.To {
		if to == "" {
			continue
		}
		addServiceEnv(&to)
		rt[to] = time.Now().Unix()
	}
	return data.From, rt, true
}

/**
在key上增加ns信息
*/
func addServiceEnv(svc *string) {
	if strings.Index(*svc, ".") == -1 {
		*svc = *svc + "." + env[CUR_SVC_ENV].SVC_NS
	}
}

/**
将Interface 转换为string
*/
func InterfaceToString(param interface{}) (string, bool) {
	value, ok := param.(string)
	return value, ok
}

/**
将Interface数组 转换为string数组
*/
func InterfacesToStrings(params []interface{}) []string {
	var paramSlice []string
	for _, param := range params {
		if value, ok := InterfaceToString(param); ok {
			paramSlice = append(paramSlice, value)
		} else {
			fmt.Print("InterfacesToStrings failed: param=", param)
			return nil
		}
	}
	return paramSlice
}

/**
将Interface 转换为int64
*/
func InterfaceToInt64(param interface{}) (int64, bool) {
	value, err := strconv.ParseInt(param.(string), 10, 64)
	if err != nil {
		fmt.Println("InterfaceToInt64 failed. err=", err.Error())
		return 0, false
	}
	return value, true
}

/**
将Interface数组 转换为int数组
*/
func InterfacesToInt64s(params []interface{}) []int64 {
	var paramSlice []int64
	for _, param := range params {
		if value, ok := InterfaceToInt64(param); ok {
			paramSlice = append(paramSlice, value)
		} else {
			fmt.Print("InterfacesToInts failed: param=", param)
			return nil
		}
	}
	return paramSlice
}
