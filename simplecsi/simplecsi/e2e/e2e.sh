#!/bin/bash
set -x
if ! which kind ; then
curl -Lo ./kind "https://kind.sigs.k8s.io/dl/v0.14.0/kind-$(uname)-amd64"
chmod +x ./kind
sudo mv ./kind /usr/local/bin/kind
fi


CURRENT_DIR=$(dirname $0)
if [ $(sudo kind get clusters  -q  | grep simplecsi | wc -l) -eq 0 ] ; then
  sudo kind create cluster --image kindest/node:v1.24.3 --config ${CURRENT_DIR}/kind-example-config.yaml 
fi
sudo docker cp ${CURRENT_DIR}/../deploy simplecsi-control-plane:/
sudo kind load docker-image simplecsi:v0.0.1 --name simplecsi
sudo kind load docker-image docker.io/library/busybox:latest --name simplecsi
sudo kind load docker-image registry.k8s.io/sig-storage/csi-attacher:v4.7.0                             --name simplecsi 
sudo kind load docker-image registry.k8s.io/sig-storage/csi-external-health-monitor-controller:v0.13.0  --name simplecsi 
sudo kind load docker-image registry.k8s.io/sig-storage/csi-node-driver-registrar:v2.12.0               --name simplecsi 
sudo kind load docker-image registry.k8s.io/sig-storage/csi-provisioner:v5.1.0       --name simplecsi 
sudo kind load docker-image registry.k8s.io/sig-storage/csi-resizer:v1.12.0          --name simplecsi 
sudo kind load docker-image registry.k8s.io/sig-storage/csi-snapshotter:v8.1.0       --name simplecsi 
sudo kind load docker-image registry.k8s.io/sig-storage/livenessprobe:v2.14.0        --name simplecsi 
sudo kind load docker-image aiops-8af5363b.ecis.huhehaote-1.cmecloud.cn/kosmos/openebs/linux-utils:3.3.0  --name simplecsi 

sudo docker exec simplecsi-control-plane kubectl  taint node simplecsi-control-plane  node-role.kubernetes.io/master-
sudo docker exec simplecsi-control-plane kubectl  taint node simplecsi-control-plane  node-role.kubernetes.io/control-plane-
sudo docker exec simplecsi-control-plane kubectl create -f deploy
