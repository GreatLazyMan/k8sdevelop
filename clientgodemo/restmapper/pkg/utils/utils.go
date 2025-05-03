package utils

import (
	"os"
	"path/filepath"

	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/restmapper"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/util/homedir"
)

type K8sClient struct {
	*rest.Config
	*kubernetes.Clientset
	*dynamic.DynamicClient
	meta.RESTMapper
	informers.SharedInformerFactory
}

func InitK8sRestConfig() (*rest.Config, error) {
	kuebconfig := filepath.Join(homedir.HomeDir(), ".kube", "config")
	var restConfig *rest.Config
	if _, err := os.Stat(kuebconfig); err != nil {
		restConfig, err = clientcmd.BuildConfigFromFlags("", kuebconfig)
		if err != nil {
			return nil, err
		}
	} else {
		restConfig, err = rest.InClusterConfig()
		if err != nil {
			return nil, err
		}
	}
	restConfig.QPS = 100
	restConfig.Burst = 50
	return restConfig, nil

}

func InitClientSet(config *rest.Config) (*kubernetes.Clientset, error) {
	clientSet, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, err
	}
	return clientSet, nil
}

func InitDynamicClient(config *rest.Config) (*dynamic.DynamicClient, error) {
	dynamicClient, err := dynamic.NewForConfig(config)
	if err != nil {
		return nil, err
	}
	return dynamicClient, nil
}

func InitRestMapper(clientSet *kubernetes.Clientset) (meta.RESTMapper, error) {
	gr, err := restmapper.GetAPIGroupResources(clientSet.Discovery())
	if err != nil {
		return nil, err
	}
	mapper := restmapper.NewDiscoveryRESTMapper(gr)

	return mapper, nil
}

func InitInformer(clientSet *kubernetes.Clientset) informers.SharedInformerFactory {
	fact := informers.NewSharedInformerFactory(clientSet, 0) //创建通用informer工厂

	informer := fact.Core().V1().Pods()
	informer.Informer().AddEventHandler(&cache.ResourceEventHandlerFuncs{})

	ch := make(chan struct{})
	fact.Start(ch)
	fact.WaitForCacheSync(ch)

	return fact
}

func InitK8sClient() *K8sClient {
	k8sClient := &K8sClient{}
	config, err := InitK8sRestConfig()
	if err != nil {
		panic(err)
	}
	k8sClient.Config = config

	clientSet, err := InitClientSet(config)
	if err != nil {
		panic(err)
	}
	k8sClient.Clientset = clientSet

	dClientSet, err := InitDynamicClient(config)
	if err != nil {
		panic(err)
	}
	k8sClient.DynamicClient = dClientSet

	mapper, err := InitRestMapper(clientSet)
	if err != nil {
		panic(err)
	}
	k8sClient.RESTMapper = mapper

	informer := InitInformer(clientSet)
	k8sClient.SharedInformerFactory = informer

	return k8sClient
}
