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

package testutil

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	k8sErrs "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

// CreateRandomTestNamespace generates test-<uuid> random namespace and returns
func CreateRandomTestNamespace(ctx context.Context, k8sClientset *kubernetes.Clientset) string {
	testNamespace := "test-" + uuid.NewString()
	Eventually(func(g Gomega) {
		err := CreateNamespace(ctx, k8sClientset, testNamespace)
		if err != nil {
			g.Expect(k8sErrs.IsAlreadyExists(err)).To(BeTrue())
		}
	}).WithTimeout(120 * time.Second).WithPolling(5 * time.Second).Should(Succeed())
	return testNamespace
}

func DeleteNamespace(ctx context.Context, clientset *kubernetes.Clientset, namespaceName string) error {
	err := clientset.CoreV1().Namespaces().Delete(ctx, namespaceName, metav1.DeleteOptions{})
	if err != nil {
		return fmt.Errorf("failed to delete the project %s: %w", namespaceName, err)
	}
	return nil
}

func CreateNamespace(ctx context.Context, clientset *kubernetes.Clientset, namespaceName string) error {
	namespace := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: namespaceName,
		},
	}
	_, err := clientset.CoreV1().Namespaces().Create(ctx, namespace, metav1.CreateOptions{})
	if err != nil {
		return fmt.Errorf("failed to create the namespace %s: %w", namespace, err)
	}
	return nil
}
