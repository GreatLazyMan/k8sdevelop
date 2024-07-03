package controller

import (
	"context"
	"fmt"
	"time"

	"k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/klog/v2"

	versioned "github.com/GreatLazyMan/simplecontroller/pkg/apis/client/clientset/versioned"
	factory "github.com/GreatLazyMan/simplecontroller/pkg/apis/client/informers/externalversions"
	listers "github.com/GreatLazyMan/simplecontroller/pkg/apis/client/listers/simplecontroller/v1"
	simplecontrollerv1 "github.com/GreatLazyMan/simplecontroller/pkg/apis/simplecontroller/v1"
)

type RClusterController struct {
	indexer   cache.Indexer
	queue     workqueue.RateLimitingInterface
	informer  cache.Controller
	lister    listers.RealClusterLister
	Clientset versioned.Interface
	ctx       context.Context
}

func (c *RClusterController) Run(threadiness int, stopCh chan struct{}) {
	defer runtime.HandleCrash()

	defer c.queue.ShutDown()
	klog.Info("Starting RealClusters controller")

	go c.informer.Run(stopCh)

	if !cache.WaitForCacheSync(stopCh, c.informer.HasSynced) {
		runtime.HandleError(fmt.Errorf("Timed out waiting for caches to sync"))
		return
	}

	for i := 0; i < threadiness; i++ {
		go wait.Until(c.runWorker, time.Second, stopCh)
	}

	<-stopCh
	klog.Info("Stopping RealClusters controller")
}

func (c *RClusterController) runWorker() {
	for c.processNextItem() {
	}
}

func (c *RClusterController) processNextItem() bool {
	key, quit := c.queue.Get()
	if quit {
		return false
	}
	defer c.queue.Done(key)

	err := c.syncToStdout(key.(string))
	c.handleErr(err, key)
	return true
}

func (c *RClusterController) syncToStdout(key string) error {
	klog.Infof("proecess: %v", key)
	obj, exists, err := c.indexer.GetByKey(key)
	if _, ok := obj.(*simplecontrollerv1.RealCluster); !ok {
		return fmt.Errorf("strange key")
	}
	if err != nil {
		klog.Errorf("Fetching object with key %s from store failed with %v", key, err)
		return err
	}
	if !exists {
		klog.Infof("RealClusters %s does not exists anymore\n", key)
	} else {
		klog.Infof("Sync/Add/Update for RealClusters %s\n", obj.(*simplecontrollerv1.RealCluster).GetName())
	}
	return nil
}

func (c *RClusterController) handleErr(err error, key interface{}) {
	if err == nil {
		c.queue.Forget(key)
		return
	}
	if c.queue.NumRequeues(key) < 5 {
		klog.Infof("Error syncing ds %v: %v", key, err)
		c.queue.AddRateLimited(key)
		return
	}
	c.queue.Forget(key)
	runtime.HandleError(err)
	klog.Infof("Dropping ds %q out of the queue: %v", key, err)
}

func NewRClusterReconsiler(ctx context.Context, clientSet versioned.Interface) RClusterController {
	queue := workqueue.NewRateLimitingQueue(workqueue.DefaultControllerRateLimiter())
	informerFactory := factory.NewSharedInformerFactoryWithOptions(clientSet, 0) //informers.WithNamespace("kube-system"))
	rcInformer := informerFactory.Greatlazyman().V1().RealClusters().Informer()
	rcLister := informerFactory.Greatlazyman().V1().RealClusters().Lister()

	_, err := rcInformer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			key, err := cache.MetaNamespaceKeyFunc(obj)
			if err == nil {
				klog.V(10).Infof("add real cluster: %v", key)
				queue.Add(key)
			} else {
				klog.Error(err)
			}
		},
		UpdateFunc: func(oldObj, newObj interface{}) {
			key, err := cache.MetaNamespaceKeyFunc(newObj)
			if err == nil {
				klog.V(10).Infof("update real cluster: %v", key)
				queue.Add(key)
			} else {
				klog.Error(err)
			}
		},
		DeleteFunc: func(obj interface{}) {
			key, err := cache.DeletionHandlingMetaNamespaceKeyFunc(obj)
			if err == nil {
				klog.V(10).Infof("delete real cluster: %v", key)
				queue.Add(key)
			} else {
				klog.Error(err)
			}
		},
	})
	if err != nil {
		klog.Errorf("init informers err: %v", err)
		panic(err)
	}

	controller := &RClusterController{
		indexer:   rcInformer.GetIndexer(),
		queue:     queue,
		informer:  rcInformer,
		lister:    rcLister,
		Clientset: clientSet,
		ctx:       ctx,
	}
	return *controller
}

func (c *RClusterController) Start() {
	stopCh := make(chan struct{})
	defer close(stopCh)
	c.Run(1, stopCh)
}
