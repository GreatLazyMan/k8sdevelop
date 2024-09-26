// Copyright 2022 flannel authors
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
	"bytes"
	"encoding/json"
	"fmt"
	"net"
)

const (
	EventAdded EventType = iota
	EventRemoved
)

type (
	EventType int

	Event struct {
		Type  EventType `json:"type"`
		Lease Lease     `json:"lease,omitempty"`
	}
)

// LeaseAttrs includes extra information for the lease
type LeaseAttrs struct {
	BackendType   string          `json:",omitempty"`
	BackendData   json.RawMessage `json:",omitempty"`
	BackendV6Data json.RawMessage `json:",omitempty"`
}

// Lease includes information about the lease
type Lease struct {
	CidrIPv4        []*net.IPNet
	CidrIPv6        []*net.IPNet
	ClusterCidrIPv4 *net.IPNet
	ClusterCidrIPv6 *net.IPNet
	Attrs           LeaseAttrs
	AttrMap         map[string]string
}

func (la *LeaseAttrs) String() string {
	var buffer bytes.Buffer
	if la.BackendData != nil {
		j, err := json.Marshal(la.BackendData)
		if err != nil {
			buffer.WriteString("BackendData: (nil), ")
		}
		buffer.WriteString(fmt.Sprintf("BackendData: %s, ", string(j)))
	} else {
		buffer.WriteString("BackendData: (nil), ")
	}
	if la.BackendV6Data != nil {
		j, err := json.Marshal(la.BackendV6Data)
		if err != nil {
			buffer.WriteString("BackendV6Data: (nil)")
		}
		buffer.WriteString(fmt.Sprintf("BackendV6Data: %s", string(j)))
	} else {
		buffer.WriteString("BackendV6Data: (nil)")
	}
	return buffer.String()
}
