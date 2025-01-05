测试，启动服务    

`bash -c "echo ok > /status && python3 -m http.server"`

k8s master节点上启动服务 `kubectl  proxy --address 0.0.0.0`

测试 `curl 127.0.0.1:8001/api/v1/namespaces/default/services/pythontest:8000/proxy/statu`

返回 ok

或者使用命令 `kubectl get --raw "/api/v1/namespaces/default/services/pythontest:8000/proxy/status"`

返回 ok

参考：
https://stackoverflow.com/questions/56262736/how-to-invoke-the-pod-proxy-verb-using-the-kubernetes-go-client/57913431
https://stackoverflow.com/questions/74590850/how-can-i-use-kubernetes-api-to-get-the-metrics-of-a-cluster-like-memory-in-gola
