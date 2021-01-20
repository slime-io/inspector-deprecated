package configmap

import (
	"context"
	"os"
	"sync"
	"time"

	"github.com/go-logr/logr"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	"yun.netease.com/inspector/constant"
)

type ConfigMapReconciler struct {
	client.Client
	Log            logr.Logger
	Scheme         *runtime.Scheme
	lock           sync.Mutex
	LockCollection map[string]*v1.ConfigMap
}

func (r *ConfigMapReconciler) Reconcile(req ctrl.Request) (ctrl.Result, error) {
	return ctrl.Result{}, nil
}

func (r *ConfigMapReconciler) SetupWithManager(mgr ctrl.Manager) error {
	fun := predicate.Funcs{
		CreateFunc: func(event event.CreateEvent) bool {
			return false
		},
		DeleteFunc: func(deleteEvent event.DeleteEvent) bool {
			return false
		},
		UpdateFunc: func(updateEvent event.UpdateEvent) bool {
			return false
		},
		GenericFunc: func(genericEvent event.GenericEvent) bool {
			return false
		},
	}
	return ctrl.NewControllerManagedBy(mgr).
		For(&v1.ConfigMap{}).WithEventFilter(fun).
		Complete(r)
}

func (r *ConfigMapReconciler) GetResourceVersion(name string, namespace string) string {

	vm := &v1.ConfigMap{}
	if err := r.Get(context.TODO(), types.NamespacedName{namespace, name}, vm); err != nil {
		r.Log.Info("unable to fetch Configmap ,cluster have no resource", "name", name, "namespace", namespace)
		return "0"
	} else {
		r.Log.Info("get already exist configmap resource ！", "name", name, "namespace", namespace)
		return vm.ResourceVersion
	}
}

// init the lock in k8s
func (r *ConfigMapReconciler) InitCronLock(name string) {
	// get lock
	configmap := r.constructResourceLock(name)

	// lock is absent and create
	if err := r.Create(context.TODO(), configmap); err != nil {
		r.Log.Info("create resource lock fail", "name", configmap.Name, "namespace", configmap.Namespace)
	} else {
		r.Log.Info("create resource lock successful ", "name", configmap.Name, "namespace", configmap.Namespace)
	}
	go func() {
		time.Sleep(3 * time.Second)
		configmap.ResourceVersion = r.GetResourceVersion(name, constant.LockNameSpace)
	}()
}

// crontab, update Servicefence last access times
func (r *ConfigMapReconciler) TryCronUpdateLock(name string) bool {
	// get lock
	configmap := r.constructResourceLock(name)
	// update lock
	if err := r.Update(context.TODO(), configmap); err != nil {
		r.Log.Info("update resource lock fail！", "name", configmap.Name, "namespace", configmap.Namespace)
		go func() {
			time.Sleep(3 * time.Second)
			configmap.ResourceVersion = r.GetResourceVersion(name, constant.LockNameSpace)
		}()
		return false
	} else {
		r.Log.Info("update resource lock successful ！", "name", configmap.Name, "namespace", configmap.Namespace)
		go func() {
			time.Sleep(3 * time.Second)
			configmap.ResourceVersion = r.GetResourceVersion(name, constant.LockNameSpace)
		}()
		return true
	}
}

// get resource lock
func (r *ConfigMapReconciler) constructResourceLock(name string) *v1.ConfigMap {

	r.lock.Lock()
	defer r.lock.Unlock()

	if val, ok := r.LockCollection[name]; ok {
		val.Data["updateTime"] = time.Unix(time.Now().Unix(), 0).Format("2006-01-02 15:04:05")
		return val
	} else {
		cflock := &v1.ConfigMap{
			TypeMeta: metav1.TypeMeta{
				Kind:       "ConfigMap",
				APIVersion: "v1",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:      name,
				Namespace: constant.LockNameSpace,
			},
			Data: map[string]string{
				"updateTime": time.Unix(time.Now().Unix(), 0).Format("2006-01-02 15:04:05"),
				"owner":      os.Getenv("POD_NAME"),
			},
			BinaryData: nil,
		}
		r.LockCollection[name] = cflock
		return cflock
	}
}

func (r *ConfigMapReconciler) TestCreate(name string) {
	configmap := r.constructResourceLock(name)
	if err := r.Create(context.TODO(), configmap); err != nil {
		r.Log.Info("test create resource lock fail", "name", configmap.Name, "namespace", configmap.Namespace)
	} else {
		r.Log.Info("test create resource lock ok", "name", configmap.Name, "namespace", configmap.Namespace)
	}
}

func (r *ConfigMapReconciler) TestUpdate(name string) {
	configmap := r.constructResourceLock(name)
	if err := r.Update(context.TODO(), configmap); err != nil {
		r.Log.Info("test update resource lock fail！", "name", configmap.Name, "namespace", configmap.Namespace)
	} else {
		r.Log.Info("test update resource lock successful ！", "name", configmap.Name, "namespace", configmap.Namespace)
	}

}
