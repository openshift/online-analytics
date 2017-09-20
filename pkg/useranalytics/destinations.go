package useranalytics

import (
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
	"strings"

	log "github.com/Sirupsen/logrus"
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

func (d *WoopraDestination) send(reqLog *log.Entry, params map[string]string) error {
	urlParams := url.Values{}
	for key, value := range params {
		urlParams.Add(key, value)
	}
	encodedParams := urlParams.Encode()
	reqLog.Debugln("sending request")
	if d.Method == "GET" {
		endpoint := d.Endpoint
		if strings.Index(endpoint, "?%s") != (len(endpoint) - 3) {
			endpoint = endpoint + "?%s"
		}
		encodedUrl := fmt.Sprintf(endpoint, encodedParams)
		reqLog.Debugf("GET request: %s", encodedUrl)
		resp, err := d.Client.Get(encodedUrl)
		if err != nil {
			reqLog.Error(err)
			return err
		}
		reqLog = reqLog.WithFields(log.Fields{
			"status": resp.StatusCode,
		})

		body, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			err = fmt.Errorf("error reading response body: %v", err)
			reqLog.Error(err)
			return err
		}
		reqLog.Debugln("response body: ", string(body))
		// A non-200 HTTP status does not throw a golang error, must check ourselves:
		// NOTE: This appears to happen quite a bit. This was silently ignored previously,
		// not returning an error here until we know more, for now just warning.
		if resp.StatusCode < 200 || resp.StatusCode > 299 {
			reqLog.WithFields(log.Fields{"body": string(body)}).Warn("unexpected HTTP response")
			return nil
		}

		// This is the one line we log at INFO level per request, it should cover enough
		// information to identify the exact event, user, endpoint, and HTTP status.
		reqLog.Infoln("analytic event sent")

	} else if d.Method == "POST" {
		return fmt.Errorf("POST not implemented yet")
	} else {
		return fmt.Errorf("Unknown HTTP method, was not GET or POST")
	}

	return nil
}

func (d *WoopraDestination) Send(ev *analyticsEvent) error {
	reqLog := log.WithFields(log.Fields{
		"endpoint":        d.Endpoint,
		"name":            ev.event,
		"objectName":      ev.objectName,
		"objectKind":      ev.objectKind,
		"objectNamespace": ev.objectNamespace,
		"objectUID":       ev.objectUID,
		"eventTimestamp":  ev.timestamp,
		"userID":          ev.userID,
	})
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
	return d.send(reqLog, params)
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
	req, err := http.NewRequest("GET", endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("error creating request: %v", err)
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
	return nil, fmt.Errorf("POST not implemented yet")
}

func prepEndpoint(endpoint string) string {
	if strings.Index(endpoint, "?%s") != (len(endpoint) - 3) {
		endpoint = endpoint + "?%s"
	}

	return endpoint
}
