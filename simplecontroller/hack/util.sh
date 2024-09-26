#!/usr/bin/env bash

set -o errexit
set -o nounset
set -o pipefail

function util::get_version() {
  git describe --tags --dirty --always
}

function util::version_ldflags() {
  # set GIT_VERSION from param
  GIT_VERSION=${1:-}
  # If GIT_VERSION is not provided, use util::get_version
  if [ -z "$GIT_VERSION" ]; then
    GIT_VERSION=$(util::get_version)
  fi
  #GIT_VERSION=$(util::get_version)
  GIT_COMMIT_HASH=$(git rev-parse HEAD)
  if git_status=$(git status --porcelain 2>/dev/null) && [[ -z ${git_status} ]]; then
    GIT_TREESTATE="clean"
  else
    GIT_TREESTATE="dirty"
  fi
  BUILDDATE=$(date -u +'%Y-%m-%dT%H:%M:%SZ')
  LDFLAGS="-X ${GITREGISTRY}/version.gitVersion=${GIT_VERSION} \
                        -X ${GITREGISTRY}/version.gitCommit=${GIT_COMMIT_HASH} \
                        -X ${GITREGISTRY}/version.gitTreeState=${GIT_TREESTATE} \
                        -X ${GITREGISTRY}/version.buildDate=${BUILDDATE}"
  echo $LDFLAGS
}
