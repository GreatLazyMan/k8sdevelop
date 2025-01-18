package v1beta1

import metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

// +genclient
// +k8s:openapi-gen=true
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

type Foo struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty" protobuf:"bytes,1,opt,name=metadata"`

	Spec   FooSpec   `json:"spec" protobuf:"bytes,2,opt,name=spec"`
	Status FooStatus `json:"status,omitempty" protobuf:"bytes,3,opt,name=status"`
}

type FooSpec struct {
	// Container image that the container is running to do our foo work
	Image string `json:"image" protobuf:"bytes,1,opt,name=image"`
	// Config is the configuration used by foo container
	Config FooConfig `json:"config" protobuf:"bytes,2,opt,name=config"`
}

type FooConfig struct {
	// Msg says hello world!
	Msg string `json:"msg" protobuf:"bytes,1,opt,name=msg"`
	// Msg1 provides some verbose information
	// +optional
	Msg1 string `json:"msg1,omitempty" protobuf:"bytes,2,opt,name=msg1"`
}

// FooPhase is a label for the condition of a foo at the current time.
type FooPhase string

const (
	// FooPhaseProcessing means the pod has been accepted by the controllers, but one or more desire has not been synchorinzed
	FooPhaseProcessing FooPhase = "Processing"
	// FooPhaseReady means all conditions of foo have been meant
	FooPhaseReady FooPhase = "Ready"
)

type FooStatus struct {
	// The phase of a Foo is a simple, high-level summary of where the Foo is in its lifecycle
	// +optional
	Phase FooPhase `json:"phase,omitempty" protobuf:"bytes,1,opt,name=phase,casttype=FooPhase"`

	/*
			+API rule violation: list_type_missing,github.com/greatlazyman/apiserver/pkg/apis/simple.io/v1beta1,FooStatus,Conditions
			 API rule violation: names_match,k8s.io/apimachinery/pkg/apis/meta/v1,APIResourceList,APIResources
			 API rule violation: names_match,k8s.io/apimachinery/pkg/apis/meta/v1,Duration,Duration
			 API rule violation: names_match,k8s.io/apimachinery/pkg/apis/meta/v1,InternalEvent,Object
			+ echo -e ERROR:
			ERROR:
			+ echo -e '\tAPI rule check failed for /root/learn/k8sdevelop/simpleapiserver/apiserver/hack/../hack/apiextensions_violation_exceptions.list: new reported violations'
				API rule check failed for /root/learn/k8sdevelop/simpleapiserver/apiserver/hack/../hack/apiextensions_violation_exceptions.list: new reported violations
			+ echo -e '\tPlease read api/api-rules/README.md'
				Please read api/api-rules/README.md
			+ return 1

			fix: https://github.com/kubernetes/kube-openapi/issues/175
		otther:
		https://github.com/kubernetes/kubernetes/blob/36981002246682ed7dc4de54ccc2a96c1a0cbbdb/api/api-rules/violation_exceptions.list
	*/
	// Represents the latest available observations of a foo's current state
	// +optional
	// +patchMergeKey=type
	// +patchStrategy=merge
	// +listType=atomic
	Conditions []FooCondition `json:"conditions,omitempty" patchStrategy:"merge" patchMergeKey:"type" protobuf:"bytes,2,rep,name=conditions"`
}

type FooConditionType string

const (
	FooConditionTypeWorker FooConditionType = "Worker"
	FooConditionTypeConfig FooConditionType = "Config"
)

type FooCondition struct {
	Type   FooConditionType       `json:"type" protobuf:"bytes,1,opt,name=type,casttype=FooConditionType"`
	Status metav1.ConditionStatus `json:"status" protobuf:"bytes,2,opt,name=status,casttype=k8s.io/apimachinery/pkg/apis/meta/v1.ConditionStatus"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

type FooList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty" protobuf:"bytes,1,opt,name=metadata"`

	Items []Foo `json:"items" protobuf:"bytes,2,rep,name=items"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// Config is the config subresource of Foo
type Config struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty" protobuf:"bytes,1,opt,name=metadata"`

	Spec ConfigSpec `json:"spec" protobuf:"bytes,2,opt,name=spec"`
}

type ConfigSpec struct {
	// Msg says hello world!
	Msg string `json:"msg" protobuf:"bytes,1,opt,name=msg"`
	// Msg1 provides some verbose information
	// +optional
	Msg1 string `json:"msg1,omitempty" protobuf:"bytes,2,opt,name=msg1"`
}
