#!/usr/bin/env bash

set -o errexit
set -o nounset
set -o pipefail

# This script holds docker related functions.
# You can set the platform to build with BUILD_PLATFORMS, with format: `<os>/<arch>`
# When `OUTPUT_TYPE=docker` is set, `BUILD_PLATFORMS` cannot be set with multi platforms.
# See: https://github.com/docker/buildx/issues/59
#
# Usage:
#   hack/docker.sh <target>
# Args:
#   $1:              target to build
# Environments:
#   BUILD_PLATFORMS:  platforms to build. You can set one or more platforms separated by comma.
#                     e.g.: linux/amd64,linux/arm64
#   OUTPUT_TYPE       Destination to save image(`docker`/`registry`/`local,dest=path`, default is `docker`).
#   REGISTRY          image registry
#   VERSION           image version
#   DOCKER_BUILD_ARGS additional arguments to the docker build command
# Examples:
#   hack/docker.sh clusterlink-controllermanager
#   BUILD_PLATFORMS=linux/amd64 hack/docker.sh clusterlink-controllermanager
#   OUTPUT_TYPE=registry BUILD_PLATFORMS=linux/amd64,linux/arm64 hack/docker.sh clusterlink-controllermanager
#   DOCKER_BUILD_ARGS="--build-arg https_proxy=${https_proxy}" hack/docker.sh clusterlink-controllermanager

REPO_ROOT=$(dirname "${BASH_SOURCE[0]}")/..
source "${REPO_ROOT}/hack/util.sh"

REGISTRY=${REGISTRY:-"docker.io/simpleio"}
VERSION=${VERSION:="unknown"}
DOCKER_BUILD_ARGS=${DOCKER_BUILD_ARGS:-}

function build_images() {
  local -r target=$1
  local -r output_type=${OUTPUT_TYPE:-docker}
  local platforms="${BUILD_PLATFORMS:-"$(util:host_platform)"}"
  local dockerfile="Dockerfile"

  # Preferentially use `docker build`. If we are building multi platform,
  # or cross building, change to `docker buildx build`
  cross=$(isCross "${platforms}")
  if [[ "${cross}" == "true" ]]; then
    build_cross_image "${output_type}" "${target}" "${platforms}" 
  else
    build_local_image "${output_type}" "${target}" "${platforms}"
  fi
}

function build_local_image() {
  local -r output_type=$1
  local -r target=$2
  local -r platform=$3

  image_name="${REGISTRY}/${target}:${VERSION}"
  image_name=$(echo $image_name | tr '[:upper:]' '[:lower:]')

  echo "Building image for ${platform}: ${image_name}"
  set -x
  # https://stackoverflow.com/questions/20481225/how-can-i-use-a-local-image-as-the-base-image-with-a-dockerfile
  # DOCKER_BUILDKIT=0  docker build -t YOUR_TAG --pull=false .
  sudo docker build --build-arg BINARY="${target}" \
          ${DOCKER_BUILD_ARGS} \
          --tag "${image_name}" \
          --file "${REPO_ROOT}/build/${dockerfile}" \
          "${REPO_ROOT}/_output/bin/${platform}"
  set +x

  if [[ "$output_type" == "registry" ]]; then
    sudo docker push "${image_name}"
  fi
}

function build_cross_image() {
  local -r output_type=$1
  local -r target=$2
  local -r platforms=$3

  local -r image_name="${REGISTRY}/${target}:${VERSION}"

  echo "Cross building image for ${platforms}: ${image_name}"
  set -x
  sudo docker buildx build --output=type="${output_type}" \
          --platform "${platforms}" \
          --build-arg BINARY="${target}" \
          ${DOCKER_BUILD_ARGS} \
          --tag "${image_name}" \
          --file "${REPO_ROOT}/build/buildx.${dockerfile}" \
          "${REPO_ROOT}/_output/bin"
  set +x
}

function isCross() {
  local platforms=$1

  IFS="," read -ra platform_array <<< "${platforms}"
  if [[ ${#platform_array[@]} -ne 1 ]]; then
    echo true
    return
  fi

  local -r arch=${platforms##*/}
  if [[ "$arch" == $(go env GOHOSTARCH) ]]; then
    echo false
  else
    echo true
  fi
}

build_images "$@"
