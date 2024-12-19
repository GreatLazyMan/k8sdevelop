#!/usr/bin/env bash
 
# Copyright 2017 The Kubernetes Authors.
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.
 
set -o errexit
set -o nounset
set -o pipefail
 
SCRIPT_ROOT=$(dirname "${BASH_SOURCE[0]}")/..
CODEGEN_PKG=${CODEGEN_PKG:-$(cd "${SCRIPT_ROOT}"; ls -d -1 ./vendor/k8s.io/code-generator 2>/dev/null || echo ../code-generator)}
 
source "${CODEGEN_PKG}/kube_codegen.sh"
 
# generate the code with:
# --output-base    because this script should also be able to run inside the vendor dir of
#                  k8s.io/kubernetes. The output-base is needed for the generators to output into the vendor dir
#                  instead of the $GOPATH directly. For normal projects this can be dropped.
 
kube::codegen::gen_helpers \
    --boilerplate "${SCRIPT_ROOT}/hack/boilerplate.go.txt" \
    "${SCRIPT_ROOT}"/pkg/apis
 
#kube::codegen::gen_helpers \
#    --boilerplate "${SCRIPT_ROOT}/hack/boilerplate.go.txt" \
#    "${SCRIPT_ROOT}"/pkg/apis/config/v1/
#
#kube::codegen::gen_client \
#    --with-watch \
#    --output-pkg github.com/GreatLazyMan/simplescheduler/pkg/apis/example.com/v1 \
#    --output-dir "${SCRIPT_ROOT}/pkg/apis/example.com/v1/" \
#    --boilerplate "${SCRIPT_ROOT}/hack/boilerplate.go.txt" \
#    "${SCRIPT_ROOT}"/pkg/apis \
#
TMPDIR=/tmp/
PLURAL_EXCEPTIONS="Endpoints:Endpoints,ResourceClaimParameters:ResourceClaimParameters,ResourceClassParameters:ResourceClassParameters"

rm -rf "${SCRIPT_ROOT}"/pkg/apis/client/clientset
source "${CODEGEN_PKG}/kube_codegen.sh"

kube::codegen::gen_client \
    "./pkg/apis/crd" \
    --with-watch \
    --output-dir "${TMPDIR}/github.com/GreatLazyMan/simplescheduler/pkg/apis/client" \
    --output-pkg "github.com/GreatLazyMan/simplescheduler/pkg/apis/client" \
    --plural-exceptions ${PLURAL_EXCEPTIONS} \
    --boilerplate "${SCRIPT_ROOT}/hack/boilerplate.go.txt"



#kube::codegen::gen_helpers \
#    --boilerplate "${SCRIPT_ROOT}/hack/boilerplate/boilerplate.go.txt" \
#    "$PWD/apis"
cp -r "${TMPDIR}/github.com/GreatLazyMan/simplescheduler/." ./

kube::codegen::gen_helpers \
    --boilerplate "${SCRIPT_ROOT}/hack/boilerplate.go.txt" \
    "$PWD/pkg"

# https://github.com/kubernetes/kubernetes/pull/124219/files
echo "Generating with register-gen"
if ! which register-gen
then
  GO111MODULE=on go install k8s.io/code-generator/cmd/register-gen
fi

register-gen \
  --go-header-file hack/boilerplate.go.txt \
  --output-file=zz_generated.register.go \
  github.com/GreatLazyMan/simplescheduler/pkg/apis/crd/simpleio/v1

