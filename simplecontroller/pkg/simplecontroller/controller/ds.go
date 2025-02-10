package controller

import (
	"context"
	"fmt"
	"time"

	"github.com/GreatLazyMan/simplecontroller/pkg/utils"
	appv1 "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/klog/v2"
)

// 定义控制器, 控制器包括 informer、workqueue、indexer
type DaemonsetController struct {
	indexer   cache.Indexer
	queue     workqueue.RateLimitingInterface
	informer  cache.Controller
	Clientset kubernetes.Interface
	ctx       context.Context
}

// Run 启动 informer, 以及开启协程消费 workqueue 中的元素
func (c *DaemonsetController) Run(threadiness int, stopCh <-chan struct{}) {
	// 错误处理
	defer runtime.HandleCrash()

	// 停止控制器后关掉队列
	defer c.queue.ShutDown()
	klog.Info("Starting DaemonSet controller")

	// 启动 informer, 如果使用factory.Start(stopper)则是启动所有informer，底层也是调用informer.Run(stopCh)
	go c.informer.Run(stopCh)

	// 等待所有相关的缓存同步，然后再开始处理队列中的项目
	if !cache.WaitForCacheSync(stopCh, c.informer.HasSynced) {
		runtime.HandleError(fmt.Errorf("Timed out waiting for caches to sync"))
		return
	}

	// 从协程池中运行消费者
	for i := 0; i < threadiness; i++ {
		go wait.Until(c.runWorker, time.Second, stopCh)
	}

	<-stopCh
	klog.Info("Stopping DaemonSet controller")
}

// 循环处理元素
func (c *DaemonsetController) runWorker() {
	for c.processNextItem() {
	}
}

// 处理元素
func (c *DaemonsetController) processNextItem() bool {
	// 等到工作队列中有一个新元素, 如果没有元素会阻塞
	key, quit := c.queue.Get()
	if quit {
		return false
	}
	// 告诉队列我们已经完成了处理此 key 的操作
	// 这将为其他 worker 解锁该 key
	// 这将确保安全的并行处理，因为永远不会并行处理具有相同 key 的两个ds
	defer c.queue.Done(key)

	// 调用包含业务逻辑的方法
	err := c.syncToStdout(key.(string))
	// 如果在执行业务逻辑期间出现错误，则处理错误
	c.handleErr(err, key)
	return true
}

// syncToStdout 是控制器的业务逻辑实现
// 在此控制器中，它只是将有关 ds 的信息打印到 stdout
// 如果发生错误，则简单地返回错误
// 此外重试逻辑不应成为业务逻辑的一部分。
func (c *DaemonsetController) syncToStdout(key string) error {
	klog.Infof("proecess: %v", key)
	// 从 indexer 获取 key 对应的对象
	obj, exists, err := c.indexer.GetByKey(key)
	if err != nil {
		klog.Errorf("Fetching object with key %s from store failed with %v", key, err)
		return err
	}
	ds := obj.(*appv1.DaemonSet)
	objs, err := c.indexer.ByIndex(utils.NamespaceIndexName, ds.Namespace)
	if err != nil {
		for _, objele := range objs {
			dsele := objele.(*appv1.DaemonSet)
			klog.Infof("Namespace %s has daemonset %s", ds.Namespace, dsele.Name)
		}
	}
	if !exists {
		klog.Infof("DaemonSets %s does not exists anymore\n", key)
	} else {
		klog.Infof("Sync/Add/Update for DaemonSets %s\n", obj.(*appv1.DaemonSet).GetName())
	}
	return nil
}

// 检查是否发生错误，并确保我们稍后重试
func (c *DaemonsetController) handleErr(err error, key interface{}) {
	if err == nil {
		// 忘记每次成功同步时 key, 下次不会再被处理, 除非事件被 resync
		c.queue.Forget(key)
		return
	}
	//如果出现问题，此控制器将重试5次
	if c.queue.NumRequeues(key) < 5 {
		klog.Infof("Error syncing ds %v: %v", key, err)
		// 重新加入 key 到限速队列
		// 根据队列上的速率限制器和重新入队历史记录，稍后将再次处理该 key
		c.queue.AddRateLimited(key)
		return
	}
	c.queue.Forget(key)
	// 多次重试，我们也无法成功处理该key
	runtime.HandleError(err)
	klog.Infof("Dropping ds %q out of the queue: %v", key, err)
}

func NewDsReconsiler(ctx context.Context, clientSet kubernetes.Interface) DaemonsetController {
	// 初始化 workqueue, 使用限速队列
	queue := workqueue.NewRateLimitingQueue(workqueue.DefaultControllerRateLimiter())
	// 初始化 sharedInformer
	// 然后为了测试效果，将 Informer resync 的周期设置为 0，resync 设置为 0 表示不会将 Indexer 的数据重新同步到 Deltafifo 中。
	// 如果设置 resync 的话，则会定期出现 update 事件，因为 resync 的元素都标记为 update 类型了
	informerFactory := informers.NewSharedInformerFactoryWithOptions(clientSet, 0) //informers.WithNamespace("kube-system"))
	dsInformer := informerFactory.Apps().V1().DaemonSets().Informer()
	// 自定义 Indexer
	dsInformer.AddIndexers(cache.Indexers{
		utils.NamespaceIndexName: utils.NamespaceIndexFunc,
	})

	// 注册回调函数到 informer
	_, err := dsInformer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		// 元素新增时，直接将事件元素添加到 Workqueue
		AddFunc: func(obj interface{}) {
			key, err := cache.MetaNamespaceKeyFunc(obj)
			if err == nil {
				klog.V(10).Infof("add ds: %v", key)
				queue.Add(key)
			} else {
				klog.Error(err)
			}
		},
		// 元素更新时，直接将事件元素添加到 Workqueue
		UpdateFunc: func(oldObj, newObj interface{}) {
			key, err := cache.MetaNamespaceKeyFunc(newObj)
			if err == nil {
				klog.V(10).Infof("update ds: %v", key)
				queue.Add(key)
			} else {
				klog.Error(err)
			}
		},
		// 元素删除时，直接将事件元素添加到 Workqueue
		DeleteFunc: func(obj interface{}) {
			key, err := cache.DeletionHandlingMetaNamespaceKeyFunc(obj)
			if err == nil {
				klog.V(10).Infof("delete ds: %v", key)
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

	// 初始化控制器
	controller := &DaemonsetController{
		indexer:   dsInformer.GetIndexer(),
		queue:     queue,
		informer:  dsInformer,
		Clientset: clientSet,
		ctx:       ctx,
	}
	return *controller
}

func (c *DaemonsetController) Start() {
	// start controller
	c.Run(1, c.ctx.Done())
}
