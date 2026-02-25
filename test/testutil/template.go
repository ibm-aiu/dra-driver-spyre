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
	"html/template"
	"os"

	. "github.com/onsi/gomega"
)

func YamlFromTemplate(tmpl string, data any) (yamlPathName string) {
	manifestTmpl, err := template.New("template").Parse(tmpl)
	Expect(err).To(BeNil(), "Error parsing template: %v", err)

	file, err := os.CreateTemp("", "manifest-*.yaml")
	Expect(err).To(BeNil(), "Error creating template file: %v", err)

	err = manifestTmpl.Execute(file, data)
	Expect(err).To(BeNil(), "Error executing template: %v", err)

	return file.Name()
}
