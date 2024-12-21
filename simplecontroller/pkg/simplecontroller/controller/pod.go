package controller

import (
	"context"
	"reflect"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/record"
	klog "k8s.io/klog/v2"
	controllerruntime "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

type PodController struct {
	client.Client
	Config        *rest.Config
	EventRecorder record.EventRecorder
	Name          string
	Clientset     *kubernetes.Clientset
}

var PodPredicatesFunc = predicate.Funcs{
	CreateFunc: func(createEvent event.CreateEvent) bool {
		return true
	},
	UpdateFunc: func(updateEvent event.UpdateEvent) bool {
		newObj := updateEvent.ObjectNew.(*corev1.Pod) //TODO
		oldObj := updateEvent.ObjectOld.(*corev1.Pod) //TODO

		if !newObj.DeletionTimestamp.IsZero() {
			return true
		}

		return !reflect.DeepEqual(newObj.Spec, oldObj.Spec)
	},
	GenericFunc: func(genericEvent event.GenericEvent) bool {
		return false
	},
}

func (c *PodController) SetupWithManager(mgr manager.Manager) error {

	// creates the clientset
	clientset, err := kubernetes.NewForConfig(c.Config)
	if err != nil {
		klog.Errorf("get clientset error: %v", clientset)
		return err
	}
	c.Clientset = clientset

	return controllerruntime.NewControllerManagedBy(mgr).
		Named(c.Name).
		WithOptions(controller.Options{}).
		For(&corev1.Pod{}, builder.WithPredicates(PodPredicatesFunc)). //TODO
		Complete(c)
}

func (c *PodController) Reconcile(ctx context.Context, request reconcile.Request) (reconcile.Result, error) {
	klog.Infof("Start reconcile Pod %s", request.Name)
	defer klog.Infof("Finish reconcile Pod %s", request.Name)
	resource := &corev1.Pod{} //TODO
	if err := c.Get(ctx, request.NamespacedName, resource); err != nil {
		if errors.IsNotFound(err) {
			klog.Infof("Pod %s has be deleted", request.Name)
			return controllerruntime.Result{}, nil
		}
		klog.Errorf("get Pod %s error:", err)
		return controllerruntime.Result{}, err
	}
	if !resource.DeletionTimestamp.IsZero() {
		klog.Infof("Pod %s has been deleted", request.Name)
		return controllerruntime.Result{}, nil
	}

	return controllerruntime.Result{}, nil
}
