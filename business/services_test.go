package business

import (
	"context"
	"os"
	"sort"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	core_v1 "k8s.io/api/core/v1"
	meta_v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"

	"github.com/kiali/kiali/config"
	"github.com/kiali/kiali/kubernetes"
	"github.com/kiali/kiali/kubernetes/kubetest"
	"github.com/kiali/kiali/models"
	"github.com/kiali/kiali/prometheus"
	"github.com/kiali/kiali/prometheus/prometheustest"
)

func TestServiceListParsing(t *testing.T) {
	assert := assert.New(t)
	require := require.New(t)

	// Setup mocks
	s1 := kubetest.FakeService("Namespace", "reviews")
	s2 := kubetest.FakeService("Namespace", "httpbin")
	objects := []runtime.Object{
		&core_v1.Namespace{ObjectMeta: meta_v1.ObjectMeta{Name: "Namespace"}},
		&s1,
		&s2,
	}
	conf := config.NewConfig()
	config.Set(conf)
	k8s := kubetest.NewFakeK8sClient(objects...)
	setupGlobalMeshConfig()
	SetupBusinessLayer(t, k8s, *conf)
	k8sclients := make(map[string]kubernetes.ClientInterface)
	k8sclients[conf.KubernetesConfig.ClusterName] = k8s
	svc := NewWithBackends(k8sclients, k8sclients, nil, nil).Svc

	criteria := ServiceCriteria{Namespace: "Namespace", IncludeIstioResources: false, IncludeHealth: false}
	serviceList, err := svc.GetServiceList(context.TODO(), criteria)
	require.NoError(err)

	require.Equal("Namespace", serviceList.Namespace.Name)
	require.Len(serviceList.Services, 2)
	serviceNames := []string{serviceList.Services[0].Name, serviceList.Services[1].Name}

	assert.Contains(serviceNames, "reviews")
	assert.Contains(serviceNames, "httpbin")
}

func TestParseRegistryServices(t *testing.T) {
	assert := assert.New(t)

	conf := config.NewConfig()
	config.Set(conf)

	k8s := new(kubetest.K8SClientMock)
	k8s.On("IsOpenShift").Return(false)
	k8s.On("IsGatewayAPI").Return(false)
	setupGlobalMeshConfig()
	k8sclients := make(map[string]kubernetes.ClientInterface)
	k8sclients[kubernetes.HomeClusterName] = k8s
	svc := NewWithBackends(k8sclients, k8sclients, nil, nil).Svc

	servicesz := "../tests/data/registry/services-registryz.json"
	bServicesz, err := os.ReadFile(servicesz)
	assert.NoError(err)
	rServices := map[string][]byte{
		"istiod1": bServicesz,
	}
	registryServices, err2 := kubernetes.ParseRegistryServices(rServices)
	assert.NoError(err2)

	assert.Equal(3, len(registryServices))

	configz := "../tests/data/registry/services-configz.json"
	bConfigz, err2 := os.ReadFile(configz)
	assert.NoError(err2)
	rConfig := map[string][]byte{
		"istiod1": bConfigz,
	}
	registryConfig, err2 := kubernetes.ParseRegistryConfig(rConfig)
	assert.NoError(err2)

	assert.Equal(2, len(registryConfig.ServiceEntries))

	istioConfigList := models.IstioConfigList{
		ServiceEntries: registryConfig.ServiceEntries,
	}

	parsedServices := svc.buildRegistryServices(registryServices, istioConfigList)
	assert.Equal(3, len(parsedServices))
	assert.Equal(1, len(parsedServices[0].IstioReferences))
	assert.Equal(1, len(parsedServices[1].IstioReferences))
	assert.Equal(0, len(parsedServices[2].IstioReferences))
}

func TestFilterLocalIstioRegistry(t *testing.T) {
	assert := assert.New(t)

	conf := config.NewConfig()
	config.Set(conf)

	servicesz := "../tests/data/registry/istio-east-registryz.json"
	bServicesz, err := os.ReadFile(servicesz)
	assert.NoError(err)
	rServices := map[string][]byte{
		"istiod1": bServicesz,
	}
	registryServices, err2 := kubernetes.ParseRegistryServices(rServices)
	assert.NoError(err2)

	assert.Equal(true, filterIstioServiceByClusterId("istio-east", registryServices[0]))
	assert.Equal(false, filterIstioServiceByClusterId("istio-east", registryServices[1]))
}

func TestGetServiceListFromMultipleClusters(t *testing.T) {
	assert := assert.New(t)
	require := require.New(t)

	conf := config.NewConfig()
	conf.ExternalServices.Istio.IstioAPIEnabled = false
	config.Set(conf)

	clientFactory := kubetest.NewK8SClientFactoryMock(nil)
	clients := map[string]kubernetes.ClientInterface{
		kubernetes.HomeClusterName: kubetest.NewFakeK8sClient(
			&core_v1.Namespace{ObjectMeta: meta_v1.ObjectMeta{Name: "bookinfo"}},
			&core_v1.Service{ObjectMeta: meta_v1.ObjectMeta{Name: "ratings-home-cluster", Namespace: "bookinfo"}},
		),
		"west": kubetest.NewFakeK8sClient(
			&core_v1.Namespace{ObjectMeta: meta_v1.ObjectMeta{Name: "bookinfo"}},
			&core_v1.Service{ObjectMeta: meta_v1.ObjectMeta{Name: "ratings-west-cluster", Namespace: "bookinfo"}},
		),
	}
	clientFactory.SetClients(clients)
	cache := newTestingCache(t, clientFactory, *conf)
	kialiCache = cache

	svc := NewWithBackends(clients, clients, nil, nil).Svc
	svcs, err := svc.GetServiceList(context.TODO(), ServiceCriteria{Namespace: "bookinfo"})
	require.NoError(err)
	require.Len(svcs.Services, 2)

	sort.Slice(svcs.Services, func(i, j int) bool {
		return svcs.Services[i].Name < svcs.Services[j].Name
	})
	assert.Equal(svcs.Services[0].Cluster, kubernetes.HomeClusterName)
	assert.Equal(svcs.Services[1].Cluster, "west")
}

func TestMultiClusterGetService(t *testing.T) {
	assert := assert.New(t)
	require := require.New(t)

	conf := config.NewConfig()
	conf.ExternalServices.Istio.IstioAPIEnabled = false
	config.Set(conf)

	clientFactory := kubetest.NewK8SClientFactoryMock(nil)
	clients := map[string]kubernetes.ClientInterface{
		kubernetes.HomeClusterName: kubetest.NewFakeK8sClient(
			&core_v1.Namespace{ObjectMeta: meta_v1.ObjectMeta{Name: "bookinfo"}},
			&core_v1.Service{ObjectMeta: meta_v1.ObjectMeta{Name: "ratings-home-cluster", Namespace: "bookinfo"}},
		),
		"west": kubetest.NewFakeK8sClient(
			&core_v1.Namespace{ObjectMeta: meta_v1.ObjectMeta{Name: "bookinfo"}},
			&core_v1.Service{ObjectMeta: meta_v1.ObjectMeta{Name: "ratings-west-cluster", Namespace: "bookinfo"}},
		),
	}
	clientFactory.SetClients(clients)
	cache := newTestingCache(t, clientFactory, *conf)
	kialiCache = cache

	svc := NewWithBackends(clients, clients, nil, nil).Svc
	s, err := svc.GetService(context.TODO(), "west", "bookinfo", "ratings-west-cluster")
	require.NoError(err)

	assert.Equal(s.Name, "ratings-west-cluster")

	s, err = svc.GetService(context.TODO(), kubernetes.HomeClusterName, "bookinfo", "ratings-home-cluster")
	require.NoError(err)

	assert.Equal(s.Name, "ratings-home-cluster")
}

func TestMultiClusterServiceUpdate(t *testing.T) {
	assert := assert.New(t)
	require := require.New(t)

	conf := config.NewConfig()
	conf.ExternalServices.Istio.IstioAPIEnabled = false
	config.Set(conf)

	clientFactory := kubetest.NewK8SClientFactoryMock(nil)
	clients := map[string]kubernetes.ClientInterface{
		kubernetes.HomeClusterName: kubetest.NewFakeK8sClient(
			&core_v1.Namespace{ObjectMeta: meta_v1.ObjectMeta{Name: "bookinfo"}},
			&core_v1.Service{ObjectMeta: meta_v1.ObjectMeta{Name: "ratings-home-cluster", Namespace: "bookinfo"}},
		),
		"west": kubetest.NewFakeK8sClient(
			&core_v1.Namespace{ObjectMeta: meta_v1.ObjectMeta{Name: "bookinfo"}},
			&core_v1.Service{ObjectMeta: meta_v1.ObjectMeta{Name: "ratings-west-cluster", Namespace: "bookinfo"}},
		),
	}
	clientFactory.SetClients(clients)
	cache := newTestingCache(t, clientFactory, *conf)
	kialiCache = cache

	prom, err := prometheus.NewClient()
	require.NoError(err)

	promMock := new(prometheustest.PromAPIMock)
	promMock.SpyArgumentsAndReturnEmpty(func(mock.Arguments) {})
	prom.Inject(promMock)
	svc := NewWithBackends(clients, clients, prom, nil).Svc
	_, err = svc.UpdateService(context.TODO(), "west", "bookinfo", "ratings-west-cluster", "60s", time.Now(), `{"metadata":{"annotations":{"test":"newlabel"}}}`, "merge")
	require.NoError(err)

	s, err := svc.GetService(context.TODO(), "west", "bookinfo", "ratings-west-cluster")
	require.NoError(err)
	assert.Contains(s.Annotations, "test")

	s, err = svc.GetService(context.TODO(), kubernetes.HomeClusterName, "bookinfo", "ratings-home-cluster")
	require.NoError(err)
	assert.NotContains(s.Annotations, "test")
}

func TestMultiClusterGetServiceDetails(t *testing.T) {
	assert := assert.New(t)
	require := require.New(t)

	conf := config.NewConfig()
	conf.ExternalServices.Istio.IstioAPIEnabled = false
	config.Set(conf)

	clientFactory := kubetest.NewK8SClientFactoryMock(nil)
	clients := map[string]kubernetes.ClientInterface{
		kubernetes.HomeClusterName: kubetest.NewFakeK8sClient(
			&core_v1.Namespace{ObjectMeta: meta_v1.ObjectMeta{Name: "bookinfo"}},
			&core_v1.Service{ObjectMeta: meta_v1.ObjectMeta{Name: "ratings-home-cluster", Namespace: "bookinfo"}},
		),
		"west": kubetest.NewFakeK8sClient(
			&core_v1.Namespace{ObjectMeta: meta_v1.ObjectMeta{Name: "bookinfo"}},
			&core_v1.Service{ObjectMeta: meta_v1.ObjectMeta{Name: "ratings-west-cluster", Namespace: "bookinfo"}},
		),
	}
	clientFactory.SetClients(clients)
	cache := newTestingCache(t, clientFactory, *conf)
	kialiCache = cache

	prom, err := prometheus.NewClient()
	require.NoError(err)

	promMock := new(prometheustest.PromAPIMock)
	promMock.SpyArgumentsAndReturnEmpty(func(mock.Arguments) {})
	prom.Inject(promMock)
	svc := NewWithBackends(clients, clients, prom, nil).Svc
	s, err := svc.GetServiceDetails(context.TODO(), "west", "bookinfo", "ratings-west-cluster", "60s", time.Now())
	require.NoError(err)

	assert.Equal(s.Service.Name, "ratings-west-cluster")
}

func TestMultiClusterGetServiceAppName(t *testing.T) {
	assert := assert.New(t)
	require := require.New(t)

	conf := config.NewConfig()
	config.Set(conf)

	clientFactory := kubetest.NewK8SClientFactoryMock(nil)
	clients := map[string]kubernetes.ClientInterface{
		kubernetes.HomeClusterName: kubetest.NewFakeK8sClient(
			&core_v1.Namespace{ObjectMeta: meta_v1.ObjectMeta{Name: "bookinfo"}},
			&core_v1.Service{ObjectMeta: meta_v1.ObjectMeta{Name: "ratings-home-cluster", Namespace: "bookinfo"}},
		),
		"west": kubetest.NewFakeK8sClient(
			&core_v1.Namespace{ObjectMeta: meta_v1.ObjectMeta{Name: "bookinfo"}},
			&core_v1.Service{
				ObjectMeta: meta_v1.ObjectMeta{
					Name:      "ratings-west-cluster",
					Namespace: "bookinfo",
				},
				Spec: core_v1.ServiceSpec{
					Selector: map[string]string{
						"app": "ratings",
					},
				},
			},
		),
	}
	clientFactory.SetClients(clients)
	cache := newTestingCache(t, clientFactory, *conf)
	kialiCache = cache

	svc := NewWithBackends(clients, clients, nil, nil).Svc
	s, err := svc.GetServiceAppName(context.TODO(), "west", "bookinfo", "ratings-west-cluster")
	require.NoError(err)

	assert.Equal("ratings", s)
}
