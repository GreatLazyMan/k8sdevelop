package utils

import (
	"context"
	"fmt"

	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes/scheme"
)

type ResourceService struct {
	restMapper *meta.RESTMapper
	client     *dynamic.DynamicClient
	fact       informers.SharedInformerFactory
}

func NewResourceService(restMapper *meta.RESTMapper, client *dynamic.DynamicClient, fact informers.SharedInformerFactory) *ResourceService {
	return &ResourceService{restMapper: restMapper, client: client, fact: fact}
}

func (r *ResourceService) ListResource(resourceOrKindArg string, ns string) ([]runtime.Object, error) {
	restMapping, err := r.mappingFor(resourceOrKindArg, r.restMapper)
	if err != nil {
		return nil, err
	}

	informer, _ := r.fact.ForResource(restMapping.Resource)
	list, _ := informer.Lister().ByNamespace(ns).List(labels.Everything())
	return list, nil
}

func (r *ResourceService) DeleteResource(resourceOrKindArg string, ns string, name string) error {
	ri, err := r.getResourceInterface(resourceOrKindArg, ns, r.client, r.restMapper)
	if err != nil {
		return err
	}

	err = ri.Delete(context.Background(), name, metav1.DeleteOptions{})
	if err != nil {
		return err
	}
	return nil
}

func (r *ResourceService) CreateResource(resourceOrKindArg string, yaml string) error {
	obj := &unstructured.Unstructured{}
	_, _, err := scheme.Codecs.UniversalDeserializer().Decode([]byte(yaml), nil, obj)
	if err != nil {
		return err
	}

	ri, err := r.getResourceInterface(resourceOrKindArg, obj.GetNamespace(), r.client, r.restMapper)
	if err != nil {
		return err
	}

	_, err = ri.Create(context.Background(), obj, metav1.CreateOptions{})
	if err != nil {
		return err
	}
	return nil
}

func (r *ResourceService) GetGVR(resourceOrKindArg string) (*schema.GroupVersionResource, error) {
	restMapping, err := r.mappingFor(resourceOrKindArg, r.restMapper)
	if err != nil {
		return nil, err
	}

	return &restMapping.Resource, nil
}

// getResourceInterface 根据资源类型或名称和命名空间返回相应的资源接口
func (r *ResourceService) getResourceInterface(resourceOrKindArg string, ns string, client dynamic.Interface, restMapper *meta.RESTMapper) (dynamic.ResourceInterface, error) {
	var ri dynamic.ResourceInterface

	restMapping, err := r.mappingFor(resourceOrKindArg, restMapper)
	if err != nil {
		return nil, fmt.Errorf("failed to get RESTMapping for %s: %v", resourceOrKindArg, err)
	}

	// 判断资源是命名空间级别的还是集群级别的
	if restMapping.Scope.Name() == "namespace" {
		ri = client.Resource(restMapping.Resource).Namespace(ns)
	} else {
		ri = client.Resource(restMapping.Resource)
	}

	return ri, nil
}

func (r *ResourceService) mappingFor(resourceOrKindArg string, restMapper *meta.RESTMapper) (*meta.RESTMapping, error) {
	fullySpecifiedGVR, groupResource := schema.ParseResourceArg(resourceOrKindArg)
	gvk := schema.GroupVersionKind{}

	if fullySpecifiedGVR != nil {
		gvk, _ = (*restMapper).KindFor(*fullySpecifiedGVR)
	}
	if gvk.Empty() {
		fmt.Println("groupResource: ", groupResource)
		gvk, _ = (*restMapper).KindFor(groupResource.WithVersion(""))
		fmt.Println("gvk: ", gvk)
	}
	if !gvk.Empty() {
		return (*restMapper).RESTMapping(gvk.GroupKind(), gvk.Version)
	}

	fullySpecifiedGVK, groupKind := schema.ParseKindArg(resourceOrKindArg)
	if fullySpecifiedGVK == nil {
		gvk := groupKind.WithVersion("")
		fullySpecifiedGVK = &gvk
	}

	if !fullySpecifiedGVK.Empty() {
		if mapping, err := (*restMapper).RESTMapping(fullySpecifiedGVK.GroupKind(), fullySpecifiedGVK.Version); err == nil {
			return mapping, nil
		}
	}

	mapping, err := (*restMapper).RESTMapping(groupKind, gvk.Version)
	if err != nil {
		if meta.IsNoMatchError(err) {
			return nil, fmt.Errorf("the server doesn't have a resource type %q", groupResource.Resource)
		}
		return nil, err
	}

	return mapping, nil
}
