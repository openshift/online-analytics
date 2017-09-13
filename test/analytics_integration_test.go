package test

import (
	"flag"
	"testing"
	"time"

	"github.com/openshift/online-analytics/pkg/useranalytics"

	osclient "github.com/openshift/origin/pkg/client"
	"github.com/openshift/origin/pkg/cmd/server/bootstrappolicy"
	testutil "github.com/openshift/origin/test/util"
	testserver "github.com/openshift/origin/test/util/server"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/uuid"
	kclientset "k8s.io/kubernetes/pkg/client/clientset_generated/internalclientset"

	"k8s.io/apimachinery/pkg/api/resource"
	api "k8s.io/kubernetes/pkg/api"
	//	buildapi "github.com/openshift/origin/pkg/build/api"
	//	deployapi "github.com/openshift/origin/pkg/deploy/api"
	//	imageapi "github.com/openshift/origin/pkg/image/api"
	projectapi "github.com/openshift/origin/pkg/project/apis/project"
	//	routeapi "github.com/openshift/origin/pkg/route/api"
	//	templateapi "github.com/openshift/origin/pkg/template/api"
	userapi "github.com/openshift/origin/pkg/user/apis/user"
	//	"github.com/golang/glog"
)

type testHarness struct {
	kubeClient   *kclientset.Clientset
	osClient     *osclient.Client
	t            *testing.T
	user         *userapi.User
	namespace    *api.Namespace
	mockEndpoint useranalytics.MockHttpEndpoint
	timeout      time.Duration
}

func TestProvisioner(t *testing.T) {

	flag.Set("v", "2")

	testutil.RequireEtcd(t)
	_, kubeConfig, err := testserver.StartTestMaster()
	if err != nil {
		t.Fatal(err)
	}

	restConfig, err := testutil.GetClusterAdminClientConfig(kubeConfig)
	if err != nil {
		t.Fatal(err)
	}

	openshiftClient, err := osclient.New(restConfig)
	if err != nil {
		t.Fatal(err)
	}
	kubeClient := kclientset.NewForConfigOrDie(restConfig)

	config := &useranalytics.AnalyticsControllerConfig{
		Destinations:            make(map[string]useranalytics.Destination),
		KubeClient:              kubeClient,
		OSClient:                openshiftClient,
		MaximumQueueLength:      10000,
		MetricsPollingFrequency: 5,
		UserKeyStrategy:         "uid",
	}

	config.Destinations["mock"] = &useranalytics.WoopraDestination{
		Method:   "GET",
		Domain:   "test",
		Endpoint: "http://127.0.0.1:8888/dest",
		Client:   useranalytics.NewSimpleHttpClient(),
	}

	analyticsController, err := useranalytics.NewAnalyticsController(config)
	if err != nil {
		t.Fatalf("Error creating controller %v", err)
	}

	mockEndpoint := useranalytics.MockHttpEndpoint{
		Port:       8888,
		URLPrefix:  "/dest",
		MaxLatency: 0, // no latency need for dupe test
		FlakeRate:  0, // flakes not needed for dupe test
	}

	c := make(chan struct{})
	defer close(c)

	analyticsController.Run(c, 3)
	mockEndpoint.Run(c)

	harness := &testHarness{
		kubeClient:   kubeClient,
		osClient:     openshiftClient,
		t:            t,
		mockEndpoint: mockEndpoint,
		timeout:      10 * time.Second,
	}

	generateUserAndNamespace(harness)
	generateObjects(1, harness)

	// allow all watches a chance to get their analytics to the endpoint
	time.Sleep(2 * time.Second)

	for name, cnt := range mockEndpoint.Analytics {
		if cnt > 1 {
			t.Errorf("Dupe! Counted %d for %s", cnt, name)
		}
	}
}

func generateUserAndNamespace(harness *testHarness) {
	user, err := harness.osClient.Users().Create(&userapi.User{
		ObjectMeta: metav1.ObjectMeta{
			Name: "foo-user",
			Annotations: map[string]string{
				useranalytics.OnlineManagedID: string(uuid.NewUUID()),
			},
		},
	})
	if err != nil {
		harness.t.Fatalf("Error creating user %v", err)
	}

	namespace, err := harness.kubeClient.Namespaces().Create(&api.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: testutil.Namespace(),
			Annotations: map[string]string{
				projectapi.ProjectRequester: user.Name,
			},
		},
	})
	if err != nil {
		harness.t.Fatalf("Error creating namespace: %v", err)
	}

	if err := testserver.WaitForServiceAccounts(harness.kubeClient, namespace.Name, []string{bootstrappolicy.DefaultServiceAccountName}); err != nil {
		harness.t.Fatalf("Error waiting for service account: %s", err)
	}
	harness.user = user
	harness.namespace = namespace
}

func generateObjects(count int, harness *testHarness) {
	for i := 0; i <= count; i++ {
		time.Sleep(250 * time.Millisecond)
		generatePod(harness)
		time.Sleep(250 * time.Millisecond)
		generateReplicationController(harness)
		time.Sleep(250 * time.Millisecond)
		generatePersistentVolumeClaim(harness)
		time.Sleep(250 * time.Millisecond)
		generateSecret(harness)
	}
}

func generatePod(harness *testHarness) {
	pod := &api.Pod{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: "pod-",
			Namespace:    testutil.Namespace(),
		},
		Spec: api.PodSpec{
			Containers: []api.Container{
				{
					Name:            "ctr",
					Image:           "img",
					ImagePullPolicy: "IfNotPresent",
				},
			},
		},
	}

	_, err := harness.kubeClient.Pods(testutil.Namespace()).Create(pod)
	if err != nil {
		harness.t.Fatalf("Error creating pod: %v", err)
	}
}

func generatePersistentVolumeClaim(harness *testHarness) {
	claim := &api.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: "pvc-",
			Namespace:    testutil.Namespace(),
		},
		Spec: api.PersistentVolumeClaimSpec{
			AccessModes: []api.PersistentVolumeAccessMode{
				api.ReadWriteOnce,
				api.ReadOnlyMany,
			},
			Resources: api.ResourceRequirements{
				Requests: api.ResourceList{
					api.ResourceName(api.ResourceStorage): resource.MustParse("10G"),
				},
			},
		},
	}

	_, err := harness.kubeClient.PersistentVolumeClaims(testutil.Namespace()).Create(claim)
	if err != nil {
		harness.t.Fatalf("Error creating pvc: %v", err)
	}
}

func generateSecret(harness *testHarness) {
	secret := &api.Secret{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: "secret-",
			Namespace:    testutil.Namespace(),
		},
		Data: map[string][]byte{
			"data-1": []byte("bar"),
		},
	}

	_, err := harness.kubeClient.Secrets(testutil.Namespace()).Create(secret)
	if err != nil {
		harness.t.Fatalf("Error creating secret: %v", err)
	}
}

func generateReplicationController(harness *testHarness) {
	rc := &api.ReplicationController{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: "rc-",
			Namespace:    testutil.Namespace(),
		},
		Spec: api.ReplicationControllerSpec{
			Replicas: 1,
			Selector: map[string]string{"foo": "bar"},
			Template: &api.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					GenerateName: "pod-",
					Namespace:    testutil.Namespace(),
					Labels:       map[string]string{"foo": "bar"},
				},
				Spec: api.PodSpec{
					Containers: []api.Container{
						{
							Name:            "ctr",
							Image:           "img",
							ImagePullPolicy: "IfNotPresent",
						},
					},
				},
			},
		},
	}
	_, err := harness.kubeClient.ReplicationControllers(testutil.Namespace()).Create(rc)
	if err != nil {
		harness.t.Fatalf("Error creating replication controller: %v", err)
	}
}
