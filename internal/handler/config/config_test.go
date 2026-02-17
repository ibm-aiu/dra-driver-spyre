// Copyright 2026 IBM Corp.
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

package config_test

import (
	"os"
	"path/filepath"
	"strings"

	. "github.com/ibm-aiu/dra-driver-spyre/internal/handler/config"
	cst "github.com/ibm-aiu/dra-driver-spyre/pkg/const"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Config", func() {
	var _ = BeforeEach(func() {
		os.Setenv(cst.TemplatePathKey, TestTemplatePath)
		os.Setenv(cst.ConfigHostPathKey, TestConfigHostPath)
		os.Setenv(cst.MetricsHostPathKey, TestMetricsHostPath)
		var err error
		Handler, err = InitConfigMount()
		Expect(err).To(BeNil())
	})

	var _ = AfterEach(func() {
		err := os.RemoveAll(TestConfigHostPath)
		Expect(err).To(BeNil())
		err = os.RemoveAll(TestMetricsHostPath)
		Expect(err).To(BeNil())
	})

	It("can get correct mount for config and metric", func() {
		mnts, err := Handler.GetConfigMetricsMount("", PseudoBusIds)
		Expect(err).To(BeNil())
		Expect(len(mnts)).To(Equal(2))
		checkParentHostPath(mnts[0].HostPath, TestConfigHostPath)
		Expect(mnts[0].ContainerPath).To(BeEquivalentTo(ConfigContainerPath))
		checkParentHostPath(mnts[1].HostPath, TestMetricsHostPath)
		Expect(mnts[1].ContainerPath).To(BeEquivalentTo(MetricsContainerPath))
	})
})

func checkParentHostPath(path string, expectedParent string) {
	hostPathSplit := strings.Split(path, "/")
	Expect(len(hostPathSplit)).To(BeNumerically(">", 1))
	parentHostPath := filepath.Join(hostPathSplit[0 : len(hostPathSplit)-1]...)
	Expect(parentHostPath).Should(BeEquivalentTo(expectedParent))
}
