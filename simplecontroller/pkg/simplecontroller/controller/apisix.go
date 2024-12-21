package controller

import (
	"context"
	"reflect"

	apisixv2 "github.com/apache/apisix-ingress-controller/pkg/kube/apisix/apis/config/v2"
	apisixv "github.com/apache/apisix-ingress-controller/pkg/kube/apisix/client/clientset/versioned"
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

type ApisixRouteController struct {
	client.Client
	Config        *rest.Config
	EventRecorder record.EventRecorder
	Name          string
	Clientset     *kubernetes.Clientset
	ApisixClient  *apisixv.Clientset
}

var ApisixRoutePredicatesFunc = predicate.Funcs{
	CreateFunc: func(createEvent event.CreateEvent) bool {
		return true
	},
	UpdateFunc: func(updateEvent event.UpdateEvent) bool {
		newObj := updateEvent.ObjectNew.(*apisixv2.ApisixRoute) //TODO
		oldObj := updateEvent.ObjectOld.(*apisixv2.ApisixRoute) //TODO

		if !newObj.DeletionTimestamp.IsZero() {
			return true
		}

		return !reflect.DeepEqual(newObj.Spec, oldObj.Spec)
	},
	GenericFunc: func(genericEvent event.GenericEvent) bool {
		return false
	},
}

func (c *ApisixRouteController) SetupWithManager(mgr manager.Manager) error {
	// creates the clientset
	clientset, err := kubernetes.NewForConfig(c.Config)
	if err != nil {
		klog.Errorf("get clientset error: %v", err)
		return err
	}
	c.Clientset = clientset
	asClientset, err := apisixv.NewForConfig(c.Config)
	if err != nil {
		klog.Errorf("get clientset error: %v", err)
		return err
	}
	c.ApisixClient = asClientset

	return controllerruntime.NewControllerManagedBy(mgr).
		Named(c.Name).
		WithOptions(controller.Options{}).
		For(&apisixv2.ApisixRoute{}, builder.WithPredicates(ApisixRoutePredicatesFunc)). //TODO
		Complete(c)
}

func (c *ApisixRouteController) Reconcile(ctx context.Context, request reconcile.Request) (reconcile.Result, error) {
	klog.Infof("Start reconcile ApisixRoute %s", request.Name)
	defer klog.Infof("Finish reconcile ApisixRoute %s", request.Name)
	resource := &apisixv2.ApisixRoute{} //TODO
	if err := c.Get(ctx, request.NamespacedName, resource); err != nil {
		if errors.IsNotFound(err) {
			klog.Infof("ApisixRoute %s has be deleted", request.Name)
			return controllerruntime.Result{}, nil
		}
		klog.Errorf("get ApisixRoute %s error:", err)
		return controllerruntime.Result{}, err
	}
	if !resource.DeletionTimestamp.IsZero() {
		klog.Infof("ApisixRoute %s has been deleted", request.Name)
		return controllerruntime.Result{}, nil
	}

	return controllerruntime.Result{}, nil
}
