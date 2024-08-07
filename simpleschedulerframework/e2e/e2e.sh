#!/bin/bash
set -eux
if ! which kind ; then
  curl -Lo ./kind "https://kind.sigs.k8s.io/dl/v0.14.0/kind-$(uname)-amd64"
  chmod +x ./kind
  sudo mv ./kind /usr/local/bin/kind
fi


CLUSTER_NAME="simplescheduler"
CURRENT_DIR=$(dirname $0)

if [ $(sudo kind get clusters  -q  | grep ${CLUSTER_NAME} | wc -l) -eq 0 ] ; then
  sudo kind create cluster --image kindest/node:v1.30.2 --config ${CURRENT_DIR}/kind-example-config.yaml 
fi
sudo docker cp ${CURRENT_DIR}/../deploy ${CLUSTER_NAME}-control-plane:/
sudo kind load docker-image simpleschedulerframework:v0.0.1 --name ${CLUSTER_NAME} 
sudo kind load docker-image docker.io/library/nginx:1.27-alpine3.19 --name ${CLUSTER_NAME}

sudo docker exec ${CLUSTER_NAME}-control-plane kubectl apply -f deploy
