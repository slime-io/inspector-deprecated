package expire

import (
	"container/list"
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/go-logr/logr"
	"github.com/patrickmn/go-cache"
	ctrl "sigs.k8s.io/controller-runtime"

	dependencyAdapter "yun.netease.com/inspector/adapter"
	"yun.netease.com/inspector/constant"
	"yun.netease.com/inspector/controllers/configmap"
	"yun.netease.com/inspector/controllers/servicefence"
	microservicev1alpha1 "yun.netease.com/inspector/pkg/apis/microservice/v1alpha1"
	"yun.netease.com/inspector/utils"
)

type ExpireManager struct {
	Controller   *servicefence.ServiceFenceReconciler
	ResourceLock *configmap.ConfigMapReconciler
	Log          logr.Logger
}

func NewConfigManager(reconciler *servicefence.ServiceFenceReconciler, lock *configmap.ConfigMapReconciler) (*ExpireManager, error) {

	if reconciler == nil || lock == nil {
		return nil, fmt.Errorf("NewConfigManager    \"lock\" or  \"reconciler\" can not be empty!")
	}

	cm := &ExpireManager{
		Controller:   reconciler,
		ResourceLock: lock,
		Log:          ctrl.Log.WithName("ConfigManager"),
	}

	// init list CRD lock
	cm.ResourceLock.InitCronLock(constant.CronUpdateLockName)
	// init load-stableplatform lock
	//cm.ResourceLock.InitCronLock(constant.LoadFromStablePlatformLockName)

	//cm.ResourceLock.TestCreate("test-create")
	//cm.ResourceLock.TestUpdate("test-update")

	return cm, nil
}

// update CRD
func (c *ExpireManager) CRDCronUpdateCall() {

	if !c.ResourceLock.TryCronUpdateLock(constant.CronUpdateLockName) {
		c.Log.Info("CRDCronUpdateCall():  resource lock get fail")
		return
	}
	c.Log.Info("CRDCronUpdateCall():  resource lock get successful")
	// list crd
	crds, err := c.Controller.ListResource(constant.AllNamespaces)
	if err != nil {
		c.Log.Info("CRDCronUpdateCall(): list resource fail", "error", err)
	}
	var (
		//srcbuild string
		//desbuild string
		//sf *microservicev1alpha1.ServiceFence
		isUpdate bool
		//timestamp  int64
		deleteHost = list.New()
	)
	// traversal crd and search redis
	for _, item := range crds.Items {

		//srcbuild = stringFormat(item.Name, item.Namespace)
		isUpdate = false
		deleteHost.Init()

		for host, strategy := range item.Spec.Host {

			//str := strings.SplitN(host, ".", 3)
			//desbuild = stringFormat(str[0], str[1])

			// get redis value and update recently call
			//if value, flag := utils.HGET(srcbuild, desbuild); flag {
			//	timestamp, _ = strconv.ParseInt(value.(string), 10, 64)
			//	if strategy.RecentlyCalled.Seconds != timestamp {
			//		isUpdate = true
			//		strategy.RecentlyCalled.Seconds = timestamp
			//	}
			//}
			if isExpired(strategy) {
				deleteHost.PushBack(host)
				isUpdate = true
			}
		}

		//var spl []string
		// delete expire host and redis field
		for i := deleteHost.Front(); i != nil; i = i.Next() {
			delete(item.Spec.Host, i.Value.(string))
			//spl = strings.SplitN(i.Value.(string), ".", 3)
			//utils.HDEL(srcbuild, stringFormat(spl[0], spl[1]))
		}

		if isUpdate {
			if err := c.Controller.Update(context.TODO(), &item); err != nil {
				c.Log.Error(err, "update fail", "name", item.Name, "namespace", item.Namespace)
			} else {
				c.Log.Info("update resource successful ！", "name", item.Name, "namespace", item.Namespace)
			}
		}
	}

}

// 是否过期
func isExpired(strategy *microservicev1alpha1.RecyclingStrategy) bool {

	if strategy.Auto != nil {
		if (strategy.Auto.Duration.Seconds + strategy.RecentlyCalled.Seconds) < (time.Now().Unix()) {
			return true
		}
	} else if strategy.Deadline != nil {
		if strategy.Deadline.Expire.Seconds < (time.Now().Unix()) {
			return true
		}
	} else {
		return false
	}

	return false
}

func stringFormat(name string, ns string) string {

	var build strings.Builder
	build.WriteString(name)
	build.WriteString(".")
	build.WriteString(ns)

	return build.String()
}

// write cache to redis
func (c *ExpireManager) RedisCronUpdateCall() {
	// cache traversal
	mapCahe := dependencyAdapter.CacheDependency.Items()
	for key, item := range mapCahe {
		//c.Log.Info("cache", "key: ", key, "value: ", val)
		str := strings.Split(key, constant.CacheKeySplit)
		utils.HSET(str[0], str[1], item.Object.(*cache.Item).Object.(int64))
	}
	c.Log.Info("RedisCronUpdateCall(): writing cache to redis successful")
}
