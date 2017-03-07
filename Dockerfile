# Data Analytics Integration for OpenShift Online v3
#
# This image provides a container application that syncs OpenShift user activity
# data with external analytics systems.

FROM rhel7.2:7.2-released

ENV PATH=/go/bin:$PATH \
    GOPATH=/go

LABEL com.redhat.component="oso-user-analytics-docker" \
      name="openshift3/oso-user-analytics" \
      version="v3.3.0.0" \
      architecture="x86_64"

ADD . /go/src/github.com/openshift/online/user-analytics

RUN yum-config-manager --enable rhel-7-server-optional-rpms && \
    INSTALL_PKGS="golang make" && \
    yum install -y --setopt=tsflags=nodocs $INSTALL_PKGS && \
    rpm -V $INSTALL_PKGS && \
    yum clean all -y

WORKDIR /go/src/github.com/openshift/online/user-analytics
RUN make build TARGET=prod
ENTRYPOINT ["user-analytics"]
