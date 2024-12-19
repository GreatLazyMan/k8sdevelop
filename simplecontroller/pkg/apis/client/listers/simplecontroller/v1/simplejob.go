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

// Code generated by lister-gen. DO NOT EDIT.

package v1

import (
	v1 "github.com/GreatLazyMan/simplecontroller/pkg/apis/simplecontroller/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/tools/cache"
)

// SimpleJobLister helps list SimpleJobs.
// All objects returned here must be treated as read-only.
type SimpleJobLister interface {
	// List lists all SimpleJobs in the indexer.
	// Objects returned here must be treated as read-only.
	List(selector labels.Selector) (ret []*v1.SimpleJob, err error)
	// SimpleJobs returns an object that can list and get SimpleJobs.
	SimpleJobs(namespace string) SimpleJobNamespaceLister
	SimpleJobListerExpansion
}

// simpleJobLister implements the SimpleJobLister interface.
type simpleJobLister struct {
	indexer cache.Indexer
}

// NewSimpleJobLister returns a new SimpleJobLister.
func NewSimpleJobLister(indexer cache.Indexer) SimpleJobLister {
	return &simpleJobLister{indexer: indexer}
}

// List lists all SimpleJobs in the indexer.
func (s *simpleJobLister) List(selector labels.Selector) (ret []*v1.SimpleJob, err error) {
	err = cache.ListAll(s.indexer, selector, func(m interface{}) {
		ret = append(ret, m.(*v1.SimpleJob))
	})
	return ret, err
}

// SimpleJobs returns an object that can list and get SimpleJobs.
func (s *simpleJobLister) SimpleJobs(namespace string) SimpleJobNamespaceLister {
	return simpleJobNamespaceLister{indexer: s.indexer, namespace: namespace}
}

// SimpleJobNamespaceLister helps list and get SimpleJobs.
// All objects returned here must be treated as read-only.
type SimpleJobNamespaceLister interface {
	// List lists all SimpleJobs in the indexer for a given namespace.
	// Objects returned here must be treated as read-only.
	List(selector labels.Selector) (ret []*v1.SimpleJob, err error)
	// Get retrieves the SimpleJob from the indexer for a given namespace and name.
	// Objects returned here must be treated as read-only.
	Get(name string) (*v1.SimpleJob, error)
	SimpleJobNamespaceListerExpansion
}

// simpleJobNamespaceLister implements the SimpleJobNamespaceLister
// interface.
type simpleJobNamespaceLister struct {
	indexer   cache.Indexer
	namespace string
}

// List lists all SimpleJobs in the indexer for a given namespace.
func (s simpleJobNamespaceLister) List(selector labels.Selector) (ret []*v1.SimpleJob, err error) {
	err = cache.ListAllByNamespace(s.indexer, s.namespace, selector, func(m interface{}) {
		ret = append(ret, m.(*v1.SimpleJob))
	})
	return ret, err
}

// Get retrieves the SimpleJob from the indexer for a given namespace and name.
func (s simpleJobNamespaceLister) Get(name string) (*v1.SimpleJob, error) {
	obj, exists, err := s.indexer.GetByKey(s.namespace + "/" + name)
	if err != nil {
		return nil, err
	}
	if !exists {
		return nil, errors.NewNotFound(v1.Resource("simplejob"), name)
	}
	return obj.(*v1.SimpleJob), nil
}
