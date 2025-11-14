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

package v1beta2

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// UnifiIPPoolSpec defines the desired state of UnifiIPPool.
type UnifiIPPoolSpec struct {
	// InstanceRef is a reference to the UnifiInstance to use
	// +kubebuilder:validation:Required
	InstanceRef corev1.ObjectReference `json:"instanceRef"`

	// NetworkID is the Unifi network ID to allocate from
	// This is the _id field from the Unifi network configuration
	// +kubebuilder:validation:Required
	NetworkID string `json:"networkId"`

	// Subnets is the list of subnets to allocate from
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinItems=1
	Subnets []SubnetSpec `json:"subnets"`
}

// SubnetSpec defines a subnet configuration.
type SubnetSpec struct {
	// CIDR is the subnet CIDR block
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:Pattern=`^([0-9]{1,3}\.){3}[0-9]{1,3}/[0-9]{1,2}$`
	CIDR string `json:"cidr"`

	// Gateway is the gateway IP address for this subnet
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:Pattern=`^([0-9]{1,3}\.){3}[0-9]{1,3}$`
	Gateway string `json:"gateway"`

	// Prefix is the network prefix length
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:validation:Maximum=32
	Prefix *int32 `json:"prefix"`

	// ExcludeRanges is a list of IP ranges to exclude from allocation
	// Format: "start-end" (e.g., "192.168.1.1-192.168.1.10")
	// +optional.
	ExcludeRanges []string `json:"excludeRanges,omitempty"`

	// DNS is the list of DNS servers for this subnet
	// +optional.
	DNS []string `json:"dns,omitempty"`
}

// UnifiIPPoolStatus defines the observed state of UnifiIPPool.
type UnifiIPPoolStatus struct {
	// Addresses provides summary statistics about address allocation
	// +optional.
	Addresses *IPAddressStatusSummary `json:"addresses,omitempty"`

	// Conditions defines current state of the UnifiIPPool using metav1.Conditions
	// +optional.
	Conditions []metav1.Condition `json:"conditions,omitempty"`

	// LastSyncTime is the last time the pool was successfully synced
	// +optional.
	LastSyncTime *metav1.Time `json:"lastSyncTime,omitempty"`
}

// IPAddressStatusSummary provides summary statistics about IP address allocation.
type IPAddressStatusSummary struct {
	// Total is the total number of addresses in the pool.
	// +optional.
	Total *int32 `json:"total,omitempty"`

	// Used is the number of addresses currently allocated.
	// +optional.
	Used *int32 `json:"used,omitempty"`

	// Free is the number of addresses available for allocation.
	// +optional.
	Free *int32 `json:"free,omitempty"`

	// OutOfRange is the number of addresses allocated outside the pool's range.
	// +optional.
	OutOfRange *int32 `json:"outOfRange,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:path=unifiippools,scope=Namespaced,categories=cluster-api
// +kubebuilder:storageversion
// +kubebuilder:printcolumn:name="NetworkID",type="string",JSONPath=".spec.networkId",description="Unifi network ID"
// +kubebuilder:printcolumn:name="Total",type="integer",JSONPath=".status.addresses.total",description="Total addresses in pool"
// +kubebuilder:printcolumn:name="Used",type="integer",JSONPath=".status.addresses.used",description="Number of allocated addresses"
// +kubebuilder:printcolumn:name="Free",type="integer",JSONPath=".status.addresses.free",description="Number of free addresses"
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp",description="Time duration since creation"

// UnifiIPPool is the Schema for the unifiippools API.
type UnifiIPPool struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   UnifiIPPoolSpec   `json:"spec,omitempty"`
	Status UnifiIPPoolStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// UnifiIPPoolList contains a list of UnifiIPPool.
type UnifiIPPoolList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []UnifiIPPool `json:"items"`
}

func init() {
	SchemeBuilder.Register(&UnifiIPPool{}, &UnifiIPPoolList{})
}
