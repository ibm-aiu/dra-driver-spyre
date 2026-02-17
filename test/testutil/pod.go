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

// pod.go defines functions to handle Pod resource and their corresponding variables and constants

package testutil

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	conf "github.com/ibm-aiu/dra-driver-spyre/internal/handler/config"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

const (
	ubi9MicroTestImage = "registry.access.redhat.com/ubi9/ubi-micro:latest"
)

const PodWithResourceClaimTemplate = `
apiVersion: v1
kind: Pod
metadata:
  name: {{ .Name }}
  namespace: {{ .Namespace }}
spec:
  containers:
  - name: app
    image: {{ .Image }}
    imagePullPolicy: IfNotPresent
    {{- if .Arg0 }}
    command: ["/bin/bash", "-c"]
    args:
    - "{{ .Arg0 }}"
    {{- else }}
    command: ["tail", "-f", "/dev/null"]
    {{- end }}
    resources:
      claims:
      - name: spyre
  resourceClaims:
  - name: spyre
    resourceClaimTemplateName: {{ .ResourceClaimTemplateName }}
  terminationGracePeriodSeconds: 0
  {{- if .NodeSelectorNode }}
  nodeSelector:
    "kubernetes.io/hostname": {{ .NodeSelectorNode }}
  {{- end }}
`

// PodTemplateData holds data used to populate a Kubernetes Pod template.
//
// Example usage:
//
//	data := BasicPodTemplateData(name, namespace)
//	data = data.SetImage(image) // default: ubi-micro image
//	data = data.SetNode(node)   // default: not set
//	data = data.SetArg0(arg0)   // default: not set
type PodTemplateData struct {
	Name             string
	Namespace        string
	Image            string
	Arg0             string
	NodeSelectorNode string
}

// BasicPodTemplateData sets default ubi image without node or arg0
func BasicPodTemplateData(name, namespace string) *PodTemplateData {
	return &PodTemplateData{
		Name:      name,
		Namespace: namespace,
		Image:     ubi9MicroTestImage,
	}
}

func (p *PodTemplateData) SetImage(image string) *PodTemplateData {
	p.Image = image
	return p
}

func (p *PodTemplateData) SetNode(node string) *PodTemplateData {
	p.NodeSelectorNode = node
	return p
}

func (p *PodTemplateData) SetArg0(arg0 string) *PodTemplateData {
	p.Arg0 = arg0
	return p
}

// PodWithResourceClaimTemplateData holds basic PodTemplateData and ResourceClaimTemplateName
type PodWithResourceClaimTemplateData struct {
	PodTemplateData
	ResourceClaimTemplateName string
}

// BuildPod creates a pod with common command to print senlib config
// Always check nil error.
func BuildPod(ctx context.Context, dynClient *dynamic.DynamicClient, discoClient *discovery.DiscoveryClient,
	data *PodTemplateData, claimName string) {
	buildPod(ctx, dynClient, discoClient, PodWithResourceClaimTemplate, data, claimName)
}

// buildPod creates a pod with the defined Arg0.
// If Arg0 is "", it sets the command to ["tail", "-f", "/dev/null"].
// Always check nil error.
func buildPod(ctx context.Context, dynClient *dynamic.DynamicClient, discoClient *discovery.DiscoveryClient,
	template string, data *PodTemplateData, claimName string) {
	var yamlData string
	if claimName == "" {
		yamlData = YamlFromTemplate(template, *data)
	} else {
		dataWithClaim := PodWithResourceClaimTemplateData{
			PodTemplateData:           *data,
			ResourceClaimTemplateName: claimName,
		}
		yamlData = YamlFromTemplate(template, dataWithClaim)
	}
	_, err := CreateResourceFromYaml(ctx, dynClient, discoClient, data.Namespace, yamlData)
	Expect(err).To(BeNil())
}

// CheckAndGetAllocationsFromPodLog reads pod log,
// parses senlib config, and checks DOOM mode (GENERAL.doom).
//
// Return: allocated PCI addresses
func CheckAndGetAllocationsFromPodLog(ctx context.Context, k8sClientset *kubernetes.Clientset, name, namespace string, vf bool) []string {
	senlibConfig := getSenlibConfig(ctx, k8sClientset, name, namespace)
	Expect(senlibConfig.General.Doom).To(Equal(vf))
	return senlibConfig.General.PciAddresses
}

// GetSenlibConfig gets pod log and parses senlib config.
func getSenlibConfig(ctx context.Context, k8sClientset *kubernetes.Clientset, name, namespace string) conf.SenlibConfig {
	req := k8sClientset.CoreV1().Pods(namespace).GetLogs(name, &v1.PodLogOptions{Container: "app"})
	podLog, err := req.Stream(ctx)
	Expect(err).To(BeNil())

	buf := new(bytes.Buffer)
	_, err = io.Copy(buf, podLog)
	Expect(err).To(BeNil())

	var senlibConfig conf.SenlibConfig
	err = json.Unmarshal(buf.Bytes(), &senlibConfig)
	Expect(err).To(BeNil())
	return senlibConfig
}

// CheckPodPhases checks status of multiple pods
func CheckPodPhases(ctx context.Context, k8sClientset *kubernetes.Clientset, pods []*PodTemplateData, expectedStateNum map[v1.PodPhase]int) {
	if len(expectedStateNum) > 0 {
		namespace := pods[0].Namespace
		By(fmt.Sprintf("Waiting for %v", expectedStateNum))
		Eventually(func(g Gomega) {
			count := make(map[v1.PodPhase]int)
			for _, pod := range pods {
				pod, err := k8sClientset.CoreV1().Pods(namespace).Get(ctx, pod.Name, metav1.GetOptions{})
				g.Expect(err).To(BeNil())
				if _, found := count[pod.Status.Phase]; !found {
					count[pod.Status.Phase] = 0
				}
				count[pod.Status.Phase] += 1
				if pod.Status.Phase != v1.PodRunning && pod.Status.Phase != v1.PodSucceeded {
					if message := getPodMessage(*pod); message != "" {
						log.Log.Info("pod is not running", "name", pod.Name, "namespace", pod.Namespace, "phase", pod.Status.Phase, "message", message)
					}
				}
			}
			for phase, num := range expectedStateNum {
				if num == 0 {
					continue
				}
				countNum, found := count[phase]
				g.Expect(found).To(BeTrue())
				g.Expect(countNum).To(Equal(num))
			}
		}).WithTimeout(7 * time.Minute).WithPolling(10 * time.Second).Should(Succeed())
	}
}

func getPodMessage(pod v1.Pod) string {
	message := ""
	if len(pod.Status.ContainerStatuses) > 0 {
		if pod.Status.ContainerStatuses[0].State.Waiting != nil {
			message = pod.Status.ContainerStatuses[0].State.Waiting.Message
		}
	}
	return message
}

func DeletePod(ctx context.Context, k8sClientset *kubernetes.Clientset, pod *PodTemplateData) {
	err := k8sClientset.CoreV1().Pods(pod.Namespace).Delete(ctx, pod.Name, metav1.DeleteOptions{})
	By(fmt.Sprintf("deleting pod %s/%s: %v", pod.Name, pod.Namespace, err))
	if err != nil {
		Expect(errors.IsNotFound(err)).To(BeTrue())
		return
	}
	Eventually(func(g Gomega) {
		_, err := k8sClientset.CoreV1().Pods(pod.Namespace).Get(ctx, pod.Name, metav1.GetOptions{})
		g.Expect(errors.IsNotFound(err)).To(BeTrue())
	}).WithTimeout(3 * time.Minute).WithPolling(10 * time.Second).Should(Succeed())
}

func GetPodsWithLabels(ctx context.Context, k8sClientset *kubernetes.Clientset, g Gomega, namespace, label string, nodeName string) []v1.Pod {
	listOptions := metav1.ListOptions{
		LabelSelector: label,
	}
	// add field selector of spec.nodeName to the list option only the target node if specified
	if nodeName != "" {
		listOptions.FieldSelector = fields.Set{"spec.nodeName": nodeName}.AsSelector().String()
	}
	pods, err := k8sClientset.CoreV1().Pods(namespace).List(ctx, listOptions)
	g.Expect(err).To(BeNil())
	return pods.Items
}
