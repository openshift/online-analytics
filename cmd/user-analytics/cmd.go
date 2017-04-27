package main

import (
	"flag"
	"log"
	"os"

	"github.com/openshift/online/user-analytics/pkg/useranalytics"

	osclient "github.com/openshift/origin/pkg/client"
	"github.com/openshift/origin/pkg/cmd/util/clientcmd"
	"k8s.io/kubernetes/pkg/client/restclient"
	kclient "k8s.io/kubernetes/pkg/client/unversioned"

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

	var kubeClient kclient.Interface
	var openshiftClient osclient.Interface
	if useServiceAccounts {
		config, err := restclient.InClusterConfig()
		if err != nil {
			glog.V(0).Infof("Error creating cluster config: %s", err)
			os.Exit(1)
		}
		oc, err := osclient.New(config)
		if err != nil {
			log.Printf("Error creating OpenShift client: %s", err)
			os.Exit(2)
		}
		kc, err := kclient.New(config)
		if err != nil {
			glog.V(0).Infof("Error creating Kubernetes client: %s", err)
			os.Exit(3)
		}
		openshiftClient = oc
		kubeClient = kc
	} else {
		config, err := clientcmd.DefaultClientConfig(pflag.NewFlagSet("empty", pflag.ContinueOnError)).ClientConfig()
		if err != nil {
			log.Fatalf("Error loading config: %s", err)
		}
		oc, err := osclient.New(config)
		if err != nil {
			log.Fatalf("Error creating OpenShift client: %s", err)
		}
		kc, err := kclient.New(config)
		if err != nil {
			log.Fatalf("Error creating Kubernetes client: %s", err)
		}
		openshiftClient = oc
		kubeClient = kc
	}

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
