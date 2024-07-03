#!/usr/bin/env bash

set -o errexit
set -o nounset
set -o pipefail

SCRIPT_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd -P)/.."

CODEGEN_PKG=${CODEGEN_PKG:-$(cd "${SCRIPT_ROOT}"; ls -d -1 ./vendor/k8s.io/code-generator 2>/dev/null || echo ../code-generator)}


rm -rf "${SCRIPT_ROOT}"/pkg/apis/client/clientset
source "${CODEGEN_PKG}/kube_codegen.sh"

TMPDIR=/tmp/ #${1}
PLURAL_EXCEPTIONS="Endpoints:Endpoints,ResourceClaimParameters:ResourceClaimParameters,ResourceClassParameters:ResourceClassParameters"

kube::codegen::gen_client \
    "./pkg/apis" \
    --with-watch \
    --output-dir "${TMPDIR}/github.com/GreatLazyMan/simplecontroller/pkg/apis/client" \
    --output-pkg "github.com/GreatLazyMan/simplecontroller/pkg/apis/client" \
    --plural-exceptions ${PLURAL_EXCEPTIONS} \
    --boilerplate "${SCRIPT_ROOT}/hack/boilerplate/boilerplate.go.txt"

#kube::codegen::gen_client \
#    "./pkg/apis" \
#    --with-watch \
#    --output-dir "${TMPDIR}/github.com/GreatLazyMan/simplecontroller/pkg/apis/apiextensions-client" \
#    --output-pkg "github.com/GreatLazyMan/simplecontroller/pkg/apis/apiextensions-client" \
#    --plural-exceptions ${PLURAL_EXCEPTIONS} \
#    --boilerplate "${SCRIPT_ROOT}/hack/boilerplate/boilerplate.go.txt"
#
#kube::codegen::gen_client \
#    "./pkg/apis" \
#    --with-watch \
#    --output-dir "${TMPDIR}/github.com/GreatLazyMan/simplecontroller/pkg/apis/client" \
#    --output-pkg "github.com/GreatLazyMan/simplecontroller/pkg/apis/client" \
#    --plural-exceptions ${PLURAL_EXCEPTIONS} \
#    --boilerplate "${SCRIPT_ROOT}/hack/boilerplate/boilerplate.go.txt"

cp -r "${TMPDIR}/github.com/GreatLazyMan/simplecontroller/." ./

#kube::codegen::gen_helpers \
#    --boilerplate "${SCRIPT_ROOT}/hack/boilerplate/boilerplate.go.txt" \
#    "$PWD/apis"

kube::codegen::gen_helpers \
    --boilerplate "${SCRIPT_ROOT}/hack/boilerplate/boilerplate.go.txt" \
    "$PWD/pkg"

# https://github.com/kubernetes/kubernetes/pull/124219/files
echo "Generating with register-gen"
GO111MODULE=on go install k8s.io/code-generator/cmd/register-gen
register-gen \
  --go-header-file hack/boilerplate/boilerplate.go.txt \
  --output-file=zz_generated.register.go \
  github.com/GreatLazyMan/simplecontroller/pkg/apis/simplecontroller/v1
