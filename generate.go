// Copyright 2024.
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

//go:build generate

package main

// Generate deep copy implementations
//go:generate go tool controller-gen object:headerFile=hack/boilerplate.go.txt paths=./...

// Generate CRDs, RBAC, and webhook manifests
//go:generate go tool controller-gen rbac:roleName=manager-role crd webhook paths=./... output:crd:artifacts:config=config/crd/bases

// Generate webhook configurations
//go:generate go tool controller-gen webhook paths=./... output:webhook:artifacts:config=config/webhook
