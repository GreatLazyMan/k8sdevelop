#!/bin/bash
set -eux
CURRENT_DIR=$(dirname $0)

pushd ${CURRENT_DIR}/../cniplugin/
go build 
mv cniplugin ../deploy/
popd
tar zcf /tmp/simplecni.tar.gz ${CURRENT_DIR}/../../simplecni 
tar zxf /tmp/simplecni.tar.gz -C ${CURRENT_DIR}/../deploy/
pushd ${CURRENT_DIR}/../deploy/
sudo docker build . -t simplecni:v0.0.1
rm -rf simplecni
rm -f cniplugin
popd

sudo docker save simplecni:v0.0.1 -o /tmp/simplecni.tar
sudo ctr -n k8s.io i import /tmp/simplecni.tar
