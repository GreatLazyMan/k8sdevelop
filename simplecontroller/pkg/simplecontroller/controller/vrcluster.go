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

type RCCluster struct {
	Kind string
	Name string
}

type RClusterController struct {
	queue workqueue.RateLimitingInterface

	rcindexer cache.Indexer
	vcindexer cache.Indexer

	rcinformer cache.Controller
	vcinformer cache.Controller

	rclister listers.RealClusterLister
	vclister listers.VirtualClusterLister

	Clientset versioned.Interface
	ctx       context.Context
}

func (c *RClusterController) Run(threadiness int, stopCh <-chan struct{}) {
	defer runtime.HandleCrash()

	defer c.queue.ShutDown()
	klog.Info("Starting RealClusters controller")

	go c.rcinformer.Run(stopCh)
	if !cache.WaitForCacheSync(stopCh, c.rcinformer.HasSynced) {
		runtime.HandleError(fmt.Errorf("Timed out waiting for caches to sync"))
		return
	}

	go c.vcinformer.Run(stopCh)
	if !cache.WaitForCacheSync(stopCh, c.vcinformer.HasSynced) {
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

	err := c.syncToStdout(key.(RCCluster))
	c.handleErr(err, key)
	return true
}

func (c *RClusterController) syncToStdout(key RCCluster) error {
	klog.Infof("proecess: %v", key)
	var obj interface{}
	var exists bool
	var err error

	if key.Kind == simplecontrollerv1.VirtualClusterKind {
		obj, exists, err = c.rcindexer.GetByKey(key.Name)
	} else if key.Kind == simplecontrollerv1.RealClusterKind {
		obj, exists, err = c.vcindexer.GetByKey(key.Name)
	} else {
		return fmt.Errorf("strang key!")
	}
	if err != nil {
		klog.Errorf("Fetching object with key %s from store failed with %v", key, err)
		return err
	}

	var name string
	switch v := obj.(type) {
	case *simplecontrollerv1.RealCluster:
		name = v.GetName()
		klog.Info("receive RealCluster ptr")
	case *simplecontrollerv1.VirtualCluster:
		name = v.GetName()
		klog.Info("receive VirtualCluster ptr")
	}

	if !exists {
		klog.Infof("obj %s does not exists anymore\n", key)
	} else {
		klog.Infof("Sync/Add/Update for obj %s\n", name)
	}
	return nil
}

func (c *RClusterController) handleErr(err error, key interface{}) {
	if err == nil {
		c.queue.Forget(key)
		return
	}
	if c.queue.NumRequeues(key) < 5 {
		klog.Infof("Error syncing obj %v: %v", key, err)
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
				klog.Infof("add real cluster: %v", key)
				queue.Add(RCCluster{
					Name: key,
					Kind: simplecontrollerv1.RealClusterKind,
				})
			} else {
				klog.Error(err)
			}
		},
		UpdateFunc: func(oldObj, newObj interface{}) {
			key, err := cache.MetaNamespaceKeyFunc(newObj)
			if err == nil {
				klog.Infof("update real cluster: %v", key)
				queue.Add(RCCluster{
					Name: key,
					Kind: simplecontrollerv1.RealClusterKind,
				})
			} else {
				klog.Error(err)
			}
		},
		DeleteFunc: func(obj interface{}) {
			key, err := cache.DeletionHandlingMetaNamespaceKeyFunc(obj)
			if err == nil {
				klog.Infof("delete real cluster: %v", key)
				queue.Add(RCCluster{
					Name: key,
					Kind: simplecontrollerv1.RealClusterKind,
				})
			} else {
				klog.Error(err)
			}
		},
	})
	if err != nil {
		klog.Errorf("init informers err: %v", err)
		panic(err)
	}

	vcInformer := informerFactory.Greatlazyman().V1().VirtualClusters().Informer()
	vcLister := informerFactory.Greatlazyman().V1().VirtualClusters().Lister()

	_, err = vcInformer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			key, err := cache.MetaNamespaceKeyFunc(obj)
			if err == nil {
				klog.Infof("add virtual cluster: %v", key)
				queue.Add(RCCluster{
					Name: key,
					Kind: simplecontrollerv1.VirtualClusterKind,
				})
			} else {
				klog.Error(err)
			}
		},
		UpdateFunc: func(oldObj, newObj interface{}) {
			key, err := cache.MetaNamespaceKeyFunc(newObj)
			if err == nil {
				klog.Infof("update virtual cluster: %v", key)
				queue.Add(RCCluster{
					Name: key,
					Kind: simplecontrollerv1.VirtualClusterKind,
				})
			} else {
				klog.Error(err)
			}
		},
		DeleteFunc: func(obj interface{}) {
			key, err := cache.DeletionHandlingMetaNamespaceKeyFunc(obj)
			if err == nil {
				klog.Infof("delete virtual cluster: %v", key)
				queue.Add(RCCluster{
					Name: key,
					Kind: simplecontrollerv1.VirtualClusterKind,
				})
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
		queue:      queue,
		rcindexer:  rcInformer.GetIndexer(),
		vcindexer:  vcInformer.GetIndexer(),
		rcinformer: rcInformer,
		rclister:   rcLister,
		vcinformer: vcInformer,
		vclister:   vcLister,
		Clientset:  clientSet,
		ctx:        ctx,
	}
	return *controller
}

func (c *RClusterController) Start() {
	c.Run(1, c.ctx.Done())
}
