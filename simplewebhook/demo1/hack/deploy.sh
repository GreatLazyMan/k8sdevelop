#!/bin/bash

export NAMESPACE="default"
SCRIPT_DIR=$(dirname "$(readlink -f "$0")")

.  ${SCRIPT_DIR}/crt.sh

docker build -f ${SCRIPT_DIR}/../Dockerfile . -t my-webhook:latest
docker save my-webhook:latest -o /tmp/my-webhook.tar
sudo /usr/bin/ctr -n k8s.io i import /tmp/my-webhook.tar

kubectl -n ${NAMESPACE} delete secret mywebhook-secret
kubectl -n ${NAMESPACE} create secret generic mywebhook-secret \
  --from-file=cert.pem=/tmp/webhook.crt  \
  --from-file=key.pem=/tmp/webhook.key
cat ${SCRIPT_DIR}/../deploy/sa.yaml | envsubst | kubectl apply -f -
kubectl -n ${NAMESPACE} create sa  mywebhook
kubectl -n ${NAMESPACE} apply -f ${SCRIPT_DIR}/../deploy/deploy.yaml  


export CABundle=$(cat /tmp/ca.crt |base64 -w 0)
cat ${SCRIPT_DIR}/../deploy/webhook.yaml | envsubst  | kubectl apply -f -
