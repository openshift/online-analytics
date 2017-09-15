FROM golang:1.8
ENV PATH=/go/bin:$PATH GOPATH=/go

ADD . /go/src/github.com/openshift/online-analytics

WORKDIR /go/src/github.com/openshift/online-analytics
RUN make build TARGET=prod
ENTRYPOINT ["user-analytics"]
