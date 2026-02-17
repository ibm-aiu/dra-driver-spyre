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

package testutil

import (
	"context"
	"fmt"
	"os"

	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/runtime/serializer/yaml"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/discovery/cached/memory"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/restmapper"
)

func CreateResource(ctx context.Context, dynClient dynamic.Interface, namespace string, mapObj map[string]interface{}, resourceName string) (*unstructured.Unstructured, error) {
	obj := &unstructured.Unstructured{Object: mapObj}
	gvr, _ := schema.ParseResourceArg(resourceName)
	object, err := dynClient.Resource(*gvr).Namespace(namespace).Create(ctx, obj, metav1.CreateOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to create resource %s in namespace %s: %w", resourceName, namespace, err)
	}
	return object, nil
}

func DeleteResource(ctx context.Context, dynClient dynamic.Interface, namespace, name string, resourceName string) error {
	gvr, _ := schema.ParseResourceArg(resourceName)
	err := dynClient.Resource(*gvr).Namespace(namespace).Delete(ctx, name, metav1.DeleteOptions{})
	if err != nil {
		return fmt.Errorf("failed to delete resource %s in namespace %s: %w", resourceName, namespace, err)
	}
	return nil
}

func GetResource(ctx context.Context, dynClient dynamic.Interface, namespace, name string, resourceName string) (*unstructured.Unstructured, error) {
	gvr, _ := schema.ParseResourceArg(resourceName)
	var object *unstructured.Unstructured
	var err error
	if namespace != "" {
		object, err = dynClient.Resource(*gvr).Namespace(namespace).Get(ctx, name, metav1.GetOptions{})
		if err != nil {
			return nil, fmt.Errorf("failed to get resource %s in namespace %s: %w", resourceName, namespace, err)
		}
	} else {
		object, err = dynClient.Resource(*gvr).Get(ctx, name, metav1.GetOptions{})
		if err != nil {
			return nil, fmt.Errorf("failed to get resource %s: %w", resourceName, err)
		}
	}
	return object, nil
}

func DecodeYAML(ymlfile string) (*unstructured.Unstructured, *schema.GroupVersionKind, error) {
	var decoder = yaml.NewDecodingSerializer(unstructured.UnstructuredJSONScheme)
	obj := &unstructured.Unstructured{}
	yml, err := os.ReadFile(ymlfile)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to read the yml file %s: %w ", ymlfile, err)
	}
	_, gvk, err := decoder.Decode(yml, nil, obj)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to decode the content into an object: %w", err)
	}
	return obj, gvk, nil
}

func GvrMap(discoClient *discovery.DiscoveryClient, gvk *schema.GroupVersionKind) (*meta.RESTMapping, error) {
	// Check server has a matching gvr given a gvk
	mapper := restmapper.NewDeferredDiscoveryRESTMapper(memory.NewMemCacheClient(discoClient))
	mapping, err := mapper.RESTMapping(gvk.GroupKind(), gvk.Version)
	if err != nil {
		return nil, fmt.Errorf("failed to find a gvr for the gvk %v: %w", gvk, err)
	}
	return mapping, nil
}

func CreateResourceFromYaml(ctx context.Context, dynClient *dynamic.DynamicClient, discoClient *discovery.DiscoveryClient, ns string, ymlfile string) (*unstructured.Unstructured, error) {
	obj, gvk, err := DecodeYAML(ymlfile)
	if err != nil {
		return nil, err
	}
	rm, err := GvrMap(discoClient, gvk)
	if err != nil {
		return nil, err
	}
	object, err := dynClient.Resource(rm.Resource).Namespace(ns).Create(ctx, obj, metav1.CreateOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to create resource %v in namespace %s: %w", rm.Resource, ns, err)
	}
	return object, nil
}
