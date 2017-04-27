package useranalytics

import (
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
	"strings"

	"github.com/golang/glog"
)

// A "Destination" is a thing that can send data to an endpoint. In the "Store and Forward" implementation, this object
// sends data to endpoints and includes any transformation required.
// Current implementations are bespoke and specific to the destination endpoint.
type Destination interface {
	Send(ev *analyticsEvent) error
}

var _ Destination = &WoopraDestination{}

type WoopraDestination struct {
	Method   string
	Endpoint string
	Domain   string
	Client   SimpleHttpClient
}

func (d *WoopraDestination) send(params map[string]string) error {
	urlParams := url.Values{}
	for key, value := range params {
		urlParams.Add(key, value)
	}
	encodedParams := urlParams.Encode()
	glog.V(6).Infof("Sending request to WoopraDestination: %+v", d)
	if d.Method == "GET" {
		endpoint := d.Endpoint
		if strings.Index(endpoint, "?%s") != (len(endpoint) - 3) {
			endpoint = endpoint + "?%s"
		}
		encodedUrl := fmt.Sprintf(endpoint, encodedParams)
		glog.V(6).Infof("GET request to %s", encodedUrl)
		resp, err := d.Client.Get(encodedUrl)
		if err != nil {
			return err
		}
		glog.V(6).Infof("Response from GET request to %s: %s", encodedUrl, resp.Body)
		_, err = ioutil.ReadAll(resp.Body)
		if err != nil {
			return fmt.Errorf("error forwarding analytic: %v", err)
		}

	} else if d.Method == "POST" {
		return fmt.Errorf("POST not implemented yet")
	} else {
		return fmt.Errorf("Unknown HTTP method, was not GET or POST")
	}

	return nil
}

func (d *WoopraDestination) Send(ev *analyticsEvent) error {
	// all vendor-specific field mapping to be done here
	params := map[string]string{
		"host":             d.Domain,
		"event":            ev.event,
		"cv_id":            ev.userID,
		"ce_name":          ev.objectName,
		"ce_namespace":     ev.objectNamespace,
		"ce_uid":           ev.objectUID,
		"ce_timestamp":     ev.timestamp.String(),
		"ce_created_at":    fmt.Sprintf("%d", ev.timestamp.Unix()),
		"ce_cluster":       ev.clusterName,
		"ce_controller_id": ev.controllerID,
		"timeout":          "1800000",
		"ip":               "0.0.0.0",
		"cookie":           ev.userID,
	}
	for key, value := range ev.properties {
		params[key] = value
	}
	return d.send(params)
}

// SimpleHttpClient is a tiny HTTP interface that allows easy mock testing
type SimpleHttpClient interface {
	Get(endpoint string) (resp *http.Response, err error)
	Post(endpoint string, bodyType string, body io.Reader) (resp *http.Response, err error)
}

func NewSimpleHttpClient() SimpleHttpClient {
	return &realHttpClient{&http.Client{}}
}

// Keep this struct in case we want to add secure tracking secret later on
type realHttpClient struct {
	httpClient *http.Client
}

var _ SimpleHttpClient = &realHttpClient{}

func (h *realHttpClient) Get(endpoint string) (*http.Response, error) {
	glog.V(6).Infof("Creating new GET request")
	req, err := http.NewRequest("GET", endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("error creating request: %v", err)
	}

	glog.V(6).Infof("Sending GET request: %+v", req)
	resp, e := h.httpClient.Do(req)
	if e != nil {
		return nil, e
	}

	glog.V(6).Infof("GET request response: %s", resp.Body)
	_, err = ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("error forwarding analytic: %v", err)
	}
	return resp, nil
}

func (h *realHttpClient) Post(endpoint string, bodyType string, body io.Reader) (resp *http.Response, err error) {
	return nil, fmt.Errorf("POST not implemented yet")
}

func prepEndpoint(endpoint string) string {
	if strings.Index(endpoint, "?%s") != (len(endpoint) - 3) {
		endpoint = endpoint + "?%s"
	}

	return endpoint
}
