package main

import (
	"context"
	"fmt"
	"time"

	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

func main() {
	// 创建 Kubernetes 客户端配置
	config, err := rest.InClusterConfig()
	if err != nil {
		fmt.Printf("InClusterConfig err: %v\n", err)
		// 如果在集群内运行时发生错误，尝试从 kubeconfig 文件加载配置
		config, err = clientcmd.BuildConfigFromFlags("", "/path/to/your/kubeconfig")
		if err != nil {
			panic(err.Error())
		}
	}

	// 创建 Kubernetes 客户端
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		panic(err.Error())
	}

	//https://stackoverflow.com/questions/56262736/how-to-invoke-the-pod-proxy-verb-using-the-kubernetes-go-client/57913431
	// 设置代理 URL
	// https://10.96.0.1:443/api/v1/namespaces/default/services/pythontest:8000/proxy/status?c=d -H 'a:b'
	proxy := clientset.CoreV1().RESTClient().Get().
		Namespace("default").
		Resource("services").
		Name("pythontest:8000").
		SubResource("proxy").
		Suffix("status").
		SetHeader("a", "b").
		Param("c", "d")
	fmt.Println(proxy.URL())

	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			res := proxy.Do(context.Background())
			rawbody, err := res.Raw()
			if err != nil {
				fmt.Printf("http get err: %v\n", err)
				continue
			}
			fmt.Println(string(rawbody))
		}
	}
}
