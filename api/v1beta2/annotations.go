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

// MACAddressAnnotation, set on an IPAddressClaim, names the MAC address of the
// device the claim is for. The UniFi fixed-IP reservation is then made on that
// device's own client record rather than on a synthetic MAC derived from the
// claim name, so the reservation actually reaches the machine over DHCP.
const MACAddressAnnotation = "unifi.ipam.cluster.x-k8s.io/mac-address"
