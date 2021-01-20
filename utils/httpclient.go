package utils

import (
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"strconv"
	"strings"
	"time"

	"yun.netease.com/inspector/controllers/servicefence"
)

type Data struct {
	From      string   `json:"name"`
	To        []string `json:"downDependService"`
	TimeStamp string   `json:"timeStamp"`
}
type Ret struct {
	Code int    `json:"code"`
	Data []Data `json:"data"`
}

func GetDataFromCaesar(controllerReconciler *servicefence.ServiceFenceReconciler, defaultStrategy string, defaultStrategyTime int64) {
	//1.开始解析返回的数据
	body, ok := DoPostByHttpclient(env[CUR_SVC_ENV].CAESAR_URI + getTimestamp())
	if !ok {
		return
	}
	ret := Ret{}
	err := json.Unmarshal(body, &ret)
	if err != nil {
		fmt.Println("[GetDataFromCaesar] Unmarshal failed.")
	}

	for _, data := range ret.Data {
		//2. 开始写入redis
		key, fields, ok := BuildEntityByHMSET(data)
		if !ok {
			fmt.Println("BuildEntityByHMSET Failed")
		}
		if !HMSET(key, fields) {
			fmt.Println("HMSET Failed")
		}
		//3. 开始写入crd
		sour_info := strings.Split(key, ".")
		for field, _ := range fields {
			des_info := strings.Split(field, ".")
			//fmt.Println(sour_info[0],sour_info[1],des_info[0],des_info[1])
			sf := servicefence.ConstructServiceFence(sour_info[0], sour_info[1], des_info[0], des_info[1], defaultStrategy, defaultStrategyTime, false)
			controllerReconciler.UpdateResource(sf)
		}
	}
}

/**
调用httpclient发送post请求
返回：body、true表示返回了200
*/
func DoPostByHttpclient(url string) ([]byte, bool) {
	client := &http.Client{}
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		fmt.Println("[DoPostFromCaesar] NewRequest failed.")
	}

	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := client.Do(req)
	if resp.StatusCode != 200 {
		fmt.Println("[DoPostFromCaesar] Do failed!")
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		fmt.Println("[DoPostFromCaesar] ReadBody failed!")
		return nil, false
	}
	return body, true
}

/**
获取当天零点的时间戳
*/
func getTimestamp() string {
	t := time.Now()
	tm := time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, t.Location())
	return strconv.FormatInt(tm.Unix(), 10)
}

/**
模拟接口
*/
func MockHTTPServer() {
	data := Data{From: "czl-yanxuan1-a", To: []string{"czl-yanxuan-b", "czl-yanxuan-c", "czl-yanxuan-d", "czl-yanxuan-f", "czl-yanxuan-g", "czl-yanxuan-h"}}
	data2 := Data{From: "czl-yanxuan2-a", To: []string{"czl-yanxuan2-b", "czl-yanxuan2-c", "czl-yanxuan2-d", "czl-yanxuan2-f", "czl-yanxuan2-g"}}
	data3 := Data{From: "czl-yanxuan3-a", To: []string{"czl-yanxuan3-b"}}
	data4 := Data{From: "czl-yanxuan4-a", To: []string{"czl-yanxuan4-b"}}
	data5 := Data{From: "czl-yanxuan5-a", To: []string{"czl-yanxuan5-b"}}
	data6 := Data{From: "czl-yanxuan6-a", To: []string{"czl-yanxuan6-b"}}
	data7 := Data{From: "czl-yanxuan7-a", To: []string{"czl-yanxuan7-b"}}
	ret := new(Ret)
	ret.Code = 200
	ret.Data = append(ret.Data, data, data2, data3, data4, data5, data6, data7)
	ret_json, _ := json.Marshal(ret)
	//fmt.Println("ret_json:",string(ret_json))
	http.HandleFunc("/service/downDependService", func(w http.ResponseWriter, r *http.Request) {
		io.WriteString(w, string(ret_json))
	})
	http.ListenAndServe("127.0.0.1:8080", nil)
}
