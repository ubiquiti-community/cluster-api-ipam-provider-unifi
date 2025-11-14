/*
Copyright 2024.

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

package v1alpha1

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// UnifiInstanceSpec defines the desired state of UnifiInstance.
type UnifiInstanceSpec struct {
	// Host is the URL of the Unifi controller
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:Pattern=`^https?://`
	Host string `json:"host"`

	// CredentialsRef references a Secret containing username and password
	// +kubebuilder:validation:Required
	CredentialsRef corev1.LocalObjectReference `json:"credentialsRef"`

	// Site is the Unifi site name (defaults to "default")
	// +optional
	// +kubebuilder:default="default"
	Site string `json:"site,omitempty"`

	// Insecure allows insecure HTTPS connections (skip TLS verification)
	// +optional.
	Insecure bool `json:"insecure,omitempty"`
}

// UnifiInstanceStatus defines the observed state of UnifiInstance.
type UnifiInstanceStatus struct {
	// Ready indicates whether the instance is ready for use.
	Ready bool `json:"ready"`

	// Conditions define the current state of the UnifiInstance
	// +optional.
	Conditions []metav1.Condition `json:"conditions,omitempty"`

	// LastSyncTime is the last time the instance was successfully validated
	// +optional.
	LastSyncTime *metav1.Time `json:"lastSyncTime,omitempty"`

	// FailureReason indicates the reason for any failure
	// +optional.
	FailureReason *string `json:"failureReason,omitempty"`

	// FailureMessage provides details about any failure
	// +optional.
	FailureMessage *string `json:"failureMessage,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:path=unifiinstances,scope=Namespaced
// +kubebuilder:printcolumn:name="Ready",type=boolean,JSONPath=`.status.ready`
// +kubebuilder:printcolumn:name="Host",type=string,JSONPath=`.spec.host`
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`

// UnifiInstance is the Schema for the unifiinstances API.
type UnifiInstance struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   UnifiInstanceSpec   `json:"spec,omitempty"`
	Status UnifiInstanceStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// UnifiInstanceList contains a list of UnifiInstance.
type UnifiInstanceList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []UnifiInstance `json:"items"`
}

func init() {
	SchemeBuilder.Register(&UnifiInstance{}, &UnifiInstanceList{})
}
