/*


Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package servicefence

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/rand"
	"k8s.io/client-go/util/retry"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	con "yun.netease.com/inspector/constant"
	microservicev1alpha1 "yun.netease.com/inspector/pkg/apis/microservice/v1alpha1"
)

var (
	log                      = ctrl.Log.WithName("serviceFence_controller")
	defaultBaseTime          int64
	defaultRecyclingStrategy = &microservicev1alpha1.RecyclingStrategy{
		Stable: nil,
		Deadline: &microservicev1alpha1.RecyclingStrategy_Deadline{
			Expire: &microservicev1alpha1.Timestamp{
				Seconds: time.Now().Unix() + 24*60*60, // 1d
				Nanos:   0,
			},
		},
		Auto: nil,
		RecentlyCalled: &microservicev1alpha1.Timestamp{
			Seconds: time.Now().Unix(),
			Nanos:   0,
		},
	}
)

// ServiceFenceReconciler reconciles a ServiceFence object
type ServiceFenceReconciler struct {
	client.Client
	Log    logr.Logger
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=microservice.netease.com,resources=servicefences,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=microservice.netease.com,resources=servicefences/status,verbs=get;update;patch

func (r *ServiceFenceReconciler) Reconcile(req ctrl.Request) (ctrl.Result, error) {
	return ctrl.Result{}, nil
}

func (r *ServiceFenceReconciler) SetupWithManager(mgr ctrl.Manager, defaultTime int64) error {
	// init default Deadline time
	defaultRecyclingStrategy.Deadline.Expire.Seconds = time.Now().Unix() + rand.Int63nRange(defaultTime, defaultTime*30)
	defaultBaseTime = defaultTime

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
		For(&microservicev1alpha1.ServiceFence{}).WithEventFilter(fun).
		Complete(r)
}

func (r *ServiceFenceReconciler) GetResource(name string, namespace string) *microservicev1alpha1.ServiceFence {

	vm := &microservicev1alpha1.ServiceFence{}
	if err := r.Get(context.TODO(), types.NamespacedName{namespace, name}, vm); err != nil {
		r.Log.Info("unable to fetch ServiceFence ,cluster have no resource", "name", name, "namespace", namespace)
		return nil
	} else {
		r.Log.Info("get already exist servicefence resource ！", "name", name, "namespace", namespace)
		return vm
	}
}

func (r *ServiceFenceReconciler) UpdateResource(servicefence *microservicev1alpha1.ServiceFence) {

	err := retry.RetryOnConflict(retry.DefaultBackoff, func() error {
		// update or create
		if ins := r.GetResource(servicefence.Name, servicefence.Namespace); ins != nil {
			// if new host
			if ismerge := merge(servicefence, ins); ismerge {
				if err := r.Update(context.TODO(), ins); err != nil {
					r.Log.Error(err, "update resource conflict！ retry auto", "name", ins.Name, "namespace", ins.Namespace)
					return err
				} else {
					r.Log.Info("update resource successful ！", "name", ins.Name, "namespace", ins.Namespace)
					fmt.Println(ins.Name, "+", ins.Namespace, "+", ins.Spec.Host)
					return nil
				}
			} else {
				r.Log.Info("no new host to merge, resource not change", "name", ins.Name, "namespace", ins.Namespace)
				return nil
			}
		} else {
			if err := r.Create(context.TODO(), servicefence); err != nil {
				r.Log.Error(err, "create resource fail", "name", servicefence.Name, "namespace", servicefence.Namespace)
				return err
			} else {
				r.Log.Info("create resource successful！", "name", servicefence.Name, "namespace", servicefence.Namespace)
				return nil
			}
		}
	})
	if err != nil {
		r.Log.Error(err, "update conflict, max retries were hit, update fail", "name", servicefence.Name, "namespace", servicefence.Namespace)
	}
}

func (r *ServiceFenceReconciler) DeleteResource(name string, namespace string) error {

	return nil
}

// list ServiceFence in namespace
func (r *ServiceFenceReconciler) ListResource(ns string) (*microservicev1alpha1.ServiceFenceList, error) {

	opt := &client.ListOptions{}
	if ns != con.AllNamespaces {
		opt.Namespace = ns
	}

	res := &microservicev1alpha1.ServiceFenceList{}
	err := r.List(context.TODO(), res, opt)
	if err != nil {
		r.Log.Error(err, "list resource fail")
	} else {
		r.Log.Info("List ServiceFence Resource successful")
	}
	//log.Info(" print resource list:   ", "content", res)
	//for _, sf := range res.Items {
	//	log.Info("List ServiceFence Resource", "name", sf.Name, "namespace", sf.Namespace)
	//}
	return res, err
}

// merge into old struct
func merge(new *microservicev1alpha1.ServiceFence, old *microservicev1alpha1.ServiceFence) bool {

	if (new.Name != old.Name) || (new.Namespace != old.Namespace) {
		return false
	}
	if old.Spec.Host == nil {
		old.Spec.Host = map[string]*microservicev1alpha1.RecyclingStrategy{}
	}
	if new.Spec.Host == nil {
		new.Spec.Host = map[string]*microservicev1alpha1.RecyclingStrategy{}
	}

	isMerge := false
	for k, vNew := range new.Spec.Host {
		if vOld, has := old.Spec.Host[k]; has {
			if vNew != nil && vNew.Deadline.Expire.Seconds > vOld.Deadline.Expire.Seconds {
				isMerge = true
				vOld.Deadline.Expire.Seconds = vNew.Deadline.Expire.Seconds
			}
		} else {
			isMerge = true
			if vNew == nil {
				old.Spec.Host[k] = defaultRecyclingStrategy
				old.Spec.Host[k].RecentlyCalled.Seconds = time.Now().Unix()
				old.Spec.Host[k].Deadline.Expire.Seconds = time.Now().Unix() + rand.Int63nRange(defaultBaseTime, defaultBaseTime*30)
			} else {
				old.Spec.Host[k] = vNew
			}
		}
	}

	return isMerge
}

// generate ServiceFence structure
// sourceServiceName, sourceServiceNamespace, destinationServiceName, destinationServiceNamespace, dependencyManagementstrategy， expireTime
func ConstructServiceFence(sourName string, sourNs string, desName string, desNs string, strategy string, strategyTime int64, enable bool) *microservicev1alpha1.ServiceFence {

	if sourName == "" || sourNs == "" {
		log.Info("servicefence construct fail, sourceServiceName or sourceServiceNamespace can't be nil")
		return nil
	}

	sf := &microservicev1alpha1.ServiceFence{}
	sf.Name = sourName
	sf.Namespace = sourNs
	sf.Spec.Enable = enable
	sf.Spec.Host = make(map[string]*microservicev1alpha1.RecyclingStrategy)

	// construct a empty servicefence
	if desName == "" || desNs == "" {
		return sf
	}

	var build strings.Builder
	build.WriteString(desName)
	build.WriteString(".")
	build.WriteString(desNs)
	build.WriteString(con.HOST_SUFFIX)

	if len(strategy) == 0 || strategyTime == 0 {
		sf.Spec.Host[build.String()] = defaultRecyclingStrategy
		return sf
	}

	sf.Spec.Host[build.String()] = constructRecyclingStrategy(strategy, strategyTime)
	log.Info("servicefence construct successful", "servicefence", sf)
	return sf
}

// generate ServiceFence Strategy structure
func constructRecyclingStrategy(strategy string, strategyTime int64) *microservicev1alpha1.RecyclingStrategy {

	rs := &microservicev1alpha1.RecyclingStrategy{}

	switch strategy {
	case con.Stable:
		rs.Stable = &microservicev1alpha1.RecyclingStrategy_Stable{}
		break
	case con.Deadline:
		rs.Deadline = &microservicev1alpha1.RecyclingStrategy_Deadline{
			Expire: ConstructTimestamp(time.Now().Unix()+strategyTime, 0),
		}
		break
	case con.Auto:
		rs.Auto = &microservicev1alpha1.RecyclingStrategy_Auto{
			Duration: ConstructTimestamp(strategyTime, 0),
		}
		break
	default:
		return defaultRecyclingStrategy
	}

	rs.RecentlyCalled = &microservicev1alpha1.Timestamp{
		Seconds: time.Now().Unix(),
		Nanos:   0,
	}

	return rs
}

// generate ServiceFence Timestamp structure
func ConstructTimestamp(sec int64, na int32) *microservicev1alpha1.Timestamp {
	return &microservicev1alpha1.Timestamp{
		Seconds: sec,
		Nanos:   na,
	}
}

// // create a empty ServiceFence structure in k8s
func (r *ServiceFenceReconciler) CreateEmptyServiceFence(sour_name string, sour_ns string, enable bool) bool {

	sf := ConstructServiceFence(sour_name, sour_ns, "", "", "", 0, enable)
	if err := r.Create(context.TODO(), sf); err != nil {
		r.Log.Error(err, "create resource fail", "name", sour_name, "namespace", sour_ns)
		return false
	} else {
		r.Log.Info("create resource successful！", "name", sour_name, "namespace", sour_ns)
		return true
	}
}
