/*
Copyright 2017 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package v1

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type SecretRef string

// +genclient
// +genclient:noStatus
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// Website is a specification for a Website resource
type Website struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   WebsiteSpec   `json:"spec"`
	Status WebsiteStatus `json:"status"`
}

// WebsiteSpec is the spec for a Website resource
type WebsiteSpec struct {
	Replicas *int32 `json:"replicas"`

	Image string          `json:"image"`
	Env   []corev1.EnvVar `json:"env" patchStrategy:"merge" patchMergeKey:"name"`
	// +optional
	EnvFrom        []corev1.EnvFromSource `json:"envFrom,omitempty"`
	DeploymentName string                 `json:"deploymentName"`
	Commands       []string               `json:"commands"`
}

// WebsiteStatus is the status for a Website resource
type WebsiteStatus struct {
	AvailableReplicas int32    `json:"availableReplicas"`
	ActionHistory     []string `json:"actionHistory"`
	VerifyCmd         string   `json:"verifyCommand"`
	ServiceIP         string   `json:"serviceIP"`
	ServicePort       string   `json:"servicePort"`
	Status            string   `json:"status"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// WebsiteList is a list of Website resources
type WebsiteList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`

	Items []Website `json:"items"`
}
