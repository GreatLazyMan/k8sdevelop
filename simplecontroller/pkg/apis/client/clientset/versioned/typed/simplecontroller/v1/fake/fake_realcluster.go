// Copyright 2023 The Kubeflow Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// Code generated by client-gen. DO NOT EDIT.

package fake

import (
	"context"

	v1 "github.com/GreatLazyMan/simplecontroller/pkg/apis/simplecontroller/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	labels "k8s.io/apimachinery/pkg/labels"
	types "k8s.io/apimachinery/pkg/types"
	watch "k8s.io/apimachinery/pkg/watch"
	testing "k8s.io/client-go/testing"
)

// FakeRealClusters implements RealClusterInterface
type FakeRealClusters struct {
	Fake *FakeGreatlazymanV1
}

var realclustersResource = v1.SchemeGroupVersion.WithResource("realclusters")

var realclustersKind = v1.SchemeGroupVersion.WithKind("RealCluster")

// Get takes name of the realCluster, and returns the corresponding realCluster object, and an error if there is any.
func (c *FakeRealClusters) Get(ctx context.Context, name string, options metav1.GetOptions) (result *v1.RealCluster, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewRootGetAction(realclustersResource, name), &v1.RealCluster{})
	if obj == nil {
		return nil, err
	}
	return obj.(*v1.RealCluster), err
}

// List takes label and field selectors, and returns the list of RealClusters that match those selectors.
func (c *FakeRealClusters) List(ctx context.Context, opts metav1.ListOptions) (result *v1.RealClusterList, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewRootListAction(realclustersResource, realclustersKind, opts), &v1.RealClusterList{})
	if obj == nil {
		return nil, err
	}

	label, _, _ := testing.ExtractFromListOptions(opts)
	if label == nil {
		label = labels.Everything()
	}
	list := &v1.RealClusterList{ListMeta: obj.(*v1.RealClusterList).ListMeta}
	for _, item := range obj.(*v1.RealClusterList).Items {
		if label.Matches(labels.Set(item.Labels)) {
			list.Items = append(list.Items, item)
		}
	}
	return list, err
}

// Watch returns a watch.Interface that watches the requested realClusters.
func (c *FakeRealClusters) Watch(ctx context.Context, opts metav1.ListOptions) (watch.Interface, error) {
	return c.Fake.
		InvokesWatch(testing.NewRootWatchAction(realclustersResource, opts))
}

// Create takes the representation of a realCluster and creates it.  Returns the server's representation of the realCluster, and an error, if there is any.
func (c *FakeRealClusters) Create(ctx context.Context, realCluster *v1.RealCluster, opts metav1.CreateOptions) (result *v1.RealCluster, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewRootCreateAction(realclustersResource, realCluster), &v1.RealCluster{})
	if obj == nil {
		return nil, err
	}
	return obj.(*v1.RealCluster), err
}

// Update takes the representation of a realCluster and updates it. Returns the server's representation of the realCluster, and an error, if there is any.
func (c *FakeRealClusters) Update(ctx context.Context, realCluster *v1.RealCluster, opts metav1.UpdateOptions) (result *v1.RealCluster, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewRootUpdateAction(realclustersResource, realCluster), &v1.RealCluster{})
	if obj == nil {
		return nil, err
	}
	return obj.(*v1.RealCluster), err
}

// UpdateStatus was generated because the type contains a Status member.
// Add a +genclient:noStatus comment above the type to avoid generating UpdateStatus().
func (c *FakeRealClusters) UpdateStatus(ctx context.Context, realCluster *v1.RealCluster, opts metav1.UpdateOptions) (*v1.RealCluster, error) {
	obj, err := c.Fake.
		Invokes(testing.NewRootUpdateSubresourceAction(realclustersResource, "status", realCluster), &v1.RealCluster{})
	if obj == nil {
		return nil, err
	}
	return obj.(*v1.RealCluster), err
}

// Delete takes name of the realCluster and deletes it. Returns an error if one occurs.
func (c *FakeRealClusters) Delete(ctx context.Context, name string, opts metav1.DeleteOptions) error {
	_, err := c.Fake.
		Invokes(testing.NewRootDeleteActionWithOptions(realclustersResource, name, opts), &v1.RealCluster{})
	return err
}

// DeleteCollection deletes a collection of objects.
func (c *FakeRealClusters) DeleteCollection(ctx context.Context, opts metav1.DeleteOptions, listOpts metav1.ListOptions) error {
	action := testing.NewRootDeleteCollectionAction(realclustersResource, listOpts)

	_, err := c.Fake.Invokes(action, &v1.RealClusterList{})
	return err
}

// Patch applies the patch and returns the patched realCluster.
func (c *FakeRealClusters) Patch(ctx context.Context, name string, pt types.PatchType, data []byte, opts metav1.PatchOptions, subresources ...string) (result *v1.RealCluster, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewRootPatchSubresourceAction(realclustersResource, name, pt, data, subresources...), &v1.RealCluster{})
	if obj == nil {
		return nil, err
	}
	return obj.(*v1.RealCluster), err
}