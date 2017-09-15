# Data Analytics Integration

This application syncs OpenShift user activity data with external analytics systems.


## Usage

#### Using Woopra with default IDs

```
oc new-app -n openshift-infra -f ansible/roles/oso_analytics/files/user-analytics.yaml \
-p WOOPRA_ENABLED="true" \
-p WOOPRA_ENDPOINT="http://www.woopra.com/track/ce" \
-p WOOPRA_DOMAIN="YOURDOMAIN" \
-p USER_KEY_STRATEGY="[name/uid/annotation]" \
-p USER_KEY_ANNOTATION="[desired annotation]" \
-p LOG_LEVEL=5
-p CLUSTER_NAME="<int/stg/prod/..>"

```

The `CLUSTER_NAME` parameter is used to distinguish different environments (such as INT, STG, or a local test cluster). Its value by default is "kubernetes".

Note the 2 new flags `USER_KEY_STRATEGY` and `USER_KEY_ANNOTATION`. Key strategy refers to the keying method used for users in Woopra:
* `name` will key users by their `user.Name`
* `uid` will key users by their `user.UID`
* `annotation` will key users by an annotation specified in `USER_KEY_ANNOTATION` (this flag is **only** required if using `strategy=annotation`)

The annotation previously used in Devpreview was `openshift.io/online-managed-id`

#### Prometheus metrics enablement

The following flags enable Prometheus metrics gathering:
```
...
-p METRICS_PORT="8080" \
-p METRICS_COLLECT_RUNTIME="true" \
-p METRICS_COLLECT_WOOPRA="true" \
-p METRICS_COLLECT_QUEUE="true" \
```

The metrics are available at `METRICS_PORT/metrics`

`METRICS_COLLECT_RUNTIME` enables various runtime, Go, and process metrics provided by Prometheus

`METRICS_COLLECT_WOOPRA` enables Woopra latency metrics at `/analytics_woopra_latency_seconds`

`METRICS_COLLECT_QUEUE` enables analytics on the controller's internal events processing queue available at `/analytics_queue_size_events`, which returns the current size of the queue, and `/analytics_events_handled`, which returns the total number of events processed since the start of the controller.

### Local development and testing

#### Using a Local Endpoint

```
oc new-app -n openshift-infra -f templates/user-analytics.yaml \
-p WOOPRA_ENABLED="false" \
-p LOCAL_ENDPOINT_ENABLED="true" \
-p LOG_LEVEL=5
```

> Note! This will log to glog.V(5). A user must be logged in for an analytic to be counted.

Generate analytics by logging in and creating a project w/ basic app.

```
$ oc login
Authentication required for https://10.240.0.2:8443 (openshift)
Username: foo
Password:
Login successful.

You don't have any projects. You can try to create a new project, by running

    oc new-project <projectname>

$ oc new-project test
Now using project "test" on server "https://10.240.0.2:8443".

You can add applications to this project with the 'new-app' command. For example, try:

    oc new-app centos/ruby-22-centos7~https://github.com/openshift/ruby-ex.git

to build a new example application in Ruby.
$ oc new-app centos/ruby-22-centos7~https://github.com/openshift/ruby-ex.git

```

## Building

Build and test with `make`:

* `make` will run vendor dependencies and then `install`
* `make build` will compile the binary for the application
* `make test` will compile and run the unit tests
* `make test-integration` will compile and run the integration tests against an OpenShift master


## Analytic Events

The following events are observed by a controller and sent to an analytics provider via Basic Authenticated GET w/ encoded URL.


## User

### user_created

|           | parameter     | v3 field              | description             |
|-----------|---------------|-----------------------|-------------------------|
| ID        | cv_id         | user.annotation["openshift.io/online-managed-id"] | User Identifier for analytics |
| Event     | event         | user_created            | Analytic event identifier |
| Name      | ce_name       | obj.Name              | v3 object name (but not identifier/UID) |
| Namespace | ce_namespace  | obj.Namespace         | object's project/namespace |
| Created   | ce_timestamp  | obj.CreationTimestamp | in milliseconds  |



### user_deleted

|           | parameter | v3 field | description |
|-----------|-----------|----------|------------|
| ID        | cv_id         | user.annotation["openshift.io/online-managed-id"] | User Identifier for analytics |
| Event     | event         | user_deleted            | Analytic event identifier  |
| Name      | ce_name       | obj.Name              | v3 object name (but not identifier/UID) |
| Namespace | ce_namespace  | obj.Namespace         | object's project/namespace  |
| Deleted   | ce_timestamp  | obj.DeletionTimestamp | in milliseconds |


### pod_failed

|           | parameter | v3 field | description |
|-----------|-----------|----------|------------|
| ID        | cv_id         | user.annotation["openshift.io/online-managed-id"] | User Identifier for analytics |
| Event     | event         | pod_failed             | Analytic event identifier  |
| Name      | ce_name       | obj.Name              | v3 object name (but not identifier/UID) |
| Namespace | ce_namespace  | obj.Namespace         | object's project/namespace  |
| Failed   | ce_timestamp  | pod.Status.Condition.LastTransitionTime | in milliseconds |
| Failed   | ce_reason   | pod.Status.Condition.Reason | in milliseconds |

## ReplicationController

### replicationcontroller_created

|           | parameter     | v3 field              | description             |
|-----------|---------------|-----------------------|-------------------------|
| ID        | cv_id         | user.annotation["openshift.io/online-managed-id"] | User Identifier for analytics |
| Event     | event         | replicationcontroller_created            | Analytic event identifier |
| Name      | ce_name       | obj.Name              | v3 object name (but not identifier/UID) |
| Namespace | ce_namespace  | obj.Namespace         | object's project/namespace |
| Created   | ce_timestamp  | obj.CreationTimestamp | in milliseconds  |
| Replica Count  | ce_replica_count | obj.Spec.Replicas  | The number of replicas of a pod  |

## PersistentVolumeClaim

### persistentvolumeclaim_created

|           | parameter     | v3 field              | description             |
|-----------|---------------|-----------------------|-------------------------|
| ID        | cv_id         | user.annotation["openshift.io/online-managed-id"] | User Identifier for analytics |
| Event     | event         | persistentvolumeclaim_created            | Analytic event identifier |
| Name      | ce_name       | obj.Name              | v3 object name (but not identifier/UID) |
| Namespace | ce_namespace  | obj.Namespace         | object's project/namespace |
| Created   | ce_timestamp  | obj.CreationTimestamp | in milliseconds  |
| Capacity   | ce_capacity   | obj.Spec.Capacity | The requested storage capacity   |
| Access Modes   | ce_access_modes  | stringify(obj.Spec.AccessModes | The requested access modes for storage |

### persistentvolumeclaim_bound

|           | parameter     | v3 field              | description             |
|-----------|---------------|-----------------------|-------------------------|
| ID        | cv_id         | user.annotation["openshift.io/online-managed-id"] | User Identifier for analytics |
| Event     | event         | persistentvolumeclaim_created            | Analytic event identifier |
| Name      | ce_name       | obj.Name              | v3 object name (but not identifier/UID) |
| Namespace | ce_namespace  | obj.Namespace         | object's project/namespace |
| Bound Date  | ce_timestamp  | obj.Condition.LastTransitionTime | in milliseconds  |
| Request Capacity   | ce_requested_capacity   | obj.Spec.Capacity | The requested storage capacity   |
| Request Access Modes   | ce_requested_access_modes  | stringify(obj.Spec.AccessModes | The requested access modes for storage |
| Actual Capacity   | ce_actual_capacity   | obj.Status.Capacity | The actual storage capacity of the backing volume   |
| Actual Access Modes   | ce_actual_access_modes  | stringify(obj.Status.AccessModes | The actual storage capacity of the backing volume  |



### deploymentconfig_failed

|           | parameter | v3 field | description |
|-----------|-----------|----------|------------|
| ID        | cv_id         | user.annotation["openshift.io/online-managed-id"] | User Identifier for analytics |
| Event     | event         | deploymentconfig_failed            | Analytic event identifier  |
| Name      | ce_name       | obj.Name              | v3 object name (but not identifier/UID) |
| Namespace | ce_namespace  | obj.Namespace         | object's project/namespace  |
| Failed   | ce_timestamp  | ? | ? |


### build_failed

|           | parameter | v3 field | description |
|-----------|-----------|----------|------------|
| ID        | cv_id         | user.annotation["openshift.io/online-managed-id"] | User Identifier for analytics |
| Event     | event         | build_failed            | Analytic event identifier  |
| Name      | ce_name       | obj.Name              | v3 object name (but not identifier/UID) |
| Namespace | ce_namespace  | obj.Namespace         | object's project/namespace  |
| Failed   | ce_timestamp  | ? | ? |




## Pod

### pod_created

|           | parameter     | v3 field              | description             |
|-----------|---------------|-----------------------|-------------------------|
| ID        | cv_id         | user.annotation["openshift.io/online-managed-id"] | User Identifier for analytics |
| Event     | event         | pod_created            | Analytic event identifier |
| Name      | ce_name       | obj.Name              | v3 object name (but not identifier/UID) |
| Namespace | ce_namespace  | obj.Namespace         | object's project/namespace |
| Created   | ce_timestamp  | obj.CreationTimestamp | in milliseconds  |

## ReplicationRontroller

### replicationrontroller_created

|           | parameter     | v3 field              | description             |
|-----------|---------------|-----------------------|-------------------------|
| ID        | cv_id         | user.annotation["openshift.io/online-managed-id"] | User Identifier for analytics |
| Event     | event         | replicationrontroller_created            | Analytic event identifier |
| Name      | ce_name       | obj.Name              | v3 object name (but not identifier/UID) |
| Namespace | ce_namespace  | obj.Namespace         | object's project/namespace |
| Created   | ce_timestamp  | obj.CreationTimestamp | in milliseconds  |

## PersistentVolumeClaim

### persistentvolumeclaim_created

|           | parameter     | v3 field              | description             |
|-----------|---------------|-----------------------|-------------------------|
| ID        | cv_id         | user.annotation["openshift.io/online-managed-id"] | User Identifier for analytics |
| Event     | event         | persistentvolumeclaim_created            | Analytic event identifier |
| Name      | ce_name       | obj.Name              | v3 object name (but not identifier/UID) |
| Namespace | ce_namespace  | obj.Namespace         | object's project/namespace |
| Created   | ce_timestamp  | obj.CreationTimestamp | in milliseconds  |

## Secret

### secret_created

|           | parameter     | v3 field              | description             |
|-----------|---------------|-----------------------|-------------------------|
| ID        | cv_id         | user.annotation["openshift.io/online-managed-id"] | User Identifier for analytics |
| Event     | event         | secret_created            | Analytic event identifier |
| Name      | ce_name       | obj.Name              | v3 object name (but not identifier/UID) |
| Namespace | ce_namespace  | obj.Namespace         | object's project/namespace |
| Created   | ce_timestamp  | obj.CreationTimestamp | in milliseconds  |

## Service

### service_created

|           | parameter     | v3 field              | description             |
|-----------|---------------|-----------------------|-------------------------|
| ID        | cv_id         | user.annotation["openshift.io/online-managed-id"] | User Identifier for analytics |
| Event     | event         | service_created            | Analytic event identifier |
| Name      | ce_name       | obj.Name              | v3 object name (but not identifier/UID) |
| Namespace | ce_namespace  | obj.Namespace         | object's project/namespace |
| Created   | ce_timestamp  | obj.CreationTimestamp | in milliseconds  |

## Namespace

### namespace_created

|           | parameter     | v3 field              | description             |
|-----------|---------------|-----------------------|-------------------------|
| ID        | cv_id         | user.annotation["openshift.io/online-managed-id"] | User Identifier for analytics |
| Event     | event         | namespace_created            | Analytic event identifier |
| Name      | ce_name       | obj.Name              | v3 object name (but not identifier/UID) |
| Namespace | ce_namespace  | obj.Namespace         | object's project/namespace |
| Created   | ce_timestamp  | obj.CreationTimestamp | in milliseconds  |

## Deployment

### deployment_created

|           | parameter     | v3 field              | description             |
|-----------|---------------|-----------------------|-------------------------|
| ID        | cv_id         | user.annotation["openshift.io/online-managed-id"] | User Identifier for analytics |
| Event     | event         | deployment_created            | Analytic event identifier |
| Name      | ce_name       | obj.Name              | v3 object name (but not identifier/UID) |
| Namespace | ce_namespace  | obj.Namespace         | object's project/namespace |
| Created   | ce_timestamp  | obj.CreationTimestamp | in milliseconds  |

## Route

### route_created

|           | parameter     | v3 field              | description             |
|-----------|---------------|-----------------------|-------------------------|
| ID        | cv_id         | user.annotation["openshift.io/online-managed-id"] | User Identifier for analytics |
| Event     | event         | route_created            | Analytic event identifier |
| Name      | ce_name       | obj.Name              | v3 object name (but not identifier/UID) |
| Namespace | ce_namespace  | obj.Namespace         | object's project/namespace |
| Created   | ce_timestamp  | obj.CreationTimestamp | in milliseconds  |

## Build

### build_created

|           | parameter     | v3 field              | description             |
|-----------|---------------|-----------------------|-------------------------|
| ID        | cv_id         | user.annotation["openshift.io/online-managed-id"] | User Identifier for analytics |
| Event     | event         | build_created            | Analytic event identifier |
| Name      | ce_name       | obj.Name              | v3 object name (but not identifier/UID) |
| Namespace | ce_namespace  | obj.Namespace         | object's project/namespace |
| Created   | ce_timestamp  | obj.CreationTimestamp | in milliseconds  |

## RoleBinding

### rolebinding_created

|           | parameter     | v3 field              | description             |
|-----------|---------------|-----------------------|-------------------------|
| ID        | cv_id         | user.annotation["openshift.io/online-managed-id"] | User Identifier for analytics |
| Event     | event         | rolebinding_created            | Analytic event identifier |
| Name      | ce_name       | obj.Name              | v3 object name (but not identifier/UID) |
| Namespace | ce_namespace  | obj.Namespace         | object's project/namespace |
| Created   | ce_timestamp  | obj.CreationTimestamp | in milliseconds  |

## Template

### template_created

|           | parameter     | v3 field              | description             |
|-----------|---------------|-----------------------|-------------------------|
| ID        | cv_id         | user.annotation["openshift.io/online-managed-id"] | User Identifier for analytics |
| Event     | event         | template_created            | Analytic event identifier |
| Name      | ce_name       | obj.Name              | v3 object name (but not identifier/UID) |
| Namespace | ce_namespace  | obj.Namespace         | object's project/namespace |
| Created   | ce_timestamp  | obj.CreationTimestamp | in milliseconds  |

## ImageStream

### imagestream_created

|           | parameter     | v3 field              | description             |
|-----------|---------------|-----------------------|-------------------------|
| ID        | cv_id         | user.annotation["openshift.io/online-managed-id"] | User Identifier for analytics |
| Event     | event         | imagestream_created            | Analytic event identifier |
| Name      | ce_name       | obj.Name              | v3 object name (but not identifier/UID) |
| Namespace | ce_namespace  | obj.Namespace         | object's project/namespace |
| Created   | ce_timestamp  | obj.CreationTimestamp | in milliseconds  |
