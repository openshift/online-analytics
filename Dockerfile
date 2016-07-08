FROM golang:1.6

LABEL BZComponent="oso-user-analytics-docker"
LABEL Name="openshift3/oso-user-analytics"
LABEL Version="v3.3.0.0"
LABEL Release="1"
LABEL Architecture="x86_64"

ADD . /go/src/github.com/openshift/online/user-analytics

WORKDIR /go/src/github.com/openshift/online/user-analytics
RUN make install test && cp /go/bin/user-analytics /usr/bin/user-analytics
ENTRYPOINT ["/usr/bin/user-analytics"]
