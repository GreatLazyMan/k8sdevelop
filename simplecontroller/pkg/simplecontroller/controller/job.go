package controller

import (
	"context"
	"fmt"
	"reflect"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/record"
	"k8s.io/client-go/util/retry"
	"k8s.io/client-go/util/workqueue"
	klog "k8s.io/klog/v2"
	controllerruntime "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"

	simplecontrollerv1 "github.com/GreatLazyMan/simplecontroller/pkg/apis/simplecontroller/v1"
	"github.com/GreatLazyMan/simplecontroller/pkg/utils"
)

var ServiceKind string
var PodKind string
var ConfigmapKind string

const FinalizerKey = "simple.io/simplejob"

func init() {
	tempPod := corev1.Pod{}
	PodKind = tempPod.GetObjectKind().GroupVersionKind().Kind
	tempSvc := corev1.Service{}
	ServiceKind = tempSvc.GetObjectKind().GroupVersionKind().Kind
	tempCm := corev1.ConfigMap{}
	ConfigmapKind = tempCm.GetObjectKind().GroupVersionKind().Kind
}

type SimpleJobController struct {
	client.Client
	Config        *rest.Config
	EventRecorder record.EventRecorder
	Name          string
	Clientset     *kubernetes.Clientset
	Scheme        *runtime.Scheme
}

func (c *SimpleJobController) getOwnerFunc() predicate.TypedPredicate[*simplecontrollerv1.SimpleJob] {
	return predicate.TypedFuncs[*simplecontrollerv1.SimpleJob]{
		CreateFunc: func(createEvent event.TypedCreateEvent[*simplecontrollerv1.SimpleJob]) bool {
			obj := createEvent.Object
			c.Scheme.Default(obj)
			return true
		},
		UpdateFunc: func(updateEvent event.TypedUpdateEvent[*simplecontrollerv1.SimpleJob]) bool {
			newObj := updateEvent.ObjectNew
			oldObj := updateEvent.ObjectOld

			if !newObj.DeletionTimestamp.IsZero() {
				return true
			}

			return !reflect.DeepEqual(newObj.Spec, oldObj.Spec)
		},
		GenericFunc: func(genericEvent event.TypedGenericEvent[*simplecontrollerv1.SimpleJob]) bool {
			return false
		},
	}
}

func (c *SimpleJobController) SetupWithManager(mgr manager.Manager) error {
	// creates the clientset
	clientset, err := kubernetes.NewForConfig(c.Config)
	if err != nil {
		klog.Errorf("get clientset error: %v", clientset)
		return err
	}
	c.Clientset = clientset

	ctl, err := controller.New(c.Name, mgr, controller.Options{
		Reconciler:              c,
		MaxConcurrentReconciles: 1,
		RateLimiter:             workqueue.DefaultControllerRateLimiter(),
	})
	if err != nil {
		klog.Errorf("init controller err: %v", err)
		return err
	}
	err = ctl.Watch(source.Kind(mgr.GetCache(), &simplecontrollerv1.SimpleJob{},
		&handler.TypedEnqueueRequestForObject[*simplecontrollerv1.SimpleJob]{},
		c.getOwnerFunc()))
	if err != nil {
		klog.Errorf("watch SimpleJob err: %v", err)
		return err
	}

	err = ctl.Watch(source.Kind(mgr.GetCache(), &corev1.Pod{},
		getOwnObjEventhandler[*corev1.Pod](mgr),
		getOwnPredicate[*corev1.Pod](),
	))
	if err != nil {
		klog.Errorf("watch owned pod err: %v", err)
		return err
	}

	err = ctl.Watch(source.Kind(mgr.GetCache(), &corev1.Service{},
		getOwnObjEventhandler[*corev1.Service](mgr),
		getOwnPredicate[*corev1.Service](),
	))
	if err != nil {
		klog.Errorf("watch owned svc err: %v", err)
		return err
	}

	err = ctl.Watch(source.Kind(mgr.GetCache(), &corev1.ConfigMap{},
		getOwnObjEventhandler[*corev1.ConfigMap](mgr),
		getOwnPredicate[*corev1.ConfigMap](),
	))
	if err != nil {
		klog.Errorf("watch owned cm err: %v", err)
		return err
	}

	return nil
}

func getOwnPredicate[T client.Object]() predicate.TypedFuncs[T] {
	return predicate.TypedFuncs[T]{
		CreateFunc: func(e event.TypedCreateEvent[T]) bool {
			klog.Info("createEvent")
			objKind := e.Object.GetObjectKind().GroupVersionKind().Kind
			switch objKind {
			case PodKind:
				klog.Infof("receive own pod %s info", e.Object.GetName())
				if pod, ok := client.Object(e.Object).(*corev1.Pod); ok {
					klog.Infof("pod spec is %v", pod.Spec)
				}
			case ServiceKind:
				klog.Infof("receive own svc %s info", e.Object.GetName())
			case ConfigmapKind:
				klog.Infof("receive own cm %s info", e.Object.GetName())
			default:
				return false
			}
			return true
		},
		UpdateFunc: func(e event.TypedUpdateEvent[T]) bool {
			klog.Info("updateEvent")
			newObj := e.ObjectNew
			oldObj := e.ObjectOld
			if newObj.GetResourceVersion() == oldObj.GetResourceVersion() {
				return false
			}
			objKind := newObj.GetObjectKind().GroupVersionKind().Kind
			switch objKind {
			case PodKind:
				klog.Infof("receive own pod %s info", newObj.GetName())
				newpod, newok := client.Object(newObj).(*corev1.Pod)
				oldpod, oldok := client.Object(oldObj).(*corev1.Pod)
				if newok && oldok {
					if !equality.Semantic.DeepEqual(newpod.Spec, oldpod.Spec) {
						return false
					}
				}
			case ServiceKind:
				klog.Infof("receive own svc %s info", newObj.GetName())
			case ConfigmapKind:
				klog.Infof("receive own cm %s info", newObj.GetName())
			default:
				return false
			}
			return true
		},
		DeleteFunc: func(e event.TypedDeleteEvent[T]) bool {
			return true
		},
	}
}

func getOwnObjEventhandler[T client.Object](mgr manager.Manager) handler.TypedEventHandler[T] {
	return handler.TypedEnqueueRequestForOwner[T](mgr.GetScheme(),
		mgr.GetRESTMapper(), &simplecontrollerv1.SimpleJob{}, handler.OnlyControllerOwner())
}

func (c *SimpleJobController) Reconcile(ctx context.Context, request reconcile.Request) (reconcile.Result, error) {
	klog.Infof("Start reconcile SimpleJob %s", request.String())
	defer klog.Infof("Finish reconcile SimpleJob %s", request.String())
	resource := &simplecontrollerv1.SimpleJob{} //TODO
	if err := c.Get(ctx, request.NamespacedName, resource); err != nil {
		if errors.IsNotFound(err) {
			klog.Infof("SimpleJob %s has be deleted", request.String())
			return controllerruntime.Result{}, nil
		}
		klog.Errorf("get SimpleJob %s error:", err)
		return controllerruntime.Result{}, err
	}
	if !resource.DeletionTimestamp.IsZero() {
		klog.Infof("SimpleJob %s has been deleted", request.String())
		return c.removeFinalizer(ctx, request)
	}
	if err := c.CreateTask(ctx, request, resource); err != nil {
		klog.Errorf("create task error: %v", err)
		return reconcile.Result{}, err
	}

	return c.ensureFinalizer(ctx, request)
}

func (c *SimpleJobController) CreateTask(ctx context.Context, request reconcile.Request,
	resource *simplecontrollerv1.SimpleJob) error {
	klog.Infof("start create task for %s", request.String())
	defer klog.Infof("finish create task for %s", request.String())
	jobPod := &corev1.Pod{}
	jobPod.Spec.Containers = []corev1.Container{{}}
	jobPod.SetName(fmt.Sprintf("%s-pod", request.Name))
	jobPod.SetNamespace(request.Namespace)
	jobPod.Spec.Containers[0].Image = "busybox:latest"
	jobPod.Spec.Containers[0].Name = "busybox"
	jobPod.Spec.Containers[0].ImagePullPolicy = corev1.PullIfNotPresent
	jobPod.Spec.Containers[0].Command = resource.Spec.Command

	utils.SetOwnerReference(resource, jobPod)
	if err := c.Create(ctx, jobPod); err != nil {
		if !errors.IsAlreadyExists(err) {
			klog.Errorf("create pods error: %v", err)
			return err
		}
		klog.Infof("pods %s exist, skip", jobPod.Name)
	}

	return nil
}

func (c *SimpleJobController) ensureFinalizer(ctx context.Context, request reconcile.Request) (reconcile.Result, error) {
	if err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
		current := &simplecontrollerv1.SimpleJob{}
		if err := c.Get(context.TODO(), request.NamespacedName, current); err != nil {
			klog.Errorf("get job %s error. %v", request.String(), err)
			return err
		}
		currentCopy := current.DeepCopy()
		if utils.EnsureFinalizer(currentCopy, FinalizerKey) {
			return c.Update(ctx, currentCopy)
		} else {
			return nil
		}
	}); err != nil {
		return reconcile.Result{}, err
	}
	return reconcile.Result{}, nil
}

func (c *SimpleJobController) removeFinalizer(ctx context.Context, request reconcile.Request) (reconcile.Result, error) {
	if err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
		current := &simplecontrollerv1.SimpleJob{}
		if err := c.Get(context.TODO(), request.NamespacedName, current); err != nil {
			klog.Errorf("get job %s error. %v", request.String(), err)
			return err
		}
		currentCopy := current.DeepCopy()
		if utils.RemoveFinalizer(currentCopy, FinalizerKey) {
			return c.Update(ctx, currentCopy)
		} else {
			return nil
		}
	}); err != nil {
		return reconcile.Result{}, err
	}
	return reconcile.Result{}, nil
}
