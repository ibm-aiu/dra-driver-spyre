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

package pcitopo

import (
	"encoding/json"
	"fmt"
	"os"
	"strconv"
)

const (
	// metadata's keys
	ClockKey           = "Clocks"
	ClockRPDKey        = "RPD"
	ClockSOCKey        = "SOC"
	MemoryKey          = "Memory"
	MemoryBoostableKey = "Boostable"
	MemoryFrequencyKey = "Freq"
	MemoryVendorKey    = "Make"
	MemorySizeKey      = "Size"
	MemorySpeedKey     = "Speed"
)

type Pcitopo struct {
	Timestamp         string            `json:"timestamp,omitempty"`
	Version           float32           `json:"version,omitempty"` // this must be string!
	Server            string            `json:"server,omitempty"`
	NumDevices        int               `json:"num_devices,omitempty"`
	Devices           map[string]Device `json:"devices,omitempty"`
	SpyreVfNumDevices int               `json:"spyre_vf_num_devices,omitempty"`
	SpyreVfDevices    map[string]Device `json:"spyre_vf_devices,omitempty"`
}

type Device struct {
	Name         string         `json:"name,omitempty"`
	NumaNode     int            `json:"numanode,omitempty"`
	Linkspeed    string         `json:"linkspeed,omitempty"`
	Peers        Peers          `json:"peers,omitempty"`
	SpyreVfPeers Peers          `json:"spyre_vf_peers,omitempty"`
	DeviceId     string         `json:"device_id,omitempty"`
	IsPf         bool           `json:"is_pf,omitempty"`
	Metadata     map[string]any `json:"metadata,omitempty"`
}

type Peers struct {
	Peer0 map[string]int `json:"peers_0,omitempty"`
	Peer1 map[string]int `json:"peers_1,omitempty"`
	Peer2 map[string]int `json:"peers_2,omitempty"`
}

func (t Pcitopo) Write(filepath string) error {
	outputFile, err := os.Create(filepath)
	if err != nil {
		return fmt.Errorf("failed to create pcitopo file: %s: %w", filepath, err)
	}
	defer outputFile.Close() //nolint:errcheck
	jsonData, err := json.MarshalIndent(t, "", "    ")
	if err != nil {
		return fmt.Errorf("failed to marshal pcitopo data: %w", err)
	}
	if _, err = outputFile.Write(jsonData); err != nil {
		return fmt.Errorf("failed to write pcitopo file: %s: %w", filepath, err)
	}
	return nil
}

func (t Pcitopo) String() string {
	if jsonData, err := json.Marshal(t); err != nil {
		return ""
	} else {
		return string(jsonData)
	}
}

func (t Pcitopo) GetDevices() []string {
	devices := []string{}
	for device := range t.Devices {
		devices = append(devices, device)
	}
	return devices
}

// UnmarshalPciTopo extracts "version" attribute if exists and calls corresponding convert function.
// returns default format of Pcitopo and error if exists.
func UnmarshalPciTopo(data []byte) (Pcitopo, error) {
	var pcitopo Pcitopo
	if err := json.Unmarshal(data, &pcitopo); err != nil {
		return pcitopo, fmt.Errorf("failed to unmarshal pcitopo data: %w", err)
	}
	return pcitopo, nil
}

// GenerateNumaInfoMapFromTopo returns mapping from pci address to generated index of numa
func GenerateNumaInfoMapFromTopo(topo Pcitopo) map[string]string {
	numaMap := make(map[string]string)
	numai := 0
	for pciAddress, deviceInfo := range topo.Devices {
		// device.Numanode is always zero in pseudo device.
		// TODO: simplify logic by reading deviceInfo.Numanode when device plugin's PseudoPciDevice supports numa info mock.

		// Algorithm to set NUMA:
		// 	If not exist in the map, consider as a new NUMA and add all members in peer0
		if _, found := numaMap[pciAddress]; !found {
			numaMap[pciAddress] = strconv.Itoa(numai)
			for peer := range deviceInfo.Peers.Peer0 {
				numaMap[peer] = strconv.Itoa(numai)
			}
			numai += 1
		}
	}
	return numaMap
}
