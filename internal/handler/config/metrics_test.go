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

package config_test

import (
	"os"
	"path/filepath"

	. "github.com/ibm-aiu/dra-driver-spyre/internal/handler/config"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
)

var _ = Describe("Metrics", func() {
	DescribeTable("GetUuidFromPath", func(configHostPath, expected string) {
		uuidValue := GetUuidFromPath(configHostPath)
		Expect(uuidValue).To(Equal(expected))
	},
		Entry("one-level folder", "config-host-path/some-uuid-value", "some-uuid-value"),
		Entry("two-level folder", "top-folder/config-host-path/some-uuid-value", "some-uuid-value"),
		Entry("non-level", "some-uuid-value", "some-uuid-value"),
		Entry("empty", "", ""),
	)

	It("write/read pod info", func() {
		pod := &corev1.Pod{}
		name := "test-pod"
		namespace := "test-ns"
		configHostPath := "config-host-path/some-uuid-value"
		pod.Name = name
		pod.Namespace = namespace
		metricsFolder, err := CreateNewMetricsFolder(TestMetricsHostPath, configHostPath)
		Expect(err).To(BeNil())
		Expect(metricsFolder).To(Equal("metrics-host-path/some-uuid-value"))
		err = WriteInfoFiles(metricsFolder, *pod)
		Expect(err).To(BeNil())
		createdPodName, err := ReadStringInFile(metricsFolder, PodNameFile)
		Expect(err).To(BeNil())
		Expect(createdPodName).To(Equal(name))
		createdPodNamespace, err := ReadStringInFile(metricsFolder, PodNamespaceFile)
		Expect(err).To(BeNil())
		Expect(createdPodNamespace).To(Equal(namespace))
		err = os.RemoveAll(metricsFolder)
		Expect(err).To(BeNil())
	})
})

func ReadStringInFile(folder, filename string) (string, error) {
	path := filepath.Join(folder, filename)
	content, err := os.ReadFile(path)
	if err == nil {
		return string(content), nil
	}
	return "", err
}
