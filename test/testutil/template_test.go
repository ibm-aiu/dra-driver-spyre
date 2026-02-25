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

package testutil

import (
	"os"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"sigs.k8s.io/controller-runtime/pkg/log"
)

var _ = Describe("template test", Ordered, func() {
	It("template test", func() {
		podData := BasicPodTemplateData("test-pod", "default").SetArg0(PrintSenlibConfig)
		claimName := "test-claim"
		claimData := BasicResourceClaimTemplateData(claimName, "default").SetCount(1)
		claimYml := YamlFromTemplate(ResourceClaimTemplate, *claimData)
		podWithClaimData := PodWithResourceClaimTemplateData{
			PodTemplateData:           *podData,
			ResourceClaimTemplateName: claimName,
		}
		podYml := YamlFromTemplate(PodWithResourceClaimTemplate, podWithClaimData)
		log.Log.Info("YamlFromTemplate", "claimYml", claimYml, "podYml", podYml)
		err := os.Remove(podYml)
		Expect(err).NotTo(HaveOccurred())
		err = os.Remove(claimYml)
		Expect(err).NotTo(HaveOccurred())
	})
})
