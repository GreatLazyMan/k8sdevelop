// Copyright 2016 flannel authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package nodemanager

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"golang.org/x/net/context"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/strategicpatch"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/apimachinery/pkg/watch"
	clientset "k8s.io/client-go/kubernetes"
	listers "k8s.io/client-go/listers/core/v1"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/clientcmd"
	log "k8s.io/klog/v2"
)

var (
	ErrUnimplemented = errors.New("unimplemented")
)

const (
	resyncPeriod              = 5 * time.Minute
	nodeControllerSyncTimeout = 10 * time.Minute
)

type subnetFileInfo struct {
	path   string
	ipMask bool
	mtu    int
}

type kubeSubnetManager struct {
	enableIPv4                bool
	enableIPv6                bool
	annotations               annotations
	annotationPrefix          string
	client                    clientset.Interface
	nodeName                  string
	nodeStore                 listers.NodeLister
	nodeController            cache.Controller
	events                    chan Event
	clusterCIDRController     cache.Controller
	setNodeNetworkUnavailable bool
	snFileInfo                *subnetFileInfo
}

func NewSubnetManager(ctx context.Context, apiUrl, kubeconfig, prefix, netConfPath string) (*kubeSubnetManager, error) {
	var cfg *rest.Config
	var err error
	// Try to build kubernetes config from a master url or a kubeconfig filepath. If neither masterUrl
	// or kubeconfigPath are passed in we fall back to inClusterConfig. If inClusterConfig fails,
	// we fallback to the default config.
	cfg, err = clientcmd.BuildConfigFromFlags(apiUrl, kubeconfig)
	if err != nil {
		return nil, fmt.Errorf("fail to create kubernetes config: %v", err)
	}

	c, err := clientset.NewForConfig(cfg)
	if err != nil {
		return nil, fmt.Errorf("unable to initialize client: %v", err)
	}

	// The kube subnet mgr needs to know the k8s node name that it's running on so it can annotate it.
	// If we're running as a pod then the POD_NAME and POD_NAMESPACE will be populated and can be used to find the node
	// name. Otherwise, the environment variable NODE_NAME can be passed in.
	nodeName := os.Getenv("NODE_NAME")
	if nodeName == "" {
		podName := os.Getenv("POD_NAME")
		podNamespace := os.Getenv("POD_NAMESPACE")
		if podName == "" || podNamespace == "" {
			return nil, fmt.Errorf("env variables POD_NAME and POD_NAMESPACE must be set")
		}

		pod, err := c.CoreV1().Pods(podNamespace).Get(ctx, podName, metav1.GetOptions{})
		if err != nil {
			return nil, fmt.Errorf("error retrieving pod spec for '%s/%s': %v", podNamespace, podName, err)
		}
		nodeName = pod.Spec.NodeName
		if nodeName == "" {
			return nil, fmt.Errorf("node name not present in pod spec '%s/%s'", podNamespace, podName)
		}
	}

	sm, err := newKubeSubnetManager(ctx, c, nodeName, prefix)
	if err != nil {
		return nil, fmt.Errorf("error creating network manager: %s", err)
	}

	go sm.Run(ctx)

	log.Infof("Waiting %s for node controller to sync", nodeControllerSyncTimeout)
	err = wait.PollUntilContextTimeout(ctx, time.Second, nodeControllerSyncTimeout, true, func(context.Context) (bool, error) {
		return sm.nodeController.HasSynced(), nil
	})
	if err != nil {
		return nil, fmt.Errorf("error waiting for nodeController to sync state: %v", err)
	}
	log.Infof("Node controller sync successful")

	return sm, nil
}

// newKubeSubnetManager fills the kubeSubnetManager. The most important part is the controller which will
// watch for kubernetes node updates
func newKubeSubnetManager(ctx context.Context, c clientset.Interface, nodeName, prefix string) (*kubeSubnetManager, error) {
	var err error
	var ksm kubeSubnetManager
	ksm.annotationPrefix = prefix
	ksm.annotations, err = newAnnotations(prefix)
	if err != nil {
		return nil, err
	}
	ksm.client = c
	ksm.nodeName = nodeName
	scale := 5000
	scaleStr := os.Getenv("EVENT_QUEUE_DEPTH")
	if scaleStr != "" {
		n, err := strconv.Atoi(scaleStr)
		if err != nil {
			return nil, fmt.Errorf("env EVENT_QUEUE_DEPTH=%s format error: %v", scaleStr, err)
		}
		if n > 0 {
			scale = n
		}
	}
	ksm.events = make(chan Event, scale)
	indexer, controller := cache.NewIndexerInformer(
		&cache.ListWatch{
			ListFunc: func(options metav1.ListOptions) (runtime.Object, error) {
				return ksm.client.CoreV1().Nodes().List(ctx, options)
			},
			WatchFunc: func(options metav1.ListOptions) (watch.Interface, error) {
				return ksm.client.CoreV1().Nodes().Watch(ctx, options)
			},
		},
		&v1.Node{},
		resyncPeriod,
		cache.ResourceEventHandlerFuncs{
			AddFunc: func(obj interface{}) {
				ksm.handleAddLeaseEvent(EventAdded, obj)
			},
			UpdateFunc: ksm.handleUpdateLeaseEvent,
			DeleteFunc: func(obj interface{}) {
				_, isNode := obj.(*v1.Node)
				// We can get DeletedFinalStateUnknown instead of *api.Node here and we need to handle that correctly.
				if !isNode {
					deletedState, ok := obj.(cache.DeletedFinalStateUnknown)
					if !ok {
						log.Infof("Error received unexpected object: %v", obj)
						return
					}
					node, ok := deletedState.Obj.(*v1.Node)
					if !ok {
						log.Infof("Error deletedFinalStateUnknown contained non-Node object: %v", deletedState.Obj)
						return
					}
					obj = node
				}
				ksm.handleAddLeaseEvent(EventRemoved, obj)
			},
		},
		cache.Indexers{cache.NamespaceIndex: cache.MetaNamespaceIndexFunc},
	)
	ksm.nodeController = controller
	ksm.nodeStore = listers.NewNodeLister(indexer)

	return &ksm, nil
}

func (ksm *kubeSubnetManager) handleAddLeaseEvent(et EventType, obj interface{}) {
	n := obj.(*v1.Node)
	if s, ok := n.Annotations[ksm.annotations.SubnetKubeManaged]; !ok || s != "true" {
		return
	}

	l, err := ksm.nodeToLease(*n)
	if err != nil {
		log.Infof("Error turning node %q to lease: %v", n.ObjectMeta.Name, err)
		return
	}
	ksm.events <- Event{Type: et, Lease: l}
}

// handleUpdateLeaseEvent verifies if anything relevant changed in the node object: either
// ksm.annotations.BackendData, ksm.annotations.BackendType or ksm.annotations.BackendPublicIP
func (ksm *kubeSubnetManager) handleUpdateLeaseEvent(oldObj, newObj interface{}) {
	o := oldObj.(*v1.Node)
	n := newObj.(*v1.Node)
	if s, ok := n.Annotations[ksm.annotations.SubnetKubeManaged]; !ok || s != "true" {
		return
	}
	var changed = true
	if ksm.enableIPv4 && o.Annotations[ksm.annotations.BackendData] == n.Annotations[ksm.annotations.BackendData] &&
		o.Annotations[ksm.annotations.BackendType] == n.Annotations[ksm.annotations.BackendType] &&
		o.Annotations[ksm.annotations.BackendPublicIP] == n.Annotations[ksm.annotations.BackendPublicIP] {
		changed = false
	}

	if !changed {
		return // No change to lease
	}

	l, err := ksm.nodeToLease(*n)
	if err != nil {
		log.Infof("Error turning node %q to lease: %v", n.ObjectMeta.Name, err)
		return
	}
	ksm.events <- Event{Type: EventAdded, Lease: l}
}

// AcquireLease adds the flannel specific node annotations (defined in the struct LeaseAttrs) and returns a lease
// with important information for the backend, such as the subnet. This function is called once by the backend when
// registering
func (ksm *kubeSubnetManager) AcquireLease(ctx context.Context, attrs *LeaseAttrs) (*Lease, error) {
	var cachedNode *v1.Node
	var err error
	waitErr := wait.PollUntilContextTimeout(ctx, 3*time.Second, 30*time.Second, true, func(context.Context) (done bool, err error) {
		cachedNode, err = ksm.nodeStore.Get(ksm.nodeName)
		if err != nil {
			log.V(2).Infof("Failed to get node %q: %v", ksm.nodeName, err)
			return false, nil
		}
		return true, nil
	})
	if waitErr != nil {
		return nil, fmt.Errorf("timeout contacting kube-api, failed to patch node %q. Error: %v", ksm.nodeName, waitErr)
	}

	n := cachedNode.DeepCopy()
	if n.Spec.PodCIDR == "" {
		return nil, fmt.Errorf("node %q pod cidr not assigned", ksm.nodeName)
	}

	var bd []byte
	bd, err = attrs.BackendData.MarshalJSON()
	if err != nil {
		return nil, err
	}

	if n.Annotations[ksm.annotations.BackendData] != string(bd) {
		n.Annotations[ksm.annotations.SubnetKubeManaged] = "true"

		oldData, err := json.Marshal(cachedNode)
		if err != nil {
			return nil, err
		}

		newData, err := json.Marshal(n)
		if err != nil {
			return nil, err
		}

		patchBytes, err := strategicpatch.CreateTwoWayMergePatch(oldData, newData, v1.Node{})
		if err != nil {
			return nil, fmt.Errorf("failed to create patch for node %q: %v", ksm.nodeName, err)
		}

		waitErr := wait.PollUntilContextTimeout(ctx, 3*time.Second, 30*time.Second, true, func(context.Context) (done bool, err error) {
			_, err = ksm.client.CoreV1().Nodes().Patch(ctx, ksm.nodeName, types.StrategicMergePatchType, patchBytes, metav1.PatchOptions{}, "status")
			if err != nil {
				log.V(2).Infof("Failed to patch node %q: %v", ksm.nodeName, err)
				return false, nil
			}
			return true, nil
		})
		if waitErr != nil {
			return nil, fmt.Errorf("timeout contacting kube-api, failed to patch node %q. Error: %v", ksm.nodeName, waitErr)
		}
	}

	lease := &Lease{
		Attrs:      *attrs,
		Expiration: time.Now().Add(24 * time.Hour),
	}
	return lease, nil
}

// WatchLeases waits for the kubeSubnetManager to provide an event in case something relevant changed in the node data
func (ksm *kubeSubnetManager) WatchLeases(ctx context.Context, receiver chan []LeaseWatchResult) error {
	for {
		select {
		case event := <-ksm.events:
			receiver <- []LeaseWatchResult{
				{
					Events: []Event{event},
				}}
		case <-ctx.Done():
			close(receiver)
			return ctx.Err()
		}
	}
}

func (ksm *kubeSubnetManager) Run(ctx context.Context) {
	log.Infof("Starting kube subnet manager")
	ksm.nodeController.Run(ctx.Done())
}

// nodeToLease updates the lease with information fetch from the node, e.g. PodCIDR
func (ksm *kubeSubnetManager) nodeToLease(n v1.Node) (l Lease, err error) {
	if ksm.enableIPv4 {
		l.Attrs.BackendData = json.RawMessage(n.Annotations[ksm.annotations.BackendData])
		l.EnableIPv4 = ksm.enableIPv4
	}

	l.Attrs.BackendType = n.Annotations[ksm.annotations.BackendType]
	return l, nil
}

func (ksm *kubeSubnetManager) Name() string {
	return fmt.Sprintf("Kubernetes Subnet Manager - %s", ksm.nodeName)
}

// CompleteLease Set Kubernetes NodeNetworkUnavailable to false when starting
// https://kubernetes.io/docs/concepts/architecture/nodes/#condition
func (ksm *kubeSubnetManager) CompleteLease(ctx context.Context, lease *Lease, wg *sync.WaitGroup) error {
	condition := v1.NodeCondition{
		Type:               v1.NodeNetworkUnavailable,
		Status:             v1.ConditionFalse,
		Reason:             "FlannelIsUp",
		Message:            "Flannel is running on this node",
		LastTransitionTime: metav1.Now(),
		LastHeartbeatTime:  metav1.Now(),
	}
	raw, err := json.Marshal(&[]v1.NodeCondition{condition})
	if err != nil {
		return err
	}
	patch := []byte(fmt.Sprintf(`{"status":{"conditions":%s}}`, raw))
	_, err = ksm.client.CoreV1().Nodes().PatchStatus(ctx, ksm.nodeName, patch)
	return err
}

// flannel/pkg/subnet/kube/kube.go
// HandleSubnetFile writes the configuration file used by the CNI flannel plugin
// and stores the immutable data in a dedicated struct of the subnet manager
// so that we can update the file later when a clustercidr resource is created.
//func (m *kubeSubnetManager) HandleSubnetFile(path string, config *subnet.Config, ipMasq bool, sn ip.IP4Net, ipv6sn ip.IP6Net, mtu int) error {
//	m.snFileInfo = &subnetFileInfo{
//		path:   path,
//		ipMask: ipMasq,
//		mtu:    mtu,
//	}
//	return subnet.WriteSubnetFile(path, config, ipMasq, sn, ipv6sn, mtu)
//}

// GetStoredMacAddresses reads MAC addresses from node annotations when flannel restarts
func (ksm *kubeSubnetManager) GetStoredMacAddresses(ctx context.Context) (string, string) {
	var macv4, macv6 string
	// get mac info from Name func.
	node, err := ksm.client.CoreV1().Nodes().Get(ctx, ksm.nodeName, metav1.GetOptions{})
	if err != nil {
		log.Errorf("Failed to get node for backend data: %v", err)
		return "", ""
	}

	// node backend data format: `{"VNI":1,"VtepMAC":"12:c6:65:89:b4:e3"}`
	// and we will return only mac addr str like 12:c6:65:89:b4:e3
	if node != nil && node.Annotations != nil {
		log.Infof("List of node(%s) annotations: %#+v", ksm.nodeName, node.Annotations)
		backendData, ok := node.Annotations[fmt.Sprintf("%s/backend-data", ksm.annotationPrefix)]
		if ok {
			macStr := strings.Trim(backendData, "\"}")
			macInfoSlice := strings.Split(macStr, ":\"")
			if len(macInfoSlice) == 2 {
				macv4 = macInfoSlice[1]
			}
		}
		backendDatav6, okv6 := node.Annotations[fmt.Sprintf("%s/backend-v6-data", ksm.annotationPrefix)]
		if okv6 {
			macStr := strings.Trim(backendDatav6, "\"}")
			macInfoSlice := strings.Split(macStr, ":\"")
			if len(macInfoSlice) == 2 {
				macv6 = macInfoSlice[1]
			}
		}
		return macv4, macv6
	}

	return "", ""
}
