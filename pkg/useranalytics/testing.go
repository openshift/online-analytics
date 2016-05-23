package useranalytics

import (
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/golang/glog"
	"k8s.io/kubernetes/pkg/util/wait"

	"k8s.io/kubernetes/pkg/util/rand"
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

type mockHttpEndpoint struct {
	port       int
	method     string
	handled    int
	last       string
	urlPrefix  string
	// minimum number of milliseconds to handle a request
	minLatency int
	// maximum number of milliseconds to handle a request
	maxLatency int
	// percent chance of returning HTTP error
	flakeRate  int
}

func (m *mockHttpEndpoint) start(stopCh <-chan struct {}) {
	if m.flakeRate < 0 {
		m.flakeRate = 0
	}
	if m.flakeRate > 100 {
		m.flakeRate = 100
	}
	go wait.Until(m.serve, 1 * time.Second, stopCh)
	go wait.Until(m.log, 1 * time.Second, stopCh)
}

func (m *mockHttpEndpoint) log() {
	fmt.Printf("MockHttpEndpoint on %d handled %d requests\n", m.port, m.handled)
}

func (m *mockHttpEndpoint) serve() {
	rand.Seed(time.Now().Unix())
	http.HandleFunc(fmt.Sprintf("/%s", m.urlPrefix), m.handler)
	strPort := fmt.Sprintf(":%d", m.port)
	if err := http.ListenAndServe(strPort, nil); err != nil {
		glog.Fatal("Could not start server")
	}
}

func (m *mockHttpEndpoint) handler(w http.ResponseWriter, r *http.Request) {
	m.last = r.RequestURI
	m.handled++
	rnd := random(m.minLatency, m.maxLatency)

	duration := time.Duration(rnd)
	latency := duration * time.Millisecond
	time.Sleep(latency)

	if random(1, 100) <= m.flakeRate {
		w.WriteHeader(http.StatusBadRequest)
		fmt.Fprint(w, "sorry error")
	} else {
		fmt.Fprintf(w, "latency: %d, uri: %s", latency, m.last)
	}
}

func random(min, max int) int {
	return rand.Intn(max - min) + min
}
