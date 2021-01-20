package main

import (
	"flag"
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/robfig/cron/v3"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	_ "k8s.io/client-go/plugin/pkg/client/auth/gcp"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	"yun.netease.com/inspector/constant"
	"yun.netease.com/inspector/controllers/configmap"
	"yun.netease.com/inspector/controllers/service"
	"yun.netease.com/inspector/controllers/servicefence"
	"yun.netease.com/inspector/expire"
	"yun.netease.com/inspector/pkg/apis/microservice/v1alpha1"

	dependencyAdapter "yun.netease.com/inspector/adapter"
)

var (
	grpcServerPort string
	httpServerPort int

	scheme   = runtime.NewScheme()
	setupLog = ctrl.Log.WithName("setup")
	testLog  = ctrl.Log.WithName("test")

	controllerReconciler *servicefence.ServiceFenceReconciler
	resourceLock         *configmap.ConfigMapReconciler
	serviceReconciler    *service.ServiceReconciler
	defaultStrategy      string
	defaultStrategyTime  int64
	currentCluster       string
	CRDCrontabCycle      string
	redisCrontabCycle    string
)

func init() {
	//utilruntime.Must(clientgoscheme.AddToScheme(scheme))

	utilruntime.Must(v1alpha1.AddToScheme(scheme))

	utilruntime.Must(v1.AddToScheme(scheme))
	// +kubebuilder:scaffold:scheme
}

func loadController() {
	grpcServerPortPtr := flag.String("grpcServerPort", "50051", "set grpc server port,for example: '50051'")
	httpServerPortPtr := flag.Int("httpServerPort", 8080, "set http server port,for example: '8080'")

	grpcServerPort = *grpcServerPortPtr
	httpServerPort = *httpServerPortPtr

	var metricsAddr string
	var enableLeaderElection bool
	flag.StringVar(&metricsAddr, "metrics-addr", ":8090", "The address the metric endpoint binds to.")
	flag.BoolVar(&enableLeaderElection, "enable-leader-election", false,
		"Enable leader election for controller manager. "+
			"Enabling this will ensure there is only one active controller manager.")
	flag.StringVar(&defaultStrategy, "defaultStrategy", constant.Deadline, "default strategy")
	flag.Int64Var(&defaultStrategyTime, "defaultStrategyTime", 60*60*24, "default Strategy Time, (base, base*30) as deadline")
	flag.StringVar(&currentCluster, "currentCluster", "dev", "current cluster")
	//flag.StringVar(&ignores, "ignoreNamespace", "logagent", "ignore ns , split with ','")
	flag.StringVar(&CRDCrontabCycle, "crdCrontabCycle", "1 2 * * *", "indicate the managing CRD expire task execution cycle, once a day as default")
	//flag.StringVar(&redisCrontabCycle, "redisCrontabCycle", "1 * * * *", "indicate the writing redis task execution cycle, once an hour as default")
	//flag.StringVar(&coverNS, "coverNamespace", "", "CRD should cover ns , split with ','")
	//flag.StringVar(&sourceSVCHeader, "sourceSVCHeader", "x-yanxuan-app", "indicate the header which can obtain the source service")
	flag.Parse()

	ctrl.SetLogger(zap.New(zap.UseDevMode(true)))

	// controller manager
	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
		Scheme:             scheme,
		MetricsBindAddress: metricsAddr,
		Port:               9443,
		LeaderElection:     enableLeaderElection,
		LeaderElectionID:   "73ad1eff.netease.com",
	})
	if err != nil {
		setupLog.Error(err, "unable to start manager")
		os.Exit(1)
	}
	// servicefence controller
	sfcontroller := &servicefence.ServiceFenceReconciler{
		Client: mgr.GetClient(),
		Log:    ctrl.Log.WithName("serviceFence_controller"),
		Scheme: mgr.GetScheme(),
	}
	controllerReconciler = sfcontroller
	if err = sfcontroller.SetupWithManager(mgr, defaultStrategyTime); err != nil {
		setupLog.Error(err, "unable to create servicefence controller", "controller", "ServiceFence")
		os.Exit(1)
	}

	// configmap controller
	cmcontroller := &configmap.ConfigMapReconciler{
		Client:         mgr.GetClient(),
		Log:            ctrl.Log.WithName("configmap_controllers"),
		Scheme:         mgr.GetScheme(),
		LockCollection: make(map[string]*v1.ConfigMap),
	}
	resourceLock = cmcontroller
	if err := cmcontroller.SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create configmap controller", "controller", "ConfigMap")
		os.Exit(1)
	}

	// service controller
	svccontroller := &service.ServiceReconciler{
		Client:     mgr.GetClient(),
		Log:        ctrl.Log.WithName("service_controllers"),
		Scheme:     mgr.GetScheme(),
		Controller: sfcontroller,
	}
	serviceReconciler = svccontroller
	if err := svccontroller.SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create service controller", "controller", "Service")
		os.Exit(1)
	}

	// +kubebuilder:scaffold:builder

	// start controller manager
	go func() {
		setupLog.Info("starting manager")
		if err := mgr.Start(ctrl.SetupSignalHandler()); err != nil {
			setupLog.Error(err, "problem running manager")
			os.Exit(1)
		}
	}()
}

func main() {
	loadController()

	// 健康检查接口
	http.HandleFunc("/healthz", handleHealthz)

	http.HandleFunc("/test/update", testUpdate)
	http.HandleFunc("/test/get", testGet)
	http.HandleFunc("/test/list", testList)
	http.HandleFunc("/test/cache", testCache)
	http.HandleFunc("/test/namespacecache", testNsCache)

	go http.ListenAndServe(fmt.Sprintf(":%d", httpServerPort), nil)

	// 根据当前集群初始化redis
	//utils.RedisInit(currentCluster)

	// 启动时加载稳定性平台依赖关系, 写redis，并同时写CRD
	// utils.GetDataFromCaesar(controllerReconciler, defaultStrategy, defaultStrategyTime)

	s, err := dependencyAdapter.NewDependencyAdapter(
		grpcServerPort,
		controllerReconciler,
		serviceReconciler,
		defaultStrategy,
		defaultStrategyTime)
	if err != nil {
		fmt.Printf("unable to start server: %v", err)
		os.Exit(-1)
	}

	// 启动定时配置更新
	cm, err := expire.NewConfigManager(controllerReconciler, resourceLock)
	if err != nil {
		setupLog.Error(err, "ConfigManager create fail")
	}
	crontab := cron.New()
	if _, err := crontab.AddFunc(CRDCrontabCycle, cm.CRDCronUpdateCall); err != nil {
		setupLog.Error(err, "CRDCrontab add function fail!")
	}
	//if _, err := crontab.AddFunc(redisCrontabCycle, cm.RedisCronUpdateCall); err != nil {
	//	setupLog.Error(err, "redisCrontab add function fail!")
	//}
	go crontab.Start()

	// init empty servicefence in specified namespaces
	//serviceReconciler.InitServiceFence(coverNS)

	shutdown := make(chan error, 1)
	go func() {
		// 启动adapter
		s.Run(shutdown)
	}()

	_ = <-shutdown

}

func handleHealthz(w http.ResponseWriter, request *http.Request) {
	w.Write([]byte("server is ready"))
}

func testUpdate(w http.ResponseWriter, request *http.Request) {

	// curl "10.178.6.199:80/test/get?name=x&ns=y&ds=b.istio-system.svc.cluster.local"
	query := request.URL.Query()
	name := query["name"][0]
	ns := query["ns"][0]
	ds := query["ds"][0]

	sf := &v1alpha1.ServiceFence{}
	sf.Name = name
	sf.Namespace = ns
	sf.Spec.Host = make(map[string]*v1alpha1.RecyclingStrategy)
	sf.Spec.Host[ds] = &v1alpha1.RecyclingStrategy{
		Stable: nil,
		Deadline: &v1alpha1.RecyclingStrategy_Deadline{
			Expire: &v1alpha1.Timestamp{
				Seconds: time.Now().Unix() + 30*1000,
				Nanos:   0,
			},
		},
		Auto: nil,
		RecentlyCalled: &v1alpha1.Timestamp{
			Seconds: time.Now().Unix(),
			Nanos:   0,
		},
	}

	controllerReconciler.UpdateResource(sf)
	w.Write([]byte("update success"))

}

func testGet(w http.ResponseWriter, request *http.Request) {
	// curl "10.178.6.199:80/test/get?name=x&ns=y"
	query := request.URL.Query()
	name := query["name"][0]
	ns := query["ns"][0]

	controllerReconciler.GetResource(name, ns)
	w.Write([]byte("get success"))
}

func testList(w http.ResponseWriter, request *http.Request) {
	// curl "10.178.6.199:80/test/list?ns=dev"
	query := request.URL.Query()
	ns := query["ns"][0]
	res, err := controllerReconciler.ListResource(ns)
	if err != nil {
		testLog.Info("List ServiceFence Resource error", "error:  ", err)
	}
	for _, sf := range res.Items {
		testLog.Info("List ServiceFence Resource", "name", sf.Name, "namespace", sf.Namespace)
	}
	w.Write([]byte("list success"))
}

func testCache(w http.ResponseWriter, request *http.Request) {
	// curl "10.178.6.199:80/test/cache"
	mapCahe := dependencyAdapter.CacheDependency.Items()
	for key, val := range mapCahe {
		testLog.Info("cache", "key: ", key, "value: ", val)
	}
	w.Write([]byte("get success"))
}

func testNsCache(w http.ResponseWriter, request *http.Request) {
	// curl "10.178.6.199:80/test/namespacecache"
	mapCache := service.NamespacesCache
	for key, val := range mapCache {
		testLog.Info("NamespacesCache", "key", key, "value ", val)
	}
	w.Write([]byte("get success"))
}
