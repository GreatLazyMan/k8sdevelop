package hack

import (
	_ "github.com/gogo/protobuf/gogoproto"
	_ "github.com/gogo/protobuf/jsonpb"
	_ "github.com/gogo/protobuf/proto"
	_ "github.com/gogo/protobuf/protoc-gen-gofast"
	_ "github.com/gogo/protobuf/protoc-gen-gogo"
	_ "k8s.io/apimachinery"
	_ "k8s.io/apimachinery/pkg/apis/testapigroup/v1"
	_ "k8s.io/code-generator"
)
