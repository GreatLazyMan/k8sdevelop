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

package v1

import (
	"context"
	"time"

	scheme "github.com/GreatLazyMan/simplecontroller/pkg/apis/client/clientset/versioned/scheme"
	v1 "github.com/GreatLazyMan/simplecontroller/pkg/apis/simplecontroller/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	types "k8s.io/apimachinery/pkg/types"
	watch "k8s.io/apimachinery/pkg/watch"
	rest "k8s.io/client-go/rest"
)

// RealClustersGetter has a method to return a RealClusterInterface.
// A group's client should implement this interface.
type RealClustersGetter interface {
	RealClusters() RealClusterInterface
}

// RealClusterInterface has methods to work with RealCluster resources.
type RealClusterInterface interface {
	Create(ctx context.Context, realCluster *v1.RealCluster, opts metav1.CreateOptions) (*v1.RealCluster, error)
	Update(ctx context.Context, realCluster *v1.RealCluster, opts metav1.UpdateOptions) (*v1.RealCluster, error)
	UpdateStatus(ctx context.Context, realCluster *v1.RealCluster, opts metav1.UpdateOptions) (*v1.RealCluster, error)
	Delete(ctx context.Context, name string, opts metav1.DeleteOptions) error
	DeleteCollection(ctx context.Context, opts metav1.DeleteOptions, listOpts metav1.ListOptions) error
	Get(ctx context.Context, name string, opts metav1.GetOptions) (*v1.RealCluster, error)
	List(ctx context.Context, opts metav1.ListOptions) (*v1.RealClusterList, error)
	Watch(ctx context.Context, opts metav1.ListOptions) (watch.Interface, error)
	Patch(ctx context.Context, name string, pt types.PatchType, data []byte, opts metav1.PatchOptions, subresources ...string) (result *v1.RealCluster, err error)
	RealClusterExpansion
}

// realClusters implements RealClusterInterface
type realClusters struct {
	client rest.Interface
}

// newRealClusters returns a RealClusters
func newRealClusters(c *GreatlazymanV1Client) *realClusters {
	return &realClusters{
		client: c.RESTClient(),
	}
}

// Get takes name of the realCluster, and returns the corresponding realCluster object, and an error if there is any.
func (c *realClusters) Get(ctx context.Context, name string, options metav1.GetOptions) (result *v1.RealCluster, err error) {
	result = &v1.RealCluster{}
	err = c.client.Get().
		Resource("realclusters").
		Name(name).
		VersionedParams(&options, scheme.ParameterCodec).
		Do(ctx).
		Into(result)
	return
}

// List takes label and field selectors, and returns the list of RealClusters that match those selectors.
func (c *realClusters) List(ctx context.Context, opts metav1.ListOptions) (result *v1.RealClusterList, err error) {
	var timeout time.Duration
	if opts.TimeoutSeconds != nil {
		timeout = time.Duration(*opts.TimeoutSeconds) * time.Second
	}
	result = &v1.RealClusterList{}
	err = c.client.Get().
		Resource("realclusters").
		VersionedParams(&opts, scheme.ParameterCodec).
		Timeout(timeout).
		Do(ctx).
		Into(result)
	return
}

// Watch returns a watch.Interface that watches the requested realClusters.
func (c *realClusters) Watch(ctx context.Context, opts metav1.ListOptions) (watch.Interface, error) {
	var timeout time.Duration
	if opts.TimeoutSeconds != nil {
		timeout = time.Duration(*opts.TimeoutSeconds) * time.Second
	}
	opts.Watch = true
	return c.client.Get().
		Resource("realclusters").
		VersionedParams(&opts, scheme.ParameterCodec).
		Timeout(timeout).
		Watch(ctx)
}

// Create takes the representation of a realCluster and creates it.  Returns the server's representation of the realCluster, and an error, if there is any.
func (c *realClusters) Create(ctx context.Context, realCluster *v1.RealCluster, opts metav1.CreateOptions) (result *v1.RealCluster, err error) {
	result = &v1.RealCluster{}
	err = c.client.Post().
		Resource("realclusters").
		VersionedParams(&opts, scheme.ParameterCodec).
		Body(realCluster).
		Do(ctx).
		Into(result)
	return
}

// Update takes the representation of a realCluster and updates it. Returns the server's representation of the realCluster, and an error, if there is any.
func (c *realClusters) Update(ctx context.Context, realCluster *v1.RealCluster, opts metav1.UpdateOptions) (result *v1.RealCluster, err error) {
	result = &v1.RealCluster{}
	err = c.client.Put().
		Resource("realclusters").
		Name(realCluster.Name).
		VersionedParams(&opts, scheme.ParameterCodec).
		Body(realCluster).
		Do(ctx).
		Into(result)
	return
}

// UpdateStatus was generated because the type contains a Status member.
// Add a +genclient:noStatus comment above the type to avoid generating UpdateStatus().
func (c *realClusters) UpdateStatus(ctx context.Context, realCluster *v1.RealCluster, opts metav1.UpdateOptions) (result *v1.RealCluster, err error) {
	result = &v1.RealCluster{}
	err = c.client.Put().
		Resource("realclusters").
		Name(realCluster.Name).
		SubResource("status").
		VersionedParams(&opts, scheme.ParameterCodec).
		Body(realCluster).
		Do(ctx).
		Into(result)
	return
}

// Delete takes name of the realCluster and deletes it. Returns an error if one occurs.
func (c *realClusters) Delete(ctx context.Context, name string, opts metav1.DeleteOptions) error {
	return c.client.Delete().
		Resource("realclusters").
		Name(name).
		Body(&opts).
		Do(ctx).
		Error()
}

// DeleteCollection deletes a collection of objects.
func (c *realClusters) DeleteCollection(ctx context.Context, opts metav1.DeleteOptions, listOpts metav1.ListOptions) error {
	var timeout time.Duration
	if listOpts.TimeoutSeconds != nil {
		timeout = time.Duration(*listOpts.TimeoutSeconds) * time.Second
	}
	return c.client.Delete().
		Resource("realclusters").
		VersionedParams(&listOpts, scheme.ParameterCodec).
		Timeout(timeout).
		Body(&opts).
		Do(ctx).
		Error()
}

// Patch applies the patch and returns the patched realCluster.
func (c *realClusters) Patch(ctx context.Context, name string, pt types.PatchType, data []byte, opts metav1.PatchOptions, subresources ...string) (result *v1.RealCluster, err error) {
	result = &v1.RealCluster{}
	err = c.client.Patch(pt).
		Resource("realclusters").
		Name(name).
		SubResource(subresources...).
		VersionedParams(&opts, scheme.ParameterCodec).
		Body(data).
		Do(ctx).
		Into(result)
	return
}
