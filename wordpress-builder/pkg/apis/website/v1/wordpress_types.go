/*

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

// Domain represents a valid domain name
type Domain string

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// WordpressSpec defines the desired state of Wordpress
type WordpressSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "make" to regenerate code after modifying this file
	Image string `json:"image"`
	// Image tag to use. Defaults to latest
	// +optional
	Tag  string `json:"tag,omitempty"`
	Port string `json:"port"`

	Env []corev1.EnvVar `json:"env" patchStrategy:"merge" patchMergeKey:"name"`
	// +optional
	EnvFrom        []corev1.EnvFromSource `json:"envFrom,omitempty"`
	DeploymentName string                 `json:"deploymentName"`
	Commands       []string               `json:"commands,omitempty"`
	// // Domains for which this this site answers.
	// // The first item is set as the "main domain" (eg. WP_HOME and WP_SITEURL constants).
	// // +kubebuilder:validation:MinItems=1
	Domains []Domain `json:"domains,omitempty"`
}

// WordpressStatus defines the observed state of Wordpress
type WordpressStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file
	AvailableReplicas int32 `json:"availableReplicas"`
	// ActionHistory     []string `json:"actionHistory"`
	// VerifyCmd         string   `json:"verifyCommand"`
	// ServiceIP         string   `json:"serviceIP"`
	// ServicePort       string   `json:"servicePort"`
	Status string `json:"status"`
}

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// Wordpress is the Schema for the wordpresses API
// +k8s:openapi-gen=true
// +kubebuilder:subresource:status
// +kubebuilder:subresource:scale:specpath=.spec.replicas,statuspath=.status.replicas
type Wordpress struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   WordpressSpec   `json:"spec,omitempty"`
	Status WordpressStatus `json:"status,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// WordpressList contains a list of Wordpress
type WordpressList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Wordpress `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Wordpress{}, &WordpressList{})
}
