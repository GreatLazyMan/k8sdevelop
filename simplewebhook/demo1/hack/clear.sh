#!/bin/bash

export NAMESPACE="default"


kubectl -n ${NAMESPACE} delete secret mywebhook-secret
kubectl -n ${NAMESPACE} delete sa  mywebhook
kubectl -n ${NAMESPACE} delete svc my-webhook
kubectl -n ${NAMESPACE} delete deployment mywebhook
kubectl delete mutatingwebhookconfigurations   my-webhook
kubectl delete validatingwebhookconfigurations my-webhook

