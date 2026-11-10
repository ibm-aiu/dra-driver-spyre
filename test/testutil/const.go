// (C) Copyright IBM Corp. 2025,2026
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

// const.go defines common constants for testing

package testutil

import "github.com/ibm-aiu/dra-driver-spyre/pkg/utils"

const PrintSenlibConfig = "cat /etc/aiu/senlib_config.json;echo;tail -f /dev/null"

// ListMetricsFiles returns a command that lists file names in the metrics mount path, one per line.
func ListMetricsFiles() string {
	return "ls -1 " + utils.GetMetricsContainerPath() + ";tail -f /dev/null"
}
