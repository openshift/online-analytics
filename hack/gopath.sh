#/bin/bash
set -o errexit
set -o nounset
set -o pipefail

if [ "${GOPATH:-}" == "" ]; then
	echo "GOPATH is undefined"
	exit 1
fi
printf "%s/src/github.com/openshift/online/user-analytics/internal/src/github.com/openshift/origin/Godeps/_workspace:%s/src/github.com/openshift/online/user-analytics/internal:%s" "$GOPATH" "$GOPATH" "$GOPATH"

