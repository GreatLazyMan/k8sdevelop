k8s master节点测试`kubectl   get --raw /apis/test.k8s.io/v1beta1`

或者

测试 `curl -k https://10.96.3.19:443` 

返回 {"status":"success","result":"nginx json"}


参考：

https://mp.weixin.qq.com/s/Te3vqPgUIQH1_Dheo_X6LA
