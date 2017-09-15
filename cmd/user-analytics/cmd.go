package main

import (
	"flag"
	"log"
	"os"

	"github.com/openshift/online-analytics/pkg/useranalytics"

	osclient "github.com/openshift/origin/pkg/client"
	"github.com/openshift/origin/pkg/cmd/util/clientcmd"

	restclient "k8s.io/client-go/rest"
	kclientcmd "k8s.io/client-go/tools/clientcmd"
	kclientset "k8s.io/kubernetes/pkg/client/clientset_generated/internalclientset"

	glog "github.com/golang/glog"
	"github.com/spf13/pflag"
)

func main() {
	var useServiceAccounts bool
	var clusterName string
	var maximumQueueLength, metricsPollingFrequency int
	var woopraEndpoint, woopraDomain string
	var woopraEnabled, localEndpointEnabled bool
	var userKeyStrategy, userKeyAnnotation string
	var metricsBindAddr string
	var collectRuntime, collectWoopra, collectQueue bool

	flag.BoolVar(&useServiceAccounts, "useServiceAccounts", false, "Connect to OpenShift using a service account")
	flag.StringVar(&clusterName, "clusterName", "kubernetes", "Cluster name")
	flag.IntVar(&maximumQueueLength, "maximumQueueLength", 1000000, "The maximum number of analytic event items that are internally queued for forwarding")
	flag.StringVar(&metricsBindAddr, "metricsBindAddr", ":8080", "The address on localhost serving metrics - http://localhost:port/metrics")
	flag.IntVar(&metricsPollingFrequency, "metricsPollingFrequency", 10, "The number of seconds between metrics snapshots.")
	flag.BoolVar(&localEndpointEnabled, "localEndpointEnabled", false, "Use a local HTTP endpoint for analytics. Useful for test and dev environments")

	flag.StringVar(&woopraEndpoint, "woopraEndpoint", "http://www.example.com", "The URL to send data to")
	flag.StringVar(&woopraDomain, "woopraDomain", "openshift", "The domain to collect data under")
	flag.BoolVar(&woopraEnabled, "woopraEnabled", true, "Enable/disable sending data to Woopra")

	flag.StringVar(&userKeyStrategy, "userKeyStrategy", useranalytics.KeyStrategyUID, "Strategy used to key users in Woopra. Options are [annotation|name|uid]")
	flag.StringVar(&userKeyAnnotation, "userKeyAnnotation", useranalytics.OnlineManagedID, "User annotation to use if userKeyStrategy=annotation")

	flag.BoolVar(&collectRuntime, "collectRuntime", true, "Enable runtime metrics")
	flag.BoolVar(&collectWoopra, "collectWoopra", true, "Enable woopra metrics")
	flag.BoolVar(&collectQueue, "collectQueue", true, "Enable queue metrics")
	flag.Parse()

	_, _, openshiftClient, kubeClient, err := createClients()

	if !validateKeyStrategy(userKeyStrategy) {
		log.Fatalf("Must set a valid userKeyStrategy.")
	} else if userKeyStrategy == "annotation" {
		if userKeyAnnotation == "" {
			log.Fatalf("Must set a userKeyAnnotation when using userKeyStrategy=annotation.")
		}
	}
	config := &useranalytics.AnalyticsControllerConfig{
		Destinations:            make(map[string]useranalytics.Destination),
		KubeClient:              kubeClient,
		OSClient:                openshiftClient,
		MaximumQueueLength:      maximumQueueLength,
		MetricsPollingFrequency: metricsPollingFrequency,
		ClusterName:             clusterName,
		UserKeyStrategy:         userKeyStrategy,
		UserKeyAnnotation:       userKeyAnnotation,
	}

	if woopraEnabled {
		config.Destinations["woopra"] = &useranalytics.WoopraDestination{
			Method:   "GET",
			Domain:   woopraDomain,
			Endpoint: woopraEndpoint,
			Client:   useranalytics.NewSimpleHttpClient(),
		}
	}

	if localEndpointEnabled {
		config.Destinations["local"] = &useranalytics.WoopraDestination{
			Method:   "GET",
			Domain:   "local",
			Endpoint: "http://127.0.0.1:8888/dest",
			Client:   useranalytics.NewSimpleHttpClient(),
		}
	}

	if len(config.Destinations) == 0 {
		glog.V(0).Infof("No analytics destinations configured.  Analytics controller will not be started.")
		os.Exit(5)
	}

	controller, err := useranalytics.NewAnalyticsController(config)
	if err != nil {
		glog.Errorf("Error creating controller: %v", err)
		os.Exit(6)
	}

	go func() {
		metricsConfig := useranalytics.MetricsConfig{
			BindAddr:       metricsBindAddr,
			CollectRuntime: collectRuntime,
			CollectWoopra:  collectWoopra,
			CollectQueue:   collectQueue,
		}
		server := &useranalytics.MetricsServer{
			Config:     metricsConfig,
			Controller: controller,
		}

		if woopraEnabled {
			server.WoopraClient = config.Destinations["woopra"].(*useranalytics.WoopraDestination)
		}
		err := server.Serve()
		if err != nil {
			glog.Errorf("Error running metrics server: %s", err)
		}
	}()

	c := make(chan struct{})

	if localEndpointEnabled {
		mockEndpoint := useranalytics.MockHttpEndpoint{
			Port:       8888,
			URLPrefix:  "/dest",
			MaxLatency: 0,
			FlakeRate:  0,
			DupeCheck:  false,
		}
		mockEndpoint.Run(c)
	}

	controller.Run(c, 3)
	<-c

}

func createClients() (*restclient.Config, *clientcmd.Factory, osclient.Interface, kclientset.Interface, error) {
	dcc := clientcmd.DefaultClientConfig(pflag.NewFlagSet("empty", pflag.ContinueOnError))
	return CreateClientsForConfig(dcc)
}

// CreateClientsForConfig creates and returns OpenShift and Kubernetes clients (as well as other useful
// client objects) for the given client config.
// TODO: stop returning internalversion kclientset
func CreateClientsForConfig(dcc kclientcmd.ClientConfig) (*restclient.Config, *clientcmd.Factory, osclient.Interface, kclientset.Interface, error) {

	_, err := dcc.RawConfig()

	clientFac := clientcmd.NewFactory(dcc)

	clientConfig, err := dcc.ClientConfig()
	if err != nil {
		log.Panicf("error creating cluster clientConfig: %s", err)
	}

	oc, kc, err := clientFac.Clients()
	return clientConfig, clientFac, oc, kc, err
}

func validateKeyStrategy(strategy string) bool {
	strategies := map[string]bool{
		useranalytics.KeyStrategyAnnotation: true,
		useranalytics.KeyStrategyName:       true,
		useranalytics.KeyStrategyUID:        true,
	}
	if _, exists := strategies[strategy]; exists {
		return true
	}
	return false
}
