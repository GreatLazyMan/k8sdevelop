#!/bin/bash
set -eux
if ! which kind ; then
curl -Lo ./kind "https://kind.sigs.k8s.io/dl/v0.14.0/kind-$(uname)-amd64"
chmod +x ./kind
sudo mv ./kind /usr/local/bin/kind
fi


CURRENT_DIR=$(dirname $0)
if [ $(sudo kind get clusters  -q  | grep simplecni | wc -l) -eq 0 ] ; then
  sudo kind create cluster --image kindest/node:v1.24.3 --config ${CURRENT_DIR}/kind-example-config.yaml 
fi
sudo docker cp ${CURRENT_DIR}/../deploy simplecni-control-plane:/
sudo kind load docker-image simplecni:v0.0.1 --name simplecni
sudo docker exec simplecni-control-plane kubectl create -f deploy
