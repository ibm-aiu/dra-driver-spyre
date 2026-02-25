// (C) Copyright IBM Corp.  2025,2026 IBM Corp.
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

package state

import (
	"encoding/json"

	"github.com/ibm-aiu/dra-driver-spyre/pkg/types"
	"k8s.io/kubernetes/pkg/kubelet/checkpointmanager/checksum"
)

type Checkpoint struct {
	Checksum checksum.Checksum `json:"checksum"`
	V1       *CheckpointV1     `json:"v1,omitempty"`
}

type CheckpointV1 struct {
	PreparedClaims types.PreparedClaims `json:"preparedClaims,omitempty"`
}

func newCheckpoint() *Checkpoint {
	pc := &Checkpoint{
		Checksum: 0,
		V1: &CheckpointV1{
			PreparedClaims: make(types.PreparedClaims),
		},
	}
	return pc
}

func (cp *Checkpoint) MarshalCheckpoint() ([]byte, error) {
	cp.Checksum = 0
	out, err := json.Marshal(*cp)
	if err != nil {
		return nil, err
	}
	cp.Checksum = checksum.New(out)
	return json.Marshal(*cp)
}

func (cp *Checkpoint) UnmarshalCheckpoint(data []byte) error {
	return json.Unmarshal(data, cp)
}

func (cp *Checkpoint) VerifyChecksum() error {
	ck := cp.Checksum
	cp.Checksum = 0
	defer func() {
		cp.Checksum = ck
	}()
	out, err := json.Marshal(*cp)
	if err != nil {
		return err
	}
	return ck.Verify(out)
}
