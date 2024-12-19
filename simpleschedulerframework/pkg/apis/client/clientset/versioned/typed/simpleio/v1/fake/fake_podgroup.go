/*
Copyright The Kubernetes Authors.

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

// Code generated by client-gen. DO NOT EDIT.

package fake

import (
	"context"

	v1 "github.com/GreatLazyMan/simplescheduler/pkg/apis/crd/simpleio/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	labels "k8s.io/apimachinery/pkg/labels"
	types "k8s.io/apimachinery/pkg/types"
	watch "k8s.io/apimachinery/pkg/watch"
	testing "k8s.io/client-go/testing"
)

// FakePodGroups implements PodGroupInterface
type FakePodGroups struct {
	Fake *FakeSimpleV1
	ns   string
}

var podgroupsResource = v1.SchemeGroupVersion.WithResource("podgroups")

var podgroupsKind = v1.SchemeGroupVersion.WithKind("PodGroup")

// Get takes name of the podGroup, and returns the corresponding podGroup object, and an error if there is any.
func (c *FakePodGroups) Get(ctx context.Context, name string, options metav1.GetOptions) (result *v1.PodGroup, err error) {
	emptyResult := &v1.PodGroup{}
	obj, err := c.Fake.
		Invokes(testing.NewGetActionWithOptions(podgroupsResource, c.ns, name, options), emptyResult)

	if obj == nil {
		return emptyResult, err
	}
	return obj.(*v1.PodGroup), err
}

// List takes label and field selectors, and returns the list of PodGroups that match those selectors.
func (c *FakePodGroups) List(ctx context.Context, opts metav1.ListOptions) (result *v1.PodGroupList, err error) {
	emptyResult := &v1.PodGroupList{}
	obj, err := c.Fake.
		Invokes(testing.NewListActionWithOptions(podgroupsResource, podgroupsKind, c.ns, opts), emptyResult)

	if obj == nil {
		return emptyResult, err
	}

	label, _, _ := testing.ExtractFromListOptions(opts)
	if label == nil {
		label = labels.Everything()
	}
	list := &v1.PodGroupList{ListMeta: obj.(*v1.PodGroupList).ListMeta}
	for _, item := range obj.(*v1.PodGroupList).Items {
		if label.Matches(labels.Set(item.Labels)) {
			list.Items = append(list.Items, item)
		}
	}
	return list, err
}

// Watch returns a watch.Interface that watches the requested podGroups.
func (c *FakePodGroups) Watch(ctx context.Context, opts metav1.ListOptions) (watch.Interface, error) {
	return c.Fake.
		InvokesWatch(testing.NewWatchActionWithOptions(podgroupsResource, c.ns, opts))

}

// Create takes the representation of a podGroup and creates it.  Returns the server's representation of the podGroup, and an error, if there is any.
func (c *FakePodGroups) Create(ctx context.Context, podGroup *v1.PodGroup, opts metav1.CreateOptions) (result *v1.PodGroup, err error) {
	emptyResult := &v1.PodGroup{}
	obj, err := c.Fake.
		Invokes(testing.NewCreateActionWithOptions(podgroupsResource, c.ns, podGroup, opts), emptyResult)

	if obj == nil {
		return emptyResult, err
	}
	return obj.(*v1.PodGroup), err
}

// Update takes the representation of a podGroup and updates it. Returns the server's representation of the podGroup, and an error, if there is any.
func (c *FakePodGroups) Update(ctx context.Context, podGroup *v1.PodGroup, opts metav1.UpdateOptions) (result *v1.PodGroup, err error) {
	emptyResult := &v1.PodGroup{}
	obj, err := c.Fake.
		Invokes(testing.NewUpdateActionWithOptions(podgroupsResource, c.ns, podGroup, opts), emptyResult)

	if obj == nil {
		return emptyResult, err
	}
	return obj.(*v1.PodGroup), err
}

// UpdateStatus was generated because the type contains a Status member.
// Add a +genclient:noStatus comment above the type to avoid generating UpdateStatus().
func (c *FakePodGroups) UpdateStatus(ctx context.Context, podGroup *v1.PodGroup, opts metav1.UpdateOptions) (result *v1.PodGroup, err error) {
	emptyResult := &v1.PodGroup{}
	obj, err := c.Fake.
		Invokes(testing.NewUpdateSubresourceActionWithOptions(podgroupsResource, "status", c.ns, podGroup, opts), emptyResult)

	if obj == nil {
		return emptyResult, err
	}
	return obj.(*v1.PodGroup), err
}

// Delete takes name of the podGroup and deletes it. Returns an error if one occurs.
func (c *FakePodGroups) Delete(ctx context.Context, name string, opts metav1.DeleteOptions) error {
	_, err := c.Fake.
		Invokes(testing.NewDeleteActionWithOptions(podgroupsResource, c.ns, name, opts), &v1.PodGroup{})

	return err
}

// DeleteCollection deletes a collection of objects.
func (c *FakePodGroups) DeleteCollection(ctx context.Context, opts metav1.DeleteOptions, listOpts metav1.ListOptions) error {
	action := testing.NewDeleteCollectionActionWithOptions(podgroupsResource, c.ns, opts, listOpts)

	_, err := c.Fake.Invokes(action, &v1.PodGroupList{})
	return err
}

// Patch applies the patch and returns the patched podGroup.
func (c *FakePodGroups) Patch(ctx context.Context, name string, pt types.PatchType, data []byte, opts metav1.PatchOptions, subresources ...string) (result *v1.PodGroup, err error) {
	emptyResult := &v1.PodGroup{}
	obj, err := c.Fake.
		Invokes(testing.NewPatchSubresourceActionWithOptions(podgroupsResource, c.ns, name, pt, data, opts, subresources...), emptyResult)

	if obj == nil {
		return emptyResult, err
	}
	return obj.(*v1.PodGroup), err
}
