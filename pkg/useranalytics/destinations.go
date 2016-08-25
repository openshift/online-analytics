package useranalytics

import (
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
	"strings"

	intercom "gopkg.in/intercom/intercom-go.v2"

	"github.com/golang/glog"
)

// A "Destination" is a thing that can send data to an endpoint. In the "Store and Forward" implementation, this object
// sends data to endpoints and includes any transformation required.
// Current implementations are bespoke and specific to the destination endpoint.
type Destination interface {
	Send(ev *analyticsEvent) error
}

var _ Destination = &WoopraDestination{}
var _ Destination = &IntercomDestination{}

type WoopraDestination struct {
	Method   string
	Endpoint string
	Domain   string
	Client   SimpleHttpClient
}

type IntercomDestination struct {
	Client IntercomEventClient
}

func (d *WoopraDestination) send(params map[string]string) error {
	urlParams := url.Values{}
	for key, value := range params {
		urlParams.Add(key, value)
	}
	encodedParams := urlParams.Encode()
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
		_, err = ioutil.ReadAll(resp.Body)
		if err != nil {
			return fmt.Errorf("error forwarding analytic: %v", err)
		}

	} else if d.Method == "POST" {
		return fmt.Errorf("not implemented yet")
	} else {
		return fmt.Errorf("Unknown HTTP method, was not GET or POST")
	}

	return nil
}

func (d *WoopraDestination) Send(ev *analyticsEvent) error {
	// all vendor-specific field mapping to be done here
	params := map[string]string{
		"host":                 d.Domain,
		"event":                ev.event,
		"cv_email":             ev.userID,
		"cv_project_namespace": ev.objectNamespace,
		"ce_name":              ev.objectName,
		"ce_namespace":         ev.objectNamespace,
		"ce_uid":               ev.objectUID,
		"ce_timestamp":         ev.timestamp.String(),
		"ce_cluster":           ev.clusterName,
		"ce_controller_id":     ev.controllerID,
	}
	for key, value := range ev.properties {
		params[key] = value
	}
	return d.send(params)
}

func (d *IntercomDestination) Send(ev *analyticsEvent) error {
	iev := &intercom.Event{
		Email:     ev.objectNamespace,
		UserID:    ev.userID,
		EventName: ev.event,
		CreatedAt: ev.timestamp.Unix(),
		Metadata: map[string]interface{}{
			"cv_project_namespace": ev.objectNamespace,
		},
	}
	glog.V(6).Infof("Intercom event %#v", iev)
	return d.Client.Save(iev)
}

// SimpleHttpClient is a tiny HTTP interface that allows easy mock testing
type SimpleHttpClient interface {
	Get(endpoint string) (resp *http.Response, err error)
	Post(endpoint string, bodyType string, body io.Reader) (resp *http.Response, err error)
}

func NewSimpleHttpClient(username, password string) SimpleHttpClient {
	return &realHttpClient{username, password, &http.Client{}}
}

type realHttpClient struct {
	// used for basic auth when both strings are non-empty
	username   string
	password   string
	httpClient *http.Client
}

var _ SimpleHttpClient = &realHttpClient{}

func (h *realHttpClient) Get(endpoint string) (*http.Response, error) {
	req, err := http.NewRequest("GET", endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("error creating request: %v", err)
	}
	if len(h.username) != 0 && len(h.password) != 0 {
		req.SetBasicAuth(h.username, h.password)
	}
	resp, e := h.httpClient.Do(req)
	if e != nil {
		return nil, e
	}
	_, err = ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("error forwarding analytic: %v", err)
	}
	return resp, nil
}

func (h *realHttpClient) Post(endpoint string, bodyType string, body io.Reader) (resp *http.Response, err error) {
	return nil, fmt.Errorf("not implemented yet")
}

func prepEndpoint(endpoint string) string {
	if strings.Index(endpoint, "?%s") != (len(endpoint) - 3) {
		endpoint = endpoint + "?%s"
	}

	return endpoint
}

type IntercomEventClient interface {
	Save(ev *intercom.Event) error
}

func NewIntercomEventClient(client *intercom.Client) IntercomEventClient {
	return &realIntercomEventClient{client}
}

type realIntercomEventClient struct {
	Client *intercom.Client
}

func (c *realIntercomEventClient) Save(ev *intercom.Event) error {
	return c.Client.Events.Save(ev)
}
