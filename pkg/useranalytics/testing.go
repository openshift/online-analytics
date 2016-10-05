package useranalytics

import (
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/golang/glog"
	"k8s.io/kubernetes/pkg/util/rand"
	"k8s.io/kubernetes/pkg/util/wait"

	intercom "gopkg.in/intercom/intercom-go.v2"
)

var _ Destination = &mockDestination{}

type mockDestination struct {}

func (d *mockDestination) Send(ev *analyticsEvent) error {
	return nil
}

type mockHttpClient struct {
	url string
}

var _ SimpleHttpClient = &mockHttpClient{}

func (c *mockHttpClient) Get(endpoint string) (resp *http.Response, err error) {
	c.url = endpoint
	return &http.Response{
		Status: "200 OK",
		Body:   &mockResponse{},
	}, nil
}

func (c *mockHttpClient) Post(endpoint string, bodyType string, body io.Reader) (resp *http.Response, err error) {
	c.url = endpoint
	return nil, fmt.Errorf("not implemented yet")
}

type mockResponse struct {
	io.Reader
	io.Closer
}

func (r *mockResponse) Read(p []byte) (n int, err error) {
	for i, _ := range p {
		p[i] = 1
	}
	return 1, io.EOF
}

func (r *mockResponse) Write(p []byte) (n int, err error) {
	for i, _ := range p {
		p[i] = 2
	}
	return 1, io.EOF
}

type mockIntercomEventClient struct {
	event *intercom.Event
}

func (m *mockIntercomEventClient) Save(ev *intercom.Event) error {
	m.event = ev
	return nil
}

type MockHttpEndpoint struct {
	Port            int
	RequestsHandled int
	LastRequestURI  string
	URLPrefix       string
	// maximum number of milliseconds to handle a request.
	// 0 for no latency
	MaxLatency      int
	// percent chance of returning HTTP error, 0 - 100
	FlakeRate       int
	DupeCheck       bool
	// contains the unique hash of an event and the number of times it is posted to the endpoint
	Analytics       map[string]int
}

func (m *MockHttpEndpoint) Run(stopCh <-chan struct {}) {
	if m.FlakeRate < 0 {
		m.FlakeRate = 0
	}
	if m.FlakeRate > 100 {
		m.FlakeRate = 100
	}
	m.Analytics = make(map[string]int)

	go wait.Until(m.serve, 1 * time.Second, stopCh)
	go wait.Until(m.log, 60 * time.Second, stopCh)
	glog.Info("Mock endpoint started")
}

func (m *MockHttpEndpoint) log() {
	fmt.Printf("MockHttpEndpoint on %d handled %d requests\n", m.Port, m.RequestsHandled)
}

func (m *MockHttpEndpoint) serve() {
	glog.V(1).Infof("Mock endpoint listening at %s", m.URLPrefix)
	http.HandleFunc("/", m.handler)
	strPort := fmt.Sprintf(":%d", m.Port)
	if err := http.ListenAndServe(strPort, nil); err != nil {
		glog.Fatal("Could not start server")
	}
}

func (m *MockHttpEndpoint) handler(w http.ResponseWriter, r *http.Request) {
	m.LastRequestURI = r.RequestURI
	m.RequestsHandled++

	r.ParseForm()
	host := r.FormValue("host")
	event := r.FormValue("event")
	cv_email := r.FormValue("cv_email")
	cv_project_namespace := r.FormValue("cv_project_namespace")
	ce_name := r.FormValue("ce_name")
	ce_namespace := r.FormValue("ce_namespace")
	ce_uid := r.FormValue("ce_uid")
	ce_timestamp := r.FormValue("ce_timestamp")

	hash := fmt.Sprintf("%s, %s, %s, %s, %s, %s, %s, %s", host, event, cv_email, cv_project_namespace, ce_name, ce_namespace, ce_uid, ce_timestamp)

	glog.V(5).Infof("MockEndpoint received %v", hash)

	if m.DupeCheck {
		if _, exists := m.Analytics[hash]; !exists {
			m.Analytics[hash] = 0
		}
		m.Analytics[hash] = m.Analytics[hash] + 1
	}

	rand.Seed(time.Now().Unix())
	latency := 0 * time.Millisecond
	if m.MaxLatency > 0 {
		rnd := rand.Intn(m.MaxLatency)
		duration := time.Duration(rnd)
		latency = duration * time.Millisecond
	}
	time.Sleep(latency)

	if rand.Intn(100) <= m.FlakeRate {
		w.WriteHeader(http.StatusBadRequest)
		fmt.Fprint(w, "sorry error")
	} else {
		fmt.Fprintf(w, "latency: %d, uri: %s", latency, m.LastRequestURI)
	}
}
