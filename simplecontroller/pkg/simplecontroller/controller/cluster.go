package controller

import (
	"context"
	"reflect"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/record"
	"k8s.io/client-go/util/retry"
	klog "k8s.io/klog/v2"
	controllerruntime "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/GreatLazyMan/simplecontroller/pkg/apis/client/clientset/versioned"
	simplecontrollerv1 "github.com/GreatLazyMan/simplecontroller/pkg/apis/simplecontroller/v1"
	"github.com/GreatLazyMan/simplecontroller/pkg/simplecontroller/constants"
)

type ClusterController struct {
	client.Client
	Config            *rest.Config
	EventRecorder     record.EventRecorder
	Name              string
	Clientset         *kubernetes.Clientset
	VsersionClientset *versioned.Clientset
}

var ClusterPredicatesFunc = predicate.Funcs{
	CreateFunc: func(createEvent event.CreateEvent) bool {
		return true
	},
	UpdateFunc: func(updateEvent event.UpdateEvent) bool {
		newObj := updateEvent.ObjectNew.(*simplecontrollerv1.Cluster) //TODO
		oldObj := updateEvent.ObjectOld.(*simplecontrollerv1.Cluster) //TODO

		if !newObj.DeletionTimestamp.IsZero() {
			return true
		}

		return !reflect.DeepEqual(newObj.Spec, oldObj.Spec)
	},
	GenericFunc: func(genericEvent event.GenericEvent) bool {
		return false
	},
}

func (c *ClusterController) SetupWithManager(mgr manager.Manager) error {

	// creates the clientset
	clientset, err := kubernetes.NewForConfig(c.Config)
	if err != nil {
		klog.Errorf("get clientset error: %v", clientset)
		return err
	}
	c.Clientset = clientset

	vsersionClientset, err := versioned.NewForConfig(c.Config)
	if err != nil {
		klog.Errorf("get versioned clientset error: %v", clientset)
		return err
	}
	c.VsersionClientset = vsersionClientset

	return controllerruntime.NewControllerManagedBy(mgr).
		Named(c.Name).
		WithOptions(controller.Options{}).
		For(&simplecontrollerv1.Cluster{}, builder.WithPredicates(ClusterPredicatesFunc)). //TODO
		Owns(&corev1.Pod{}).
		Complete(c)
}

func (c *ClusterController) Reconcile(ctx context.Context, request reconcile.Request) (reconcile.Result, error) {
	klog.Infof("Start reconcile Cluster %s", request.Name)
	defer klog.Infof("Finish reconcile Cluster %s", request.Name)
	resource := &simplecontrollerv1.Cluster{} //TODO
	if err := c.Get(ctx, request.NamespacedName, resource); err != nil {
		if errors.IsNotFound(err) {
			klog.Infof("Cluster %s has be deleted", request.Name)
		}
		klog.Errorf("get Cluster %s error:", err)
		return controllerruntime.Result{
			RequeueAfter: constants.DefaultRequeueTime,
			Requeue:      true}, err
	}
	if !resource.DeletionTimestamp.IsZero() {
		klog.Infof("Cluster %s has been deleted", request.Name)
	}

	if err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
		current := &simplecontrollerv1.Cluster{}
		if err := c.Get(context.TODO(), types.NamespacedName{
			Name: resource.Name,
		}, current); err != nil {
			klog.Errorf("get cluster %s error. %v", resource.Name, err)
			return err
		}
		return c.Client.Patch(context.TODO(), resource, client.MergeFrom(current))
	}); err != nil {
		return controllerruntime.Result{}, err
	}

	return controllerruntime.Result{}, nil
}
