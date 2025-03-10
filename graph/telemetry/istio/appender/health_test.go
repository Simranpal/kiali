package appender

import (
	"fmt"
	"testing"
	"time"

	osproject_v1 "github.com/openshift/api/project/v1"
	"github.com/prometheus/common/model"
	"github.com/stretchr/testify/assert"
	apps_v1 "k8s.io/api/apps/v1"
	core_v1 "k8s.io/api/core/v1"
	meta_v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"

	"github.com/kiali/kiali/business"
	"github.com/kiali/kiali/config"
	"github.com/kiali/kiali/graph"
	"github.com/kiali/kiali/kubernetes"
	"github.com/kiali/kiali/kubernetes/cache"
	"github.com/kiali/kiali/kubernetes/kubetest"
	"github.com/kiali/kiali/models"
	"github.com/kiali/kiali/prometheus/prometheustest"
)

const (
	rateDefinition         = "400,10,20,http,inbound"
	rateWorkloadDefinition = "4xx,20,30,http,inbound"
)

func TestServicesHealthConfigPasses(t *testing.T) {
	config.Set(config.NewConfig())
	trafficMap := buildServiceTrafficMap()
	businessLayer := setupHealthConfig(t, buildFakeServicesHealth(rateDefinition), buildFakeWorkloadDeploymentsHealth(rateWorkloadDefinition), buildFakePodsHealth(rateWorkloadDefinition))

	globalInfo := graph.NewAppenderGlobalInfo()
	globalInfo.Business = businessLayer
	namespaceInfo := graph.NewAppenderNamespaceInfo("testNamespace")

	a := HealthAppender{}
	a.AppendGraph(trafficMap, globalInfo, namespaceInfo)

	for _, node := range trafficMap {
		srv, ok := node.Metadata[graph.HasHealthConfig].(map[string]string)
		assert.True(t, ok)
		assert.Equal(t, rateDefinition, srv[string(models.RateHealthAnnotation)])
	}
}

func TestServicesHealthNoConfigPasses(t *testing.T) {
	cfg := config.NewConfig()
	cfg.KubernetesConfig.CacheEnabled = false
	config.Set(cfg)
	trafficMap := buildServiceTrafficMap()
	businessLayer := setupHealthConfig(t, buildFakeServicesHealth(""), buildFakeWorkloadDeploymentsHealth(""), buildFakePodsHealth(""))

	globalInfo := graph.NewAppenderGlobalInfo()
	globalInfo.Business = businessLayer
	namespaceInfo := graph.NewAppenderNamespaceInfo("testNamespace")

	a := HealthAppender{}
	a.AppendGraph(trafficMap, globalInfo, namespaceInfo)

	for _, node := range trafficMap {
		srv, ok := node.Metadata[graph.HasHealthConfig].(map[string]string)
		assert.True(t, ok)
		assert.Equal(t, "", srv[string(models.RateHealthAnnotation)])
	}
}

func TestWorkloadHealthConfigPasses(t *testing.T) {
	cfg := config.NewConfig()
	cfg.KubernetesConfig.CacheEnabled = false
	config.Set(cfg)
	trafficMap := buildWorkloadTrafficMap()
	businessLayer := setupHealthConfig(t, buildFakeServicesHealth(rateDefinition), buildFakeWorkloadDeploymentsHealth(rateWorkloadDefinition), buildFakePodsHealth(rateWorkloadDefinition))

	globalInfo := graph.NewAppenderGlobalInfo()
	globalInfo.Business = businessLayer
	namespaceInfo := graph.NewAppenderNamespaceInfo("testNamespace")

	a := HealthAppender{}
	a.AppendGraph(trafficMap, globalInfo, namespaceInfo)

	for _, node := range trafficMap {
		srv, ok := node.Metadata[graph.HasHealthConfig].(map[string]string)
		assert.True(t, ok)
		assert.Equal(t, rateWorkloadDefinition, srv[string(models.RateHealthAnnotation)])
	}
}

func TestWorkloadHealthNoConfigPasses(t *testing.T) {
	cfg := config.NewConfig()
	cfg.KubernetesConfig.CacheEnabled = false
	config.Set(cfg)
	trafficMap := buildWorkloadTrafficMap()
	businessLayer := setupHealthConfig(t, buildFakeServicesHealth(""), buildFakeWorkloadDeploymentsHealth(""), buildFakePodsHealth(""))

	globalInfo := graph.NewAppenderGlobalInfo()
	globalInfo.Business = businessLayer
	namespaceInfo := graph.NewAppenderNamespaceInfo("testNamespace")

	a := HealthAppender{}
	a.AppendGraph(trafficMap, globalInfo, namespaceInfo)

	for _, node := range trafficMap {
		srv, ok := node.Metadata[graph.HasHealthConfig].(map[string]string)
		assert.True(t, ok)
		assert.Equal(t, "", srv[string(models.RateHealthAnnotation)])
	}
}

func TestHealthDataPresent(t *testing.T) {
	assert := assert.New(t)

	cfg := config.NewConfig()
	cfg.KubernetesConfig.CacheEnabled = false
	config.Set(cfg)
	svcNodes := buildServiceTrafficMap()
	appNodes := buildAppTrafficMap()
	wkNodes := buildWorkloadTrafficMap()
	trafficMap := make(graph.TrafficMap)
	for k, v := range svcNodes {
		trafficMap[k] = v
	}
	for k, v := range appNodes {
		trafficMap[k] = v
	}
	for k, v := range wkNodes {
		trafficMap[k] = v
	}
	businessLayer := setupHealthConfig(t, buildFakeServicesHealth(rateDefinition), buildFakeWorkloadDeploymentsHealth(rateWorkloadDefinition), buildFakePodsHealth(rateWorkloadDefinition))

	globalInfo := graph.NewAppenderGlobalInfo()
	globalInfo.Business = businessLayer
	namespaceInfo := graph.NewAppenderNamespaceInfo("testNamespace")

	a := HealthAppender{}
	a.AppendGraph(trafficMap, globalInfo, namespaceInfo)

	for _, node := range trafficMap {
		assert.Contains(node.Metadata, graph.HealthData)
	}
}

func TestHealthDataPresent200SvcWk(t *testing.T) {
	assert := assert.New(t)

	svcNodes := buildServiceTrafficMap()
	appNodes := buildAppTrafficMap()
	wkNodes := buildWorkloadTrafficMap()
	trafficMap := make(graph.TrafficMap)
	var (
		svc *graph.Node
		wk  *graph.Node
	)
	for k, v := range svcNodes {
		trafficMap[k] = v
		svc = v
	}
	for k, v := range appNodes {
		trafficMap[k] = v
	}
	for k, v := range wkNodes {
		trafficMap[k] = v
		wk = v
	}
	edge := svc.AddEdge(wk)
	/* Example of edge data:
	{
	 	"traffic": {
	 		"protocol": "http",
	 		"rates": {
	 			"http": "1.93",
	 			"httpPercentReq": "100.0"
	 		},
	 		"responses": {
	 			"200": {
	 				"flags": {
	 					"-": "100.0"
	 				},
	 				"hosts": {
	 					"v-server.beta.svc.cluster.local": "100.0"
	 				}
	 			}
	 		}
	 	}
	 }
	*/
	edge.Metadata[graph.ProtocolKey] = "http"
	edge.Metadata[graph.MetadataKey(graph.HTTP.EdgeResponses)] = graph.Responses{
		"200": &graph.ResponseDetail{
			Flags: graph.ResponseFlags{"-": 100.0},
			Hosts: map[string]float64{"v-server.beta.svc.cluster.local": 100.0},
		},
	}
	businessLayer := setupHealthConfig(t, buildFakeServicesHealth(rateDefinition), buildFakeWorkloadDeploymentsHealth(rateWorkloadDefinition), buildFakePodsHealth(rateWorkloadDefinition))

	globalInfo := graph.NewAppenderGlobalInfo()
	globalInfo.Business = businessLayer
	namespaceInfo := graph.NewAppenderNamespaceInfo("testNamespace")

	a := HealthAppender{}
	a.AppendGraph(trafficMap, globalInfo, namespaceInfo)

	for _, node := range trafficMap {
		assert.Contains(node.Metadata, graph.HealthData)
	}
	source := trafficMap[svc.ID]
	sourceHealth := source.Metadata[graph.HealthData].(*models.ServiceHealth)
	assert.Equal(sourceHealth.Requests.Outbound["http"]["200"], 100.0)

	dest := trafficMap[wk.ID]
	destHealth := dest.Metadata[graph.HealthData].(*models.WorkloadHealth)
	assert.Equal(destHealth.Requests.Inbound["http"]["200"], 100.0)
}

func TestHealthDataPresent200500WkSvc(t *testing.T) {
	assert := assert.New(t)

	config.Set(config.NewConfig())
	svcNodes := buildServiceTrafficMap()
	appNodes := buildAppTrafficMap()
	wkNodes := buildWorkloadTrafficMap()
	trafficMap := make(graph.TrafficMap)
	var (
		svc *graph.Node
		wk  *graph.Node
	)
	for k, v := range svcNodes {
		trafficMap[k] = v
		svc = v
	}
	for k, v := range appNodes {
		trafficMap[k] = v
	}
	for k, v := range wkNodes {
		trafficMap[k] = v
		wk = v
	}
	edge := wk.AddEdge(svc)
	edge.Metadata[graph.ProtocolKey] = "http"
	edge.Metadata[graph.MetadataKey(graph.HTTP.EdgeResponses)] = graph.Responses{
		"200": &graph.ResponseDetail{
			Flags: graph.ResponseFlags{"-": 100.0},
			Hosts: map[string]float64{"v-server.beta.svc.cluster.local": 100.0},
		},
		"500": &graph.ResponseDetail{
			Flags: graph.ResponseFlags{"-": 10.0},
			Hosts: map[string]float64{"v-server.beta.svc.cluster.local": 10.0},
		},
	}
	businessLayer := setupHealthConfig(t, buildFakeServicesHealth(rateDefinition), buildFakeWorkloadDeploymentsHealth(rateWorkloadDefinition), buildFakePodsHealth(rateWorkloadDefinition))

	globalInfo := graph.NewAppenderGlobalInfo()
	globalInfo.Business = businessLayer
	namespaceInfo := graph.NewAppenderNamespaceInfo("testNamespace")

	a := HealthAppender{}
	a.AppendGraph(trafficMap, globalInfo, namespaceInfo)

	for _, node := range trafficMap {
		assert.Contains(node.Metadata, graph.HealthData)
	}
	source := trafficMap[wk.ID]
	sourceHealth := source.Metadata[graph.HealthData].(*models.WorkloadHealth)
	assert.Equal(sourceHealth.Requests.Outbound["http"]["200"], 100.0)
	assert.Equal(sourceHealth.Requests.Outbound["http"]["500"], 10.0)

	dest := trafficMap[svc.ID]
	destHealth := dest.Metadata[graph.HealthData].(*models.ServiceHealth)
	assert.Equal(destHealth.Requests.Inbound["http"]["200"], 100.0)
	assert.Equal(destHealth.Requests.Inbound["http"]["500"], 10.0)
}

func TestHealthDataPresentToApp(t *testing.T) {
	assert := assert.New(t)

	config.Set(config.NewConfig())
	svcNodes := buildServiceTrafficMap()
	appNodes := buildAppTrafficMap()
	wkNodes := buildWorkloadTrafficMap()
	trafficMap := make(graph.TrafficMap)
	var (
		svc *graph.Node
		app *graph.Node
	)
	for k, v := range svcNodes {
		trafficMap[k] = v
		svc = v
	}
	for k, v := range appNodes {
		trafficMap[k] = v
		app = v
	}
	for k, v := range wkNodes {
		trafficMap[k] = v
	}
	edge := svc.AddEdge(app)
	edge.Metadata[graph.ProtocolKey] = "http"
	edge.Metadata[graph.MetadataKey(graph.HTTP.EdgeResponses)] = graph.Responses{
		"200": &graph.ResponseDetail{
			Flags: graph.ResponseFlags{"-": 100.0},
			Hosts: map[string]float64{"v-server.beta.svc.cluster.local": 100.0},
		},
	}
	businessLayer := setupHealthConfig(t, buildFakeServicesHealth(rateDefinition), buildFakeWorkloadDeploymentsHealth(rateWorkloadDefinition), buildFakePodsHealth(rateWorkloadDefinition))

	globalInfo := graph.NewAppenderGlobalInfo()
	globalInfo.Business = businessLayer
	namespaceInfo := graph.NewAppenderNamespaceInfo("testNamespace")

	a := HealthAppender{}
	a.AppendGraph(trafficMap, globalInfo, namespaceInfo)

	for _, node := range trafficMap {
		assert.Contains(node.Metadata, graph.HealthData)
	}
	source := trafficMap[svc.ID]
	sourceHealth := source.Metadata[graph.HealthData].(*models.ServiceHealth)
	assert.Equal(sourceHealth.Requests.Outbound["http"]["200"], 100.0)

	dest := trafficMap[app.ID]
	destHealth := dest.Metadata[graph.HealthData].(*models.AppHealth)
	assert.Equal(destHealth.Requests.Inbound["http"]["200"], 100.0)
}

func TestHealthDataPresentFromApp(t *testing.T) {
	assert := assert.New(t)

	config.Set(config.NewConfig())
	svcNodes := buildServiceTrafficMap()
	appNodes := buildAppTrafficMap()
	wkNodes := buildWorkloadTrafficMap()
	trafficMap := make(graph.TrafficMap)
	var (
		svc *graph.Node
		app *graph.Node
	)
	for k, v := range svcNodes {
		trafficMap[k] = v
		svc = v
	}
	for k, v := range appNodes {
		trafficMap[k] = v
		app = v
	}
	for k, v := range wkNodes {
		trafficMap[k] = v
		app.Workload = v.Workload
	}
	edge := app.AddEdge(svc)
	edge.Metadata[graph.ProtocolKey] = "http"
	edge.Metadata[graph.MetadataKey(graph.HTTP.EdgeResponses)] = graph.Responses{
		"200": &graph.ResponseDetail{
			Flags: graph.ResponseFlags{"-": 100.0},
			Hosts: map[string]float64{"v-server.beta.svc.cluster.local": 100.0},
		},
	}
	businessLayer := setupHealthConfig(t, buildFakeServicesHealth(rateDefinition), buildFakeWorkloadDeploymentsHealth(rateWorkloadDefinition), buildFakePodsHealth(rateWorkloadDefinition))

	globalInfo := graph.NewAppenderGlobalInfo()
	globalInfo.Business = businessLayer
	namespaceInfo := graph.NewAppenderNamespaceInfo("testNamespace")

	a := HealthAppender{}
	a.AppendGraph(trafficMap, globalInfo, namespaceInfo)

	for _, node := range trafficMap {
		assert.Contains(node.Metadata, graph.HealthData)
	}
	source := trafficMap[app.ID]
	sourceHealth := source.Metadata[graph.HealthData].(*models.AppHealth)
	assert.Equal(sourceHealth.Requests.Outbound["http"]["200"], 100.0)
	assert.Contains(source.Metadata, graph.HealthDataApp)
	sourceAppHealth := source.Metadata[graph.HealthDataApp].(*models.AppHealth)
	assert.Equal(sourceAppHealth.Requests.Outbound["http"]["200"], 100.0)

	dest := trafficMap[svc.ID]
	destHealth := dest.Metadata[graph.HealthData].(*models.ServiceHealth)
	assert.Equal(destHealth.Requests.Inbound["http"]["200"], 100.0)
}

func TestHealthDataBadResponses(t *testing.T) {
	assert := assert.New(t)

	config.Set(config.NewConfig())
	svcNodes := buildServiceTrafficMap()
	appNodes := buildAppTrafficMap()
	wkNodes := buildWorkloadTrafficMap()
	trafficMap := make(graph.TrafficMap)
	var (
		svc *graph.Node
		wk  *graph.Node
		app *graph.Node
	)
	for k, v := range svcNodes {
		trafficMap[k] = v
		svc = v
	}
	for k, v := range appNodes {
		trafficMap[k] = v
		app = v
	}
	for k, v := range wkNodes {
		trafficMap[k] = v
		wk = v
	}
	edge1 := app.AddEdge(svc)
	edge1.Metadata[graph.ProtocolKey] = "badprotocol"
	edge1.Metadata[graph.MetadataKey("badprotocol")] = graph.Responses{
		"200": &graph.ResponseDetail{
			Flags: graph.ResponseFlags{"-": 100.0},
			Hosts: map[string]float64{"v-server.beta.svc.cluster.local": 100.0},
		},
	}
	edge2 := wk.AddEdge(svc)
	edge2.Metadata[graph.ProtocolKey] = 20000
	businessLayer := setupHealthConfig(t, buildFakeServicesHealth(rateDefinition), buildFakeWorkloadDeploymentsHealth(rateWorkloadDefinition), buildFakePodsHealth(rateWorkloadDefinition))

	globalInfo := graph.NewAppenderGlobalInfo()
	globalInfo.Business = businessLayer
	namespaceInfo := graph.NewAppenderNamespaceInfo("testNamespace")

	a := HealthAppender{}
	a.AppendGraph(trafficMap, globalInfo, namespaceInfo)

	for _, node := range trafficMap {
		assert.Contains(node.Metadata, graph.HealthData)
	}
	source := trafficMap[app.ID]
	sourceHealth := source.Metadata[graph.HealthData].(*models.AppHealth)
	assert.Empty(sourceHealth.Requests.Outbound)

	dest := trafficMap[svc.ID]
	destHealth := dest.Metadata[graph.HealthData].(*models.ServiceHealth)
	assert.Empty(destHealth.Requests.Inbound)
}

func TestIdleNodesHaveHealthData(t *testing.T) {
	assert := assert.New(t)

	config.Set(config.NewConfig())
	trafficMap := make(graph.TrafficMap)
	idleNode, _ := graph.NewNode("cluster-default", "testNamespace", "svc", "", "", "", "v1", graph.GraphTypeVersionedApp)
	trafficMap[idleNode.ID] = idleNode
	idleNode.Metadata[graph.IsIdle] = true
	idleNode.Metadata[graph.IsInaccessible] = true
	businessLayer := setupHealthConfig(t, buildFakeServicesHealth(rateDefinition), buildFakeWorkloadDeploymentsHealth(rateWorkloadDefinition), buildFakePodsHealth(rateWorkloadDefinition))

	globalInfo := graph.NewAppenderGlobalInfo()
	globalInfo.Business = businessLayer
	namespaceInfo := graph.NewAppenderNamespaceInfo("testNamespace")

	a := HealthAppender{}
	a.AppendGraph(trafficMap, globalInfo, namespaceInfo)

	assert.NotNil(trafficMap[idleNode.ID].Metadata[graph.HealthData])
}

type servicesError struct {
	cache.KialiCache
	errorMsg string
}

func (s *servicesError) GetServices(namespace string, selectorLabels map[string]string) ([]core_v1.Service, error) {
	return nil, fmt.Errorf(s.errorMsg)
}

func (s *servicesError) GetKubeCache(cluster string) (cache.KubeCache, error) {
	return s, nil
}

func TestErrorCausesPanic(t *testing.T) {
	assert := assert.New(t)

	trafficMap := buildAppTrafficMap()
	objects := []runtime.Object{
		&osproject_v1.Project{ObjectMeta: meta_v1.ObjectMeta{Name: "testNamespace"}},
		&core_v1.Namespace{ObjectMeta: meta_v1.ObjectMeta{Name: "testNamespace"}},
	}
	for _, obj := range buildFakeWorkloadDeploymentsHealth(rateDefinition) {
		o := obj
		objects = append(objects, &o)
	}
	for _, obj := range buildFakePodsHealth(rateDefinition) {
		o := obj
		objects = append(objects, &o)
	}
	var k8s kubernetes.ClientInterface = kubetest.NewFakeK8sClient(objects...)

	conf := config.NewConfig()
	conf.ExternalServices.Istio.IstioAPIEnabled = false
	config.Set(conf)
	cache := business.NewTestingCache(t, k8s, *conf)
	const panicErrMsg = "test error! This should cause a panic"
	cache = &servicesError{cache, panicErrMsg}
	business.WithKialiCache(cache)

	business.SetKialiControlPlaneCluster(&business.Cluster{
		Name: business.DefaultClusterID,
	})

	prom := new(prometheustest.PromClientMock)
	prom.MockNamespaceServicesRequestRates("testNamespace", "0s", time.Unix(0, 0), model.Vector{})
	prom.MockAllRequestRates("testNamespace", conf.KubernetesConfig.ClusterName, "0s", time.Unix(0, 0), model.Vector{})
	k8sclients := make(map[string]kubernetes.ClientInterface)
	k8sclients[kubernetes.HomeClusterName] = k8s
	businessLayer := business.NewWithBackends(k8sclients, k8sclients, prom, nil)

	globalInfo := graph.NewAppenderGlobalInfo()
	globalInfo.Business = businessLayer
	namespaceInfo := graph.NewAppenderNamespaceInfo("testNamespace")

	a := HealthAppender{}

	assert.PanicsWithValue(panicErrMsg, func() { a.AppendGraph(trafficMap, globalInfo, namespaceInfo) })
}

func buildFakeServicesHealth(rate string) []core_v1.Service {
	annotationMap := map[string]string{}
	if rate != "" {
		annotationMap[string(models.RateHealthAnnotation)] = rate
	}
	return []core_v1.Service{
		{
			ObjectMeta: meta_v1.ObjectMeta{
				Name:        "svc",
				Namespace:   "testNamespace",
				Annotations: annotationMap,
			},
		},
	}
}

func buildFakeWorkloadDeploymentsHealth(rate string) []apps_v1.Deployment {
	apps := buildFakeWorkloadDeployments()
	if rate != "" {
		apps[0].ObjectMeta.Annotations = map[string]string{string(models.RateHealthAnnotation): rate}
	}
	return apps
}

func buildFakePodsHealth(rate string) []core_v1.Pod {
	pods := buildFakeWorkloadPods()
	if rate != "" {
		pods[0].ObjectMeta.Annotations[string(models.RateHealthAnnotation)] = rate
	}
	return pods
}

func setupHealthConfig(t *testing.T, services []core_v1.Service, deployments []apps_v1.Deployment, pods []core_v1.Pod) *business.Layer {
	objects := []runtime.Object{
		&osproject_v1.Project{ObjectMeta: meta_v1.ObjectMeta{Name: "testNamespace"}},
		&core_v1.Namespace{ObjectMeta: meta_v1.ObjectMeta{Name: "testNamespace"}},
	}
	for _, obj := range services {
		o := obj
		objects = append(objects, &o)
	}
	for _, obj := range deployments {
		o := obj
		objects = append(objects, &o)
	}
	for _, obj := range pods {
		o := obj
		objects = append(objects, &o)
	}
	k8s := kubetest.NewFakeK8sClient(objects...)

	conf := config.NewConfig()
	conf.ExternalServices.Istio.IstioAPIEnabled = false
	config.Set(conf)
	business.SetupBusinessLayer(t, k8s, *conf)
	business.SetKialiControlPlaneCluster(&business.Cluster{
		Name: business.DefaultClusterID,
	})

	prom := new(prometheustest.PromClientMock)
	prom.MockNamespaceServicesRequestRates("testNamespace", "0s", time.Unix(0, 0), model.Vector{})
	prom.MockAllRequestRates("testNamespace", kubernetes.HomeClusterName, "0s", time.Unix(0, 0), model.Vector{})
	k8sclients := make(map[string]kubernetes.ClientInterface)
	k8sclients[kubernetes.HomeClusterName] = k8s
	businessLayer := business.NewWithBackends(k8sclients, k8sclients, prom, nil)
	return businessLayer
}
