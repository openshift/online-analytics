# Data Analytics Integration for OpenShift Online v3
#
# This image provides a container application that syncs OpenShift user activity
# data with external analytics systems.

FROM rhel7.2:7.2-released

ENV PATH /go/bin:/usr/local/go/bin:$PATH
ENV GOPATH=/go

LABEL BZComponent="oso-user-analytics-docker" \
      Name="openshift3/oso-user-analytics" \
      Version="v3.3.0.0" \
      Architecture="x86_64"

ADD . /go/src/github.com/openshift/online/user-analytics

RUN yum-config-manager --enable rhel-7-server-optional-rpms && \
    INSTALL_PKGS="golang make" && \
    yum install -y --setopt=tsflags=nodocs $INSTALL_PKGS && \
    rpm -V $INSTALL_PKGS && \
    yum clean all -y

WORKDIR /go/src/github.com/openshift/online/user-analytics
RUN export GOPATH && make install test && cp /go/bin/user-analytics /usr/bin/user-analytics
ENTRYPOINT ["/usr/bin/user-analytics"]
