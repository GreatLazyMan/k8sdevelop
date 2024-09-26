/*
Copyright 2021 Loggie Authors

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package controller

import (
	"fmt"
	"time"

	"github.com/GreatLazyMan/simplecontroller/pkg/simplecontroller/constants"
	klog "k8s.io/klog/v2"

	"k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"

	typedcorev1 "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/client-go/tools/record"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	corev1Informers "k8s.io/client-go/informers/core/v1"
	corev1Listers "k8s.io/client-go/listers/core/v1"
)

const (
	EventService       = "svc"
	EventEndpoints     = "ep"
	ByindexBySvcLabels = "Labels"
	ReasonSuccess      = "syncSuccess"
	MessageSyncSuccess = "Sync %s %v success"
	ReasonFailed       = "syncFailed"
	MessageSyncFailed  = "Sync %s failed: %s"
)

// Element the item add to queue
type Element struct {
	Type string `json:"type"` // resource type, eg: pod
	Key  string `json:"key"`  // MetaNamespaceKey, format: <namespace>/<name>
}

type Controller struct {
	workqueue workqueue.RateLimitingInterface

	kubeClientset kubernetes.Interface

	svcsLister corev1Listers.ServiceLister
	epsLister  corev1Listers.EndpointsLister
	svcIndexer cache.Indexer

	record record.EventRecorder
}

func indexBySvcLabels(obj interface{}) ([]string, error) {
	svc, ok := obj.(*corev1.Service)
	if !ok {
		return []string{}, nil
	}
	if len(svc.Spec.Selector) == 0 {
		return []string{}, nil
	}
	return []string{svc.ObjectMeta.Labels["app"]}, nil
}

func NewController(
	kubeClientset kubernetes.Interface,
	svcInformer corev1Informers.ServiceInformer,
	epInformer corev1Informers.EndpointsInformer,
) *Controller {

	klog.Info("Creating event broadcaster")
	eventBroadcaster := record.NewBroadcaster()
	eventBroadcaster.StartRecordingToSink(&typedcorev1.EventSinkImpl{Interface: kubeClientset.CoreV1().Events("")})
	recorder := eventBroadcaster.NewRecorder(scheme.Scheme,
		corev1.EventSource{Component: constants.SimplecontrollerControllerName})

	indexer := cache.Indexers{
		ByindexBySvcLabels: indexBySvcLabels,
	}
	var controller *Controller
	controller = &Controller{
		workqueue:     workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "logConfig"),
		kubeClientset: kubeClientset,
		svcsLister:    svcInformer.Lister(),
		epsLister:     epInformer.Lister(),
		record:        recorder,
		svcIndexer:    svcInformer.Informer().GetIndexer(),
	}

	klog.Info("Setting up event handlers")

	svcInformer.Informer().AddIndexers(indexer)
	svcInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			svc := obj.(*corev1.Service)
			if svc.Spec.Selector == nil {
				return
			}
			controller.enqueue(obj, EventService)
		},
		UpdateFunc: func(old, new interface{}) {
			newSvc := new.(*corev1.Service)
			oldSvc := old.(*corev1.Service)
			if newSvc.ResourceVersion == oldSvc.ResourceVersion {
				return
			}
			if newSvc.Spec.Selector == nil {
				return
			}
			controller.enqueue(new, EventService)
		},
		DeleteFunc: func(obj interface{}) {
			controller.enqueueForDelete(obj, EventService)
		},
	})

	epInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			ep := obj.(*corev1.Endpoints)
			if len(ep.Subsets) == 0 {
				return
			}
			controller.enqueue(obj, EventEndpoints)
		},
		UpdateFunc: func(old, new interface{}) {
			newEp := new.(*corev1.Endpoints)
			oldEp := old.(*corev1.Endpoints)
			equal := equality.Semantic.DeepEqual(newEp, oldEp)
			if !equal {
				controller.enqueue(new, EventEndpoints)
			}
		},
		DeleteFunc: func(obj interface{}) {
			controller.enqueueForDelete(obj, EventEndpoints)
		},
	})

	return controller
}

func (c *Controller) enqueue(obj interface{}, eleType string) {
	var key string
	var err error
	if key, err = cache.MetaNamespaceKeyFunc(obj); err != nil {
		utilruntime.HandleError(err)
		return
	}
	e := Element{
		Type: eleType,
		Key:  key,
	}
	c.workqueue.Add(e)
}

func (c *Controller) enqueueForDelete(obj interface{}, eleType string) {
	var key string
	var err error
	if key, err = cache.DeletionHandlingMetaNamespaceKeyFunc(obj); err != nil {
		utilruntime.HandleError(err)
		return
	}
	e := Element{
		Type: eleType,
		Key:  key,
	}
	c.workqueue.Add(e)
}

func (c *Controller) Run(stopCh <-chan struct{}, cacheSyncs ...cache.InformerSynced) error {
	defer utilruntime.HandleCrash()
	defer c.workqueue.ShutDown()

	// Start the informer factories to begin populating the informer caches
	klog.Info("Starting controller")

	// Wait for the caches to be synced before starting workers
	klog.Info("Waiting for informer caches to sync")
	if ok := cache.WaitForCacheSync(stopCh, cacheSyncs...); !ok {
		return fmt.Errorf("failed to wait for caches to sync")
	}

	klog.Info("Starting kubernetes discovery workers")

	go wait.Until(c.runWorker, time.Second, stopCh)

	<-stopCh
	klog.Info("Shutting down kubernetes discovery workers")

	return nil
}

func (c *Controller) runWorker() {
	for c.processNextWorkItem() {
	}
}

func (c *Controller) processNextWorkItem() bool {
	obj, shutdown := c.workqueue.Get()

	if shutdown {
		return false
	}

	// We wrap this block in a func so we can defer c.workqueue.Done.
	err := func(obj interface{}) error {
		// We call Done here so the workqueue knows we have finished
		// processing this item. We also must remember to call Forget if we
		// do not want this work item being re-queued. For example, we do
		// not call Forget if a transient error occurs, instead the item is
		// put back on the workqueue and attempted again after a back-off
		// period.
		defer c.workqueue.Done(obj)
		var element Element
		var ok bool
		// We expect strings to come off the workqueue. These are of the
		// form namespace/name. We do this as the delayed nature of the
		// workqueue means the items in the informer cache may actually be
		// more up to date that when the item was initially put onto the
		// workqueue.
		if element, ok = obj.(Element); !ok {
			// As the item in the workqueue is actually invalid, we call
			// Forget here else we'd go into a loop of attempting to
			// process a work item that is invalid.
			c.workqueue.Forget(obj)
			utilruntime.HandleError(fmt.Errorf("expected string in workqueue but got %#v", obj))
			return nil
		}
		// Run the syncHandler, passing it the namespace/name string of the
		// Foo resource to be synced.
		if err := c.syncHandler(element); err != nil {
			// Put the item back on the workqueue to handle any transient errors.
			c.workqueue.AddRateLimited(element)
			return fmt.Errorf("error syncing '%+v': %w, requeuing", element, err)
		}
		// Finally, if no error occurs we Forget this item so it does not
		// get queued again until another change happens.
		c.workqueue.Forget(obj)
		// log.Debug("Successfully synced '%s'", element)
		return nil
	}(obj)

	if err != nil {
		utilruntime.HandleError(err)
		return true
	}

	return true
}

func (c *Controller) syncHandler(element Element) error {
	klog.Infof("syncHandler start process: %+v", element)

	var err error
	switch element.Type {
	case EventService:
		if err = c.reconcileService(element.Key); err != nil {
			klog.Warningf("reconcile service %s err: %+v", element.Key, err)
		}

	case EventEndpoints:
		if err = c.reconcileEndpoints(element.Key); err != nil {
			klog.Warningf("reconcile endpoint %s err: %+v", element.Key, err)
		}

	default:
		utilruntime.HandleError(fmt.Errorf("element type: %s not supported", element.Type))
		return nil
	}

	return nil
}

func (c *Controller) reconcileService(key string) error {
	klog.Infof("start reconsile svc %s", key)
	namespace, name, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		runtime.HandleError(fmt.Errorf("invalid resource key: %s", key))
		return err
	}

	svc, err := c.svcsLister.Services(namespace).Get(name)
	if apierrors.IsNotFound(err) {
		klog.Infof("svc %s has been deleted!", svc.Name)
		return nil
	} else if err != nil {
		runtime.HandleError(fmt.Errorf("{svc: %s/%s} failed to get svc by lister", namespace, name))
		return err
	}
	if svc.Labels != nil {
		if v, ok := svc.Labels["app"]; ok && v == constants.SimplecontrollerControllerName {
			c.record.Eventf(svc, corev1.EventTypeNormal, ReasonSuccess, MessageSyncSuccess, svc.Name, "ok")
		} else {
			c.record.Eventf(svc, corev1.EventTypeWarning, ReasonFailed, MessageSyncFailed, svc.Name, "error")
		}
	} else {
		c.record.Eventf(svc, corev1.EventTypeWarning, ReasonFailed, MessageSyncFailed, svc.Name, "error")
	}
	svcs, err := c.svcIndexer.ByIndex(ByindexBySvcLabels, constants.SimplecontrollerControllerName)
	if err != nil {
		klog.Errorf("Error retrieving pods by index: %v", err)
	}
	for _, labelSvc := range svcs {
		if svc, ok := labelSvc.(*corev1.Service); ok {
			klog.Infof("get svc %s by index", svc.Name)
		}
	}
	klog.Infof("finish reconsile svc %s", key)
	return nil
}

func (c *Controller) reconcileEndpoints(key string) error {
	klog.Infof("start reconsile ep %s", key)
	namespace, name, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		runtime.HandleError(fmt.Errorf("invalid resource key: %s", key))
		return err
	}

	ep, err := c.epsLister.Endpoints(namespace).Get(name)
	if apierrors.IsNotFound(err) {
		klog.Infof("ep %s has been deleted!", ep.Name)
		return nil
	} else if err != nil {
		runtime.HandleError(fmt.Errorf("{ep: %s/%s} failed to get ep by lister", namespace, name))
		return err
	}
	klog.Infof("finish reconsile ep %s", key)
	return nil
}
