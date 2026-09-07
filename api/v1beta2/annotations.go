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

// Annotations an IPAddressClaim may carry to describe the machine it is for.
//
// The ipam.cluster.x-k8s.io keys are a provider-neutral convention: an
// infrastructure provider can set them without knowing which IPAM provider
// serves the pool. The unifi.ipam.cluster.x-k8s.io key is this provider's own
// and is kept for claims that already use it.
const (
	// IPAMMACAddressAnnotation names the MAC address of the device the claim
	// is for. The UniFi fixed-IP reservation is made on that device's own
	// client record rather than on a synthetic MAC derived from the claim
	// name, so the reservation actually reaches the machine over DHCP.
	IPAMMACAddressAnnotation = "ipam.cluster.x-k8s.io/mac-address"

	// IPAMHostnameAnnotation names the machine. When set, the UniFi client
	// record's display name becomes the hostname and a local DNS record
	// (<hostname>.<network domain>) is enabled for the reserved address.
	IPAMHostnameAnnotation = "ipam.cluster.x-k8s.io/hostname"

	// MACAddressAnnotation is this provider's own spelling of
	// IPAMMACAddressAnnotation, recognized when the neutral key is absent.
	MACAddressAnnotation = "unifi.ipam.cluster.x-k8s.io/mac-address"
)
