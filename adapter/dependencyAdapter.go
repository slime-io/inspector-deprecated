// nolint:lll
// Generates the mygrpcadapter adapter's resource yaml. It contains the adapter's configuration, name, supported template
// names (metric in this case), and whether it is session or no-session based.
//go:generate $REPO_ROOT/bin/mixer_codegen.sh -a mixer/adapter/inspector/config/config.proto -x "-s=false -n dependencyadapter -t metric"

package adapter

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"io/ioutil"
	"net"
	"os"
	"strings"

	"github.com/patrickmn/go-cache"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"istio.io/api/mixer/adapter/model/v1beta1"
	policy "istio.io/api/policy/v1beta1"
	"istio.io/istio/mixer/template/metric"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/rand"
	ctrl "sigs.k8s.io/controller-runtime"

	con "yun.netease.com/inspector/constant"
	"yun.netease.com/inspector/controllers/service"
	"yun.netease.com/inspector/controllers/servicefence"
)

type (
	Server interface {
		Addr() string
		Close() error
		Run(shutdown chan error)
	}

	DependencyAdapter struct {
		listener      net.Listener
		server        *grpc.Server
		controller    *servicefence.ServiceFenceReconciler
		svccontroller *service.ServiceReconciler
	}
)

var (
	_               metric.HandleMetricServiceServer = &DependencyAdapter{}
	strategy_time   int64
	strategy        string
	CacheDependency *cache.Cache
	log             = ctrl.Log.WithName("DependencyAdapter")
)

// HandleMetric records metric entries
func (s *DependencyAdapter) HandleMetric(ctx context.Context, r *metric.HandleMetricRequest) (*v1beta1.ReportResult, error) {

	//log.Infof("received request %v\n", *r)
	instances(s, r.Instances)
	return &v1beta1.ReportResult{}, nil
}

// handle data from Mixer
func instances(s *DependencyAdapter, in []*metric.InstanceMsg) {
	for _, inst := range in {

		src, dest := getSourceAndDestination(inst.Dimensions)

		for k, v := range inst.Dimensions {
			log.Info(fmt.Sprintf("info accept : key: %s, value: %s", k, v))
		}

		log.Info(fmt.Sprintf("\n"+
			" qualified data: "+
			"  {\n"+
			"		source_service = %v\n"+
			"		request_host_destination = %v\n"+
			"  }",
			src, dest))

		if len(strings.Split(dest, ".")) == 1 {
			if validate(src) {
				ns := strings.Split(src, ".")[1]
				dest = dest + "." + ns
			}
		}
		if !validate(src) || !validate(dest) {
			continue
		}

		source := strings.Split(src, ".")
		srcYxApp := source[0]
		srcNs := source[1]

		// indicate the namespaces to process

		if !service.NamespacesCache[srcNs] {
			log.Info("source_service_namespace is not indicated to cover , continue ")
			continue
		}

		str := strings.SplitN(dest, ".", 3)
		svcName := str[0]
		svcNs := str[1]

		svc := &v1.Service{}
		if err := s.svccontroller.Get(context.TODO(), types.NamespacedName{svcNs, svcName}, svc); err != nil {
			log.Info("service isn't in current cluster，should be routed to egress  , continue ")
			continue
		}

		if service.ToEgressCache[stringFormat(svcName, svcNs, con.CacheKeySplit)] {
			log.Info("service doesn't have endpoint，should be routed to egress  , continue ")
			continue
		}

		// writing to crd
		log.Info("adapter recieve data, writing to CRD", "name", srcYxApp)
		random_strategy_time := rand.Int63nRange(strategy_time, strategy_time*30)
		sf := servicefence.ConstructServiceFence(srcYxApp, srcNs, svcName, svcNs, strategy, random_strategy_time, false)
		s.controller.UpdateResource(sf)

		//desbuild := stringFormat(svcName, svcNs, ".")
		//cachebuild := stringFormat(sourceSvc, desbuild, con.CacheKeySplit)
		//
		//// cache:  key, val = source#dest, time
		//item := &cache.Item{
		//	Object:     time.Now().Unix(),
		//	Expiration: -1,
		//}

		//if err := CacheDependency.Add(cachebuild, item, -1); err != nil {
		//	// key exist，update value
		//	CacheDependency.Replace(cachebuild, item, -1)
		//	continue
		//} else {
		//	// key doesn't  exist
		//	log.Info("the key doesn't exist in cache, writing to CRD")
		//	sf := servicefence.ConstructServiceFence(srcYxApp, srcNs, svcName, svcNs, strategy, strategy_time, false)
		//	s.controller.UpdateResource(sf)
		//	// update visit time
		//	//go utils.HSET(sourceSvc, desbuild, time.Now().Unix())
		//}

		//if _, flag := utils.HGET(sourceSvc, desbuild.String()); !flag {
		//	// redis不存在key-field时,写CRD
		//	sf := servicefence.ConstructServiceFence(srcYxApp, srcNs, svcName, svcNs, strategy, strategy_time)
		//	s.controller.UpdateResource(sf)
		//}
		//// update visit time
		//go utils.HSET(sourceSvc, desbuild.String(), time.Now().Unix())

	}
}

// Addr returns the listening address of the server
func (s *DependencyAdapter) Addr() string {
	return s.listener.Addr().String()
}

// Run starts the server run
func (s *DependencyAdapter) Run(shutdown chan error) {
	log.Info("grpc server start!!!")
	shutdown <- s.server.Serve(s.listener)
}

// Close gracefully shuts down the server; used for testing
func (s *DependencyAdapter) Close() error {
	if s.server != nil {
		s.server.GracefulStop()
	}

	if s.listener != nil {
		_ = s.listener.Close()
	}

	return nil
}

func getServerTLSOption(credential, privateKey, caCertificate string) (grpc.ServerOption, error) {
	certificate, err := tls.LoadX509KeyPair(
		credential,
		privateKey,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to load key cert pair")
	}
	certPool := x509.NewCertPool()
	bs, err := ioutil.ReadFile(caCertificate)
	if err != nil {
		return nil, fmt.Errorf("failed to read client ca cert: %s", err)
	}

	ok := certPool.AppendCertsFromPEM(bs)
	if !ok {
		return nil, fmt.Errorf("failed to append client certs")
	}

	tlsConfig := &tls.Config{
		Certificates: []tls.Certificate{certificate},
		ClientCAs:    certPool,
	}
	tlsConfig.ClientAuth = tls.RequireAndVerifyClientCert

	return grpc.Creds(credentials.NewTLS(tlsConfig)), nil
}

// DependencyAdapter creates a new IBP adapter that listens at provided port.
func NewDependencyAdapter(
	addr string,
	reconciler *servicefence.ServiceFenceReconciler,
	serviceReconciler *service.ServiceReconciler,
	defaultStrategy string,
	defaultStrategyTime int64) (Server, error) {

	strategy = defaultStrategy
	strategy_time = defaultStrategyTime

	listener, err := net.Listen("tcp", fmt.Sprintf(":%s", addr))
	if err != nil {
		return nil, fmt.Errorf("unable to listen on socket: %v", err)
	}
	s := &DependencyAdapter{
		listener: listener,
	}
	fmt.Printf("listening on \"%v\"\n", s.Addr())

	credential := os.Getenv("GRPC_ADAPTER_CREDENTIAL")
	privateKey := os.Getenv("GRPC_ADAPTER_PRIVATE_KEY")
	certificate := os.Getenv("GRPC_ADAPTER_CERTIFICATE")
	if credential != "" {
		so, err := getServerTLSOption(credential, privateKey, certificate)
		if err != nil {
			return nil, err
		}
		s.server = grpc.NewServer(so)
		s.controller = reconciler
		s.svccontroller = serviceReconciler
	} else {
		s.server = grpc.NewServer()
		s.controller = reconciler
		s.svccontroller = serviceReconciler
	}

	cacheMap := make(map[string]cache.Item, 500)
	CacheDependency = cache.NewFrom(cache.DefaultExpiration, cache.DefaultExpiration, cacheMap)

	metric.RegisterHandleMetricServiceServer(s.server, s)
	return s, nil
}

func stringFormat(name string, ns string, con string) string {

	var build strings.Builder
	build.WriteString(name)
	build.WriteString(con)
	build.WriteString(ns)

	return build.String()
}

func getSourceAndDestination(dimensions map[string]*policy.Value) (source, destination string) {
	protocol := dimensions[con.Protocol].GetStringValue()
	if protocol == "http" {
		// Get sourceIp from x-forwarder-for
		ip := dimensions[con.GlobalSidecarSourceIp].GetStringValue()
		if d, ok := service.IP2Host.Get(ip); !ok {
			source = "unknown.unknown"
		} else {
			source = d.(string)
		}
		destination = dimensions[con.GlobalSidecarDestination].GetStringValue()
		destination = strings.Split(destination, ":")[0]
	} else {
		source = dimensions[con.SidecarSource].GetStringValue() + "." + dimensions[con.SidecarSourceNamespace].GetStringValue()
		destinationName := dimensions[con.SidecarDestination].GetStringValue()
		destinationNamespace := dimensions[con.SidecarDestinationNamespace].GetStringValue()
		if destinationName != "unknown" {
			destination = destinationName + "." + destinationNamespace
		} else {
			ip := convertToIp(dimensions[con.SidecarDestinationIp].GetIpAddressValue().GetValue())
			if d, ok := service.IP2Host.Get(ip); !ok {
				destination = "unknown.unknown"
			} else {
				destination = d.(string)
			}
		}
	}
	return source, destination
}

func validate(svc string) bool {
	ss := strings.Split(svc, ".")
	if len(ss) != 2 {
		fmt.Println("invalid svc : " + svc)
		return false
	} else {
		if ss[1] == "" {
			fmt.Println("invalid svc : " + svc)
			return false
		}
		if ss[0] == "unknown" {
			fmt.Println("unknown service")
			return false
		}
	}
	return true
}

func convertToIp(d []byte) string {
	ips := make([]int, 4)
	if len(d) != 4 {
		return ""
	} else {
		for index, i := range d {
			ips[index] = int(i)
		}
	}
	return fmt.Sprintf("%d.%d.%d.%d", ips[0], ips[1], ips[2], ips[3])
}
