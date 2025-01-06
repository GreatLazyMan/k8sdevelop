package main

import (
	"context"
	"fmt"
	"time"

	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

func main() {
	// 创建 Kubernetes 客户端配置
	config, err := rest.InClusterConfig()
	if err != nil {
		panic(err.Error())
	}

	// 创建 Kubernetes 客户端
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		panic(err.Error())
	}

	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			// 构建HTTP请求
			request := clientset.CoreV1().RESTClient().Get().
				AbsPath("/apis/test.k8s.io/v1beta1").
				Do(context.TODO())
			// 发送请求并获取响应
			raw, err := request.Raw()
			if err != nil {
				panic(err.Error())
			}
			fmt.Printf("Response: %s\n", string(raw))
		}
	}
}
