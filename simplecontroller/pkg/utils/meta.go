package utils

import (
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// NamespaceIndexName 自定义索引名称
const NamespaceIndexName = "namespace"

func EnsureFinalizer(obj client.Object, key string) bool {
	if !CheckFinalizerKeyExist(obj, key) {
		obj.SetFinalizers(append(obj.GetFinalizers(), key))
		return true
	}
	return false
}

func CheckFinalizerKeyExist(obj client.Object, key string) bool {
	finalizers := obj.GetFinalizers()
	for _, finalizer := range finalizers {
		if finalizer == key {
			return true
		}
	}
	return false
}

func RemoveFinalizer(obj client.Object, key string) bool {
	if CheckFinalizerKeyExist(obj, key) {
		oriFinalizers := obj.GetFinalizers()
		newFinalizers := []string{}
		for _, finalizer := range oriFinalizers {
			if finalizer != key {
				newFinalizers = append(newFinalizers, key)
			}
		}
		obj.SetFinalizers(newFinalizers)
		return true
	}
	return false
}

func SetOwnerReference(owner, ownee client.Object) {
	ownee.SetOwnerReferences(append(ownee.GetOwnerReferences(), metav1.OwnerReference{
		APIVersion:         owner.GetObjectKind().GroupVersionKind().GroupVersion().String(),
		Kind:               owner.GetObjectKind().GroupVersionKind().Kind,
		Name:               owner.GetName(),
		UID:                owner.GetUID(),
		BlockOwnerDeletion: ptr.To(true),
		Controller:         ptr.To(true),
	}),
	)
}

// NamespaceIndexFunc 自定义索引函数，根据对象的命名空间生成索引键
func NamespaceIndexFunc(obj interface{}) ([]string, error) {
	metaObj, ok := obj.(metav1.Object)
	if !ok {
		return nil, fmt.Errorf("object is not a metav1.Object")
	}
	return []string{metaObj.GetNamespace()}, nil
}
