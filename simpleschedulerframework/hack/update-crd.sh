#!/usr/bin/env bash

set -o errexit
set -o nounset
set -o pipefail

REPO_ROOT=$(pwd)

echo "Generating CRDs With controller-gen"
GOPATH=$(go env GOPATH | awk -F ':' '{print $1}')
export PATH=$PATH:$GOPATH/bin
if ! which controller-gen 
then
  GO11MODULE=on go install sigs.k8s.io/controller-tools/cmd/controller-gen@v0.16.5
fi


controller-gen crd paths=./pkg/apis/crd/simpleio/... output:crd:dir="${REPO_ROOT}/deploy/crds"
