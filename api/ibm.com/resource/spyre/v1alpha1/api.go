/*
 * Copyright 2024 The IBM Authors.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

type ClaimParameters struct {
	metav1.TypeMeta `json:",inline"`
	Config          SpyreConfig `json:"config,omitempty"`
}

// Decoder implements a decoder for objects in this API group.
var Decoder runtime.Decoder

// DefaultParams provides the default GPU configuration.
func DefaultParams() *ClaimParameters {
	return &ClaimParameters{
		TypeMeta: metav1.TypeMeta{
			APIVersion: GroupName + "/" + Version,
			Kind:       ClaimParamKind,
		},
	}
}
