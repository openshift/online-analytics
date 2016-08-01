package useranalytics

import (
	"fmt"
	"reflect"
	"strings"
	"time"

	"k8s.io/kubernetes/pkg/api"
	meta "k8s.io/kubernetes/pkg/api/meta"
	"k8s.io/kubernetes/pkg/runtime"
	"k8s.io/kubernetes/pkg/watch"
)

type analyticsEvent struct {
	// userID of the namespace/project owner.  TODO: change to action owner
	userID string
	// pod_add, secret_delete, etc.
	event string
	// Pod, ReplicationController, etc.
	objectKind string
	objectName string
	objectUID  string
	// Namespace/Project. Owner of project is analyticEvent owner.
	objectNamespace string
	// instance ID of the controller to help detect dupes
	controllerID string
	clusterName  string
	properties   map[string]string
	// timestamp of event occurrence
	timestamp time.Time
	// the name of the dest to send this event to
	destination string
	// unix time when this event was successfully sent to the destination
	sentTime int64
	// any error message that occurs during sending to destination
	errorMessage string
}

func newEventFromRuntime(obj runtime.Object, eventType watch.EventType) (*analyticsEvent, error) {
	m, err := meta.Accessor(obj)
	if err != nil {
		return nil, fmt.Errorf("Unable to create object meta for %v", obj)
	}
	o2 := reflect.ValueOf(obj)
	simpleTypeName := strings.ToLower(strings.Replace(o2.Type().String(), "*api.", "", 1))
	eventName := fmt.Sprintf("%s_%s", simpleTypeName, strings.ToLower(string(eventType)))

	analyticEvent := &analyticsEvent{
		objectKind:      simpleTypeName,
		event:           eventName,
		objectName:      m.GetName(),
		objectNamespace: m.GetNamespace(),
		objectUID:       string(m.GetUID()),
		properties:      make(map[string]string),
		timestamp:       time.Now(),
	}

	// TODO: this is deprecated. Replace with meta.Accessor after rebase.
	om, err := api.ObjectMetaFor(obj)
	// These funcs are in a newer version of Kube. Rebase is currently underway.
	//	_ = meta.GetCreationTimestamp()
	//	_ = meta.GetDeletionTimestamp()

	switch eventType {
	case watch.Added:
		analyticEvent.timestamp = om.CreationTimestamp.Time
	case watch.Deleted:
		analyticEvent.timestamp = om.DeletionTimestamp.Time
	default:
		return nil, fmt.Errorf("Unknown event %v", eventType)
	}

	return analyticEvent, nil
}
func newEvent(obj interface{}, eventType watch.EventType) (*analyticsEvent, error) {
	if rt, ok := obj.(runtime.Object); ok {
		return newEventFromRuntime(rt, eventType)
	}
	return nil, fmt.Errorf("Object not kind runtime.Object:  %v", obj)
}

func (ev *analyticsEvent) Hash() string {
	return fmt.Sprintf("%s,%s,%s,%s,%s,%s", ev.userID, ev.event, ev.objectKind, ev.objectName, ev.objectNamespace, ev.destination)
}
