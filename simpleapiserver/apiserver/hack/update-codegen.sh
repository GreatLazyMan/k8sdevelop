#!/usr/bin/env bash

set -o errexit
set -o nounset
set -o pipefail
set -x

SCRIPT_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd -P)/.."

CODEGEN_PKG=${CODEGEN_PKG:-$(cd "${SCRIPT_ROOT}"; ls -d -1 ./vendor/k8s.io/code-generator 2>/dev/null || echo ../code-generator)}

rm -rf "${SCRIPT_ROOT}"/pkg/apis/client/clientset
source "${CODEGEN_PKG}/kube_codegen.sh"

TMPDIR=/tmp/ #${1}
PLURAL_EXCEPTIONS="Endpoints:Endpoints,ResourceClaimParameters:ResourceClaimParameters,ResourceClassParameters:ResourceClassParameters"

kube::codegen::gen_client \
    "./pkg/apis" \
    --with-watch \
    --output-dir "${TMPDIR}/github.com/greatlazyman/apiserver/pkg/apis/client" \
    --output-pkg "github.com/greatlazyman/apiserver/pkg/apis/client" \
    --plural-exceptions ${PLURAL_EXCEPTIONS} \
    --boilerplate "${SCRIPT_ROOT}/hack/boilerplate.go.txt"

cp -r "${TMPDIR}/github.com/greatlazyman/apiserver/." ./

kube::codegen::gen_helpers \
    --boilerplate "${SCRIPT_ROOT}/hack/boilerplate.go.txt" \
    "$PWD/pkg"

# https://github.com/kubernetes/kubernetes/pull/124219/files
echo "Generating with register-gen"
GO111MODULE=on go install k8s.io/code-generator/cmd/register-gen
register-gen \
  --go-header-file hack/boilerplate.go.txt \
  --output-file=zz_generated.register.go \
  github.com/greatlazyman/apiserver/pkg/apis/simple.io/v1beta1

register-gen \
  --go-header-file hack/boilerplate.go.txt \
  --output-file=zz_generated.register.go \
  github.com/greatlazyman/apiserver/pkg/apis/transformation/v1beta1

#register-gen \
#  --go-header-file hack/boilerplate.go.txt \
#  --output-file=zz_generated.register.go \
#  github.com/greatlazyman/apiserver/pkg/apis/simple.io

API_KNOWN_VIOLATIONS_DIR="${API_KNOWN_VIOLATIONS_DIR:-"${SCRIPT_ROOT}/hack"}"
if [[ -n "${API_KNOWN_VIOLATIONS_DIR:-}" ]]; then
    report_filename="${API_KNOWN_VIOLATIONS_DIR}/apiextensions_violation_exceptions.list"
    if [[ "${UPDATE_API_KNOWN_VIOLATIONS:-}" == "true" ]]; then
        update_report="--update-report"
    fi
fi
THIS_PKG="github.com/greatlazyman/apiserver"
GO111MODULE=on go install -mod=readonly k8s.io/kube-openapi/cmd/openapi-gen
kube::codegen::gen_openapi \
    --output-dir "${SCRIPT_ROOT}/pkg/generated/openapi" \
    --output-pkg "${THIS_PKG}/pkg/generated/openapi" \
    --report-filename "${report_filename:-"/dev/null"}" \
    ${update_report:+"${update_report}"} \
    --boilerplate "${SCRIPT_ROOT}/hack/boilerplate.go.txt" \
    "${SCRIPT_ROOT}/pkg/apis"

# TODO
#if ! which go-to-protobuf  
#then
#  GO111MODULE=on go install k8s.io/code-generator/cmd/go-to-protobuf
#  GO111MODULE=on go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
#  GO111MODULE=on go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest
#fi
#
#if ! which protoc
#then
#  wget https://github.com/protocolbuffers/protobuf/releases/download/v3.19.5/protoc-3.19.5-linux-x86_64.zip -O /tmp/protoc.zip
#  pushd /tmp
#  unzip protoc.zip
#  chmod 755 bin/protoc
#  mv bin/protoc /usr/local/bin/
#  popd
#fi
#
#function go_mod_gopath_hack() {
#  local tmp_dir=$(mktemp -d)
#  local module="$(go list -m)"
#
#  local tmp_repo="${tmp_dir}/src/${module}"
#  mkdir -p "$(dirname ${tmp_repo})"
#  ln -s "$PWD" "${tmp_repo}"
#
#  echo "${tmp_dir}"
#}
#export GOPATH=$(go_mod_gopath_hack)
#trap "rm -rf ${GOPATH}; git checkout vendor" EXIT
#
#PKG="github.com/greatlazyman/apiserver"
#go-to-protobuf \
#    --proto-import=${SCRIPT_ROOT}/vendor \
#    --proto-import=${GOPATH}/pkg/mod/k8s.io/kubernetes@v1.32.0/third_party/protobuf \
#    --proto-import=${GOPATH}/pkg/mod/ \
#    --packages github.com/greatlazyman/apiserver/pkg/apis/simple.io/v1alpha1  \
#    --go-header-file hack/boilerplate.go.txt
#
#

#readonly API_PKGS=(
#  "github.com/greatlazyman/apiserver/pkg/apis/simple.io/v1alpha1"
#)
#
#readonly APIMACHINERY_PKGS=(
#  "-k8s.io/api/core/v1"
#  "-k8s.io/api/batch/v1"
#  "-k8s.io/api/rbac/v1"
#  "-k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
#  "-k8s.io/apimachinery/pkg/util/intstr"
#  "-k8s.io/apimachinery/pkg/api/resource"
#  "-k8s.io/apimachinery/pkg/runtime/schema"
#  "-k8s.io/apimachinery/pkg/runtime"
#  "-k8s.io/apimachinery/pkg/apis/meta/v1"
#)
#
## Change working directory to the root of the repository
#work_dir=$(dirname "${0}")
#work_dir=$(readlink -f "${work_dir}/../..")
#
## Create a temporary build directory
#build_dir=$(mktemp -d)
#
#function clean() {
#  echo "Clean up intermediate resources..."
#  rm -r "${build_dir}" || true
#  rm -r "${work_dir}/pkg/api/v1alpha1" || true
#  rm -r "${work_dir}/pkg/api/rbac" || true
#  rm -r "${work_dir}/vendor" || true
#}
#trap 'clean' EXIT
#
#function main() {
#  echo "Change working directory to temporary build directory..."
#  cd "${build_dir}"
#
#  echo "Prepare build directory..."
#  # Initialize dummy module inside build directory
#  go mod init github.com/greatlazyman
#  go work init
#
#  # Copy source files to build directory
#  local build_src_dir
#  build_src_dir="${build_dir}/src/github.com/greatlazyman/apiserver"
#  mkdir -p "${build_src_dir}"
#
#  # Use find to locate and copy .go files, go.mod, and go.sum
#  # while preserving the directory structure
#  find "$work_dir" \( \
#    -name '*.go' -o \
#    -name 'go.mod' -o \
#    -name 'go.sum' \
#  \) -type f | while read -r file; do
#    rel_path="${file#$work_dir/}"
#    dest_file="$build_src_dir/$rel_path"
#    dest_dir=$(dirname "$dest_file")
#    mkdir -p "$dest_dir"
#    cp "$file" "$dest_file"
#  done
#  go work use ./src/github.com/greatlazyman/apiserver
#
#  echo "Vendor dependencies for protobuf code generation..."
#  go work sync
#  go work vendor
#
#  echo "Generate protobuf code from Kubebuilder structs..."
#  go-to-protobuf \
#    --go-header-file="${work_dir}/hack/boilerplate.go.txt" \
#    --packages="$(IFS=, ; echo "${API_PKGS[*]}")" \
#    --apimachinery-packages="$(IFS=, ; echo "${APIMACHINERY_PKGS[*]}")" \
#    --proto-import="${work_dir}/hack/include" \
#    --proto-import="${build_dir}/vendor" \
#    --output-dir="${build_dir}/src"
#}
#
#(
#  # Include local binaries in the PATH
#  export PATH="${work_dir}/hack/bin:${PATH}"
#  main
#)

