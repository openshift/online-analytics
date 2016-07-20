FROM golang:1.6

ADD . /go/src/github.com/openshift/online/user-analytics

WORKDIR /go/src/github.com/openshift/online/user-analytics
RUN make install test && cp /go/bin/user-analytics /usr/bin/user-analytics
ENTRYPOINT ["/usr/bin/user-analytics"]
