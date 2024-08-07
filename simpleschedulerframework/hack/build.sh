#!/bin/bash
set -eux
CURRENT_DIR=$(dirname $0)
CODE_DIR_NAME="simpleschedulerframework"

tar zcf /tmp/${CODE_DIR_NAME}.tar.gz ${CURRENT_DIR}/../../${CODE_DIR_NAME}
tar zxf /tmp/${CODE_DIR_NAME}.tar.gz -C ${CURRENT_DIR}/../deploy/
pushd ${CURRENT_DIR}/../deploy/
sudo docker build . -t ${CODE_DIR_NAME}:v0.0.1  --build-arg CODE_DIR_NAME=${CODE_DIR_NAME}
rm -rf ${CODE_DIR_NAME}
popd

sudo docker save ${CODE_DIR_NAME}:v0.0.1 -o /tmp/${CODE_DIR_NAME}.tar
sudo ctr -n k8s.io i import /tmp/${CODE_DIR_NAME}.tar
