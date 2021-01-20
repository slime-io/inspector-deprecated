package constant

const (
	HOST_SUFFIX                    = ".svc.cluster.local"
	Redis_Lock                     = "inspector_ListServiceFence"
	Stable                         = "Stable"   // 永久存在
	Deadline                       = "Deadline" // 到期失效
	Auto                           = "Auto"     // 多久未访问失效
	TELE_UNKNOWN                   = "unknown"
	CronUpdateLockName             = "servicefence-cronupdate-lock" // crontab lock name
	LoadFromStablePlatformLockName = "load-stableplatform-lock"     // load-stableplatform-lock name
	LockNameSpace                  = "kube-system"
	DependencyHotLoading           = "servicefence-dependency-hotloading"
	CacheKeySplit                  = "#" // memory cache separator
	AllNamespaces                  = "all_namespaces"
	NSOpenLabelKey                 = "inspector-servicefence" // indicate namespace open the servicefence
	NSOpenLabelVal                 = "open"
	SVCOpenAnnotationsKey          = "istio.dependency.servicefence/status"

	Protocol = "protocol"
	// GlobalSidecar的标签
	GlobalSidecarLabel = "qz-ingress"
	// 上报者为GlobalSidecar时通过如下attribute获取调用方/被调用方
	GlobalSidecarSource      = "global_sidecar_source"
	GlobalSidecarDestination = "global_sidecar_destination"
	GlobalSidecarSourceIp    = "global_sidecar_source_ip"
	// 上报者为Sidecar时通过如下attribute获取调用方/被调用方
	SidecarSource               = "sidecar_source"
	SidecarSourceNamespace      = "sidecar_source_namespace"
	SidecarDestination          = "sidecar_destination"
	SidecarDestinationNamespace = "sidecar_destination_namespace"
	SidecarDestinationIp        = "sidecar_destination_ip"
)
