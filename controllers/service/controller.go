package service

import (
	"context"
	"strings"
	"sync"
	"time"

	"github.com/go-logr/logr"
	"github.com/orcaman/concurrent-map"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/util/workqueue"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/source"
	"yun.netease.com/inspector/constant"
	con "yun.netease.com/inspector/constant"
	"yun.netease.com/inspector/controllers/servicefence"
)

var (
	NamespacesCache map[string]bool
	CacheMutex      = sync.RWMutex{}
	ToEgressCache   map[string]bool // 有svc无pod
	EgressMutex     = sync.RWMutex{}
	IP2Host         = cmap.New()
)

type ServiceReconciler struct {
	client.Client
	Log        logr.Logger
	Scheme     *runtime.Scheme
	Controller *servicefence.ServiceFenceReconciler
}

func (r *ServiceReconciler) Reconcile(req ctrl.Request) (ctrl.Result, error) {
	//r.Controller.CreateEmptyServiceFence(req.Name, req.Namespace)
	//.Log.Info("Reconcile", "name", req.Name, "ns", req.Namespace)
	return ctrl.Result{}, nil
}

func (r *ServiceReconciler) SetupWithManager(mgr ctrl.Manager) error {
	NamespacesCache = make(map[string]bool)
	ToEgressCache = make(map[string]bool)
	go func() {
		time.Sleep(10 * time.Second)
		r.redo()
	}()

	// service eventhandler
	svchandle := handler.Funcs{
		CreateFunc: func(createEvent event.CreateEvent, limitingInterface workqueue.RateLimitingInterface) {
			cip := createEvent.Object.(*v1.Service).Spec.ClusterIP
			if cip != "None" {
				IP2Host.Set(cip, createEvent.Meta.GetName()+"."+createEvent.Meta.GetNamespace())
			}
			// create sf based on service
			_, hasAnno := createEvent.Meta.GetAnnotations()[constant.SVCOpenAnnotationsKey]
			CacheMutex.RLock()
			if NamespacesCache[createEvent.Meta.GetNamespace()] {
				if hasAnno {
					r.Controller.CreateEmptyServiceFence(createEvent.Meta.GetName(), createEvent.Meta.GetNamespace(), true)
					r.Log.Info("create empty servicefence in coverns and enable", "name", createEvent.Meta.GetName(), "namespace", createEvent.Meta.GetNamespace())
				} else {
					r.Log.Info("create empty servicefence in coverns and disable", "name", createEvent.Meta.GetName(), "namespace", createEvent.Meta.GetNamespace())
					r.Controller.CreateEmptyServiceFence(createEvent.Meta.GetName(), createEvent.Meta.GetNamespace(), false)
				}
			}
			CacheMutex.RUnlock()
		},
		UpdateFunc: func(updateEvent event.UpdateEvent, limitingInterface workqueue.RateLimitingInterface) {
			cip := updateEvent.ObjectNew.(*v1.Service).Spec.ClusterIP
			if cip != "None" {
				IP2Host.Set(cip, updateEvent.MetaNew.GetName()+"."+updateEvent.MetaNew.GetNamespace())
			}
			// get annotations change event
			if _, oldhas := updateEvent.MetaOld.GetAnnotations()[constant.SVCOpenAnnotationsKey]; oldhas {
				// old has
				if _, newhas := updateEvent.MetaNew.GetAnnotations()[constant.SVCOpenAnnotationsKey]; !newhas {
					// new doesn't has
					r.Log.Info("handle func: find svc  delete the annotations", "name", updateEvent.MetaNew.GetName(), "namespace", updateEvent.MetaNew.GetNamespace())
					CacheMutex.RLock()
					if NamespacesCache[updateEvent.MetaNew.GetNamespace()] {
						// disable sf
						if ins := r.Controller.GetResource(updateEvent.MetaNew.GetName(), updateEvent.MetaNew.GetNamespace()); ins != nil {
							// already has servicefence, update
							ins.Spec.Enable = false
							r.Update(context.TODO(), ins)
						} else {
							r.Controller.CreateEmptyServiceFence(updateEvent.MetaNew.GetName(), updateEvent.MetaNew.GetNamespace(), false)
						}

					}
					CacheMutex.RUnlock()
					//r.ServiceFenceDisable(updateEvent.MetaNew.GetNamespace())
				}
			} else {
				// old doesn't has
				if _, newhas := updateEvent.MetaNew.GetAnnotations()[constant.SVCOpenAnnotationsKey]; newhas {
					// new has
					r.Log.Info("handle func: find add the annotations", "name", updateEvent.MetaNew.GetName(), "namespace", updateEvent.MetaNew.GetNamespace())
					CacheMutex.RLock()
					if NamespacesCache[updateEvent.MetaNew.GetNamespace()] {
						// enable sf
						if ins := r.Controller.GetResource(updateEvent.MetaNew.GetName(), updateEvent.MetaNew.GetNamespace()); ins != nil {
							// already has servicefence, update
							ins.Spec.Enable = true
							r.Update(context.TODO(), ins)
						} else {
							r.Controller.CreateEmptyServiceFence(updateEvent.MetaNew.GetName(), updateEvent.MetaNew.GetNamespace(), true)
						}
					}
					CacheMutex.RUnlock()
					//r.ServiceFenceEnable(updateEvent.MetaNew.GetNamespace())
				}
			}
		},
		DeleteFunc: func(deleteEvent event.DeleteEvent, limitingInterface workqueue.RateLimitingInterface) {
			cip := deleteEvent.Object.(*v1.Service).Spec.ClusterIP
			IP2Host.Pop(cip)
		},
		GenericFunc: func(genericEvent event.GenericEvent, limitingInterface workqueue.RateLimitingInterface) {},
	}

	// namespace eventhandler
	nshandle := handler.Funcs{
		CreateFunc: func(createEvent event.CreateEvent, limitingInterface workqueue.RateLimitingInterface) {
			// init NamespacesCache
			if _, has := createEvent.Meta.GetLabels()[constant.NSOpenLabelKey]; has {
				CacheMutex.Lock()
				NamespacesCache[createEvent.Meta.GetName()] = true
				CacheMutex.Unlock()
			} else {
				CacheMutex.Lock()
				NamespacesCache[createEvent.Meta.GetName()] = false
				CacheMutex.Unlock()
			}
			r.Log.Info("init NamespacesCache", "name", createEvent.Meta.GetName(), "label_status", NamespacesCache[createEvent.Meta.GetName()])
		},
		UpdateFunc: func(updateEvent event.UpdateEvent, limitingInterface workqueue.RateLimitingInterface) {
			// get label change event
			if _, oldhas := updateEvent.MetaOld.GetLabels()[constant.NSOpenLabelKey]; oldhas {
				// old has
				if _, newhas := updateEvent.MetaNew.GetLabels()[constant.NSOpenLabelKey]; !newhas {
					// new doesn't has
					r.Log.Info("handle func: find namespace delete the label", "name", updateEvent.MetaNew.GetName(), "namespace", updateEvent.MetaNew.GetNamespace())
					CacheMutex.Lock()
					NamespacesCache[updateEvent.MetaNew.GetName()] = false
					r.ServiceFenceDisable(updateEvent.MetaNew.GetName())
					CacheMutex.Unlock()
				}
			} else {
				// old doesn't has
				if _, newhas := updateEvent.MetaNew.GetLabels()[constant.NSOpenLabelKey]; newhas {
					// new has
					r.Log.Info("handle func: find namespace add the label", "name", updateEvent.MetaNew.GetName())
					CacheMutex.Lock()
					NamespacesCache[updateEvent.MetaNew.GetName()] = true
					r.ServiceFenceEnable(updateEvent.MetaNew.GetName())
					CacheMutex.Unlock()
				}
			}
		},
		DeleteFunc:  func(deleteEvent event.DeleteEvent, limitingInterface workqueue.RateLimitingInterface) {},
		GenericFunc: func(genericEvent event.GenericEvent, limitingInterface workqueue.RateLimitingInterface) {},
	}

	ephandle := handler.Funcs{
		CreateFunc: func(createEvent event.CreateEvent, limitingInterface workqueue.RateLimitingInterface) {
			ep := createEvent.Object.DeepCopyObject().(*v1.Endpoints)
			r.handlerToEgressCache(ep)
			for _, subset := range ep.Subsets {
				for _, address := range subset.Addresses {
					IP2Host.Set(address.IP, createEvent.Meta.GetName()+"."+createEvent.Meta.GetNamespace())
				}
			}
		},
		UpdateFunc: func(updateEvent event.UpdateEvent, limitingInterface workqueue.RateLimitingInterface) {
			ep := updateEvent.ObjectNew.DeepCopyObject().(*v1.Endpoints)
			r.handlerToEgressCache(ep)
			for _, subset := range ep.Subsets {
				for _, address := range subset.Addresses {
					IP2Host.Set(address.IP, updateEvent.MetaNew.GetName()+"."+updateEvent.MetaNew.GetNamespace())
				}
			}
		},
		DeleteFunc: func(deleteEvent event.DeleteEvent, limitingInterface workqueue.RateLimitingInterface) {
			ep := deleteEvent.Object.DeepCopyObject().(*v1.Endpoints)
			for _, subset := range ep.Subsets {
				for _, address := range subset.Addresses {
					IP2Host.Pop(address.IP)
				}
			}
		},
		GenericFunc: func(genericEvent event.GenericEvent, limitingInterface workqueue.RateLimitingInterface) {},
	}

	return ctrl.NewControllerManagedBy(mgr).For(&v1.Service{}).
		Watches(&source.Kind{Type: &v1.Namespace{}}, nshandle).
		Watches(&source.Kind{Type: &v1.Service{}}, svchandle).
		Watches(&source.Kind{Type: &v1.Endpoints{}}, ephandle).
		Complete(r)
}

// list service resource
func (r *ServiceReconciler) ListResource(ns string) (*v1.ServiceList, error) {

	res := &v1.ServiceList{}
	opt := &client.ListOptions{
		Namespace: ns,
	}
	err := r.List(context.TODO(), res, opt)
	if err != nil {
		r.Log.Error(err, "list Service fail")
	} else {
		r.Log.Info("List Service successful")
	}
	//r.Log.Info(" print resource list:   ", "content", res)
	//for _, svc := range res.Items {
	//	r.Log.Info("List Service", "name", svc.Name, "namespace", svc.Namespace)
	//}
	return res, err
}

// enable servicefence according to annotations in current ns
func (r *ServiceReconciler) ServiceFenceEnable(ns string) {

	// list svc at this ns，check annotations
	services, err := r.ListResource(ns)
	if err != nil {
		r.Log.Info("ServiceFenceEnable(): list service fail", "error", err)
		return
	}

	//traversal service , if svc's annotation is true, enable
	for _, item := range services.Items {
		if _, ok := item.ObjectMeta.Annotations[constant.SVCOpenAnnotationsKey]; ok {
			if ins := r.Controller.GetResource(item.Name, item.Namespace); ins != nil {
				// already has servicefence, update
				ins.Spec.Enable = true
				r.Update(context.TODO(), ins)
			} else {
				r.Controller.CreateEmptyServiceFence(item.Name, item.Namespace, true)
			}
		} else {
			r.Controller.CreateEmptyServiceFence(item.Name, item.Namespace, false)
		}
	}
	r.Log.Info("finish enable servicefence according to annotations in current namespace", "namespace: ", ns)
}

// disable servicefence in current ns
func (r *ServiceReconciler) ServiceFenceDisable(ns string) {

	// list sf at this ns, close
	crds, err := r.Controller.ListResource(ns)
	if err != nil {
		r.Log.Info("ServiceFenceDisable(): list resource fail", "error", err)
		return
	}
	// traversal crd
	for _, item := range crds.Items {
		if item.Spec.Enable {
			item.Spec.Enable = false
			if err := r.Controller.Update(context.TODO(), &item); err != nil {
				r.Log.Error(err, "update fail", "name", item.Name, "namespace", item.Namespace)
			} else {
				r.Log.Info("update resource successful ！", "name", item.Name, "namespace", item.Namespace)
			}
		}
	}
}

// init empty servicefence in specified namespaces
func (r *ServiceReconciler) InitServiceFence(namespaces string) {

	strs := strings.Split(namespaces, ",")
	var svcName string
	var svcNs string
	for _, ns := range strs {
		// list service in namespace
		services, err := r.ListResource(ns)
		if err != nil {
			r.Log.Info("InitServiceFence(): list service fail", "error", err)
			continue
		}

		//traversal service and create empty servicefence
		for _, item := range services.Items {
			svcName = item.Name
			svcNs = item.Namespace
			r.Controller.CreateEmptyServiceFence(svcName, svcNs, false)
		}
	}

	r.Log.Info("Init empty servicefence in specified namespaces successful", "namespaces:", namespaces)
}

// 组件启动时，可能还没有ns的cache内容，导致创建的sf的开关是关闭的。开关修正
func (r *ServiceReconciler) redo() {
	r.Log.Info("redo() begin!!!")
	for key, val := range NamespacesCache {
		if val {
			r.ServiceFenceEnable(key)
		} else {
			r.ServiceFenceDisable(key)
		}
	}
}

func (r *ServiceReconciler) handlerToEgressCache(ep *v1.Endpoints) {
	hasEndpoints := false
	if ep.Subsets != nil && len(ep.Subsets) > 0 {
		for _, subset := range ep.Subsets {
			if subset.Addresses != nil && len(subset.Addresses) > 0 {
				hasEndpoints = true
				break
			}
		}
	}
	var cachebuild string
	if hasEndpoints {
		cachebuild = stringFormat(ep.Name, ep.Namespace, con.CacheKeySplit)
		EgressMutex.Lock()
		ToEgressCache[cachebuild] = false
		EgressMutex.Unlock()
		//r.Log.Info("handle func: find endpoint event", "name", ep.Name, "namespace", ep.Namespace)
	} else {
		cachebuild = stringFormat(ep.Name, ep.Namespace, con.CacheKeySplit)
		EgressMutex.Lock()
		ToEgressCache[cachebuild] = true
		EgressMutex.Unlock()
		//r.Log.Info("handle func: find endpoint event and request to egress", "name", ep.Name, "namespace", ep.Namespace)
	}
}

func stringFormat(name string, ns string, con string) string {

	var build strings.Builder
	build.WriteString(name)
	build.WriteString(con)
	build.WriteString(ns)

	return build.String()
}
