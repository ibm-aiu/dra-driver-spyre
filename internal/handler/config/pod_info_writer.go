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

package config

import (
	"os"
	"path/filepath"

	corev1 "k8s.io/api/core/v1"
)

const (
	PodNameFile      = "POD_NAME"
	PodNamespaceFile = "POD_NAMESPACE"
)

func writeInfoFiles(metricsFolder string, pod corev1.Pod) error {
	// write pod name
	if err := WriteFile(metricsFolder, PodNameFile, pod.Name); err != nil {
		return err
	}
	// write pod namespace
	return WriteFile(metricsFolder, PodNamespaceFile, pod.Namespace)
}

func WriteFile(folder, filename, value string) error {
	path := filepath.Join(folder, filename)
	file, err := os.Create(path)
	if err != nil {
		return err
	}
	defer file.Close() //nolint:errcheck
	_, err = file.WriteString(value)
	return err
}
