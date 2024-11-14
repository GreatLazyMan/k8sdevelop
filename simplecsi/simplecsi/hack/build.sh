#!/bin/bash
set -eux
CURRENT_DIR=$(dirname $0)

rm -rf  ${CURRENT_DIR}/../build/simplecsi
tar zcf /tmp/simplecsi.tar.gz go.sum go.mod main.go cmd pkg
mkdir ${CURRENT_DIR}/../build/simplecsi
tar zxf /tmp/simplecsi.tar.gz -C ${CURRENT_DIR}/../build/simplecsi/
pushd ${CURRENT_DIR}/../build/
sudo docker build . -t simplecsi:v0.0.1
rm -rf simplecsi
popd

sudo docker save simplecsi:v0.0.1 -o /tmp/simplecsi.tar
sudo ctr -n k8s.io i import /tmp/simplecsi.tar
