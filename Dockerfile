FROM openshift/origin-base

RUN yum install -y golang make && yum clean all

ENV PATH=$PATH:$GOROOT/bin
ENV GOPATH=/var/go
ENV GOBIN=$GOPATH/bin

ADD . $GOPATH/src/github.com/openshift/online/user-analytics
WORKDIR $GOPATH/src/github.com/openshift/online/user-analytics

RUN make install && cp $GOBIN/user-analytics /usr/bin/user-analytics

ENTRYPOINT ["/usr/bin/user-analytics"]
