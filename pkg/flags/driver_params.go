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

package flags

import (
	cli "github.com/urfave/cli/v2"
)

type DiscoveryConfig struct {
	TopologyFilepath string
}

func (c *DiscoveryConfig) Flags() []cli.Flag {
	flags := []cli.Flag{
		&cli.StringFlag{
			Category:    "Discovery config:",
			Name:        "topology-file",
			Usage:       "File location of device topology.",
			Destination: &c.TopologyFilepath,
			EnvVars:     []string{"TOPOLOGY_FILE"},
		},
	}
	return flags
}
