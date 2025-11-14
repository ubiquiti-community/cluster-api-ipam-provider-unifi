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

package predicates

import (
	"sigs.k8s.io/cluster-api/util/annotations"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	ipamv1alpha1 "github.com/ubiquiti-community/cluster-api-ipam-provider-unifi/api/v1alpha1"
)

// ResourceTransitionedToUnpaused returns a predicate that triggers on resources
// transitioning from paused to unpaused.
func ResourceTransitionedToUnpaused() predicate.Predicate {
	return predicate.Funcs{
		UpdateFunc: func(e event.UpdateEvent) bool {
			return annotations.HasPaused(e.ObjectOld) && !annotations.HasPaused(e.ObjectNew)
		},
		CreateFunc: func(e event.CreateEvent) bool {
			return !annotations.HasPaused(e.Object)
		},
	}
}

// PoolNoLongerEmpty returns a predicate that triggers when a pool transitions
// from having no free addresses to having free addresses.
func PoolNoLongerEmpty() predicate.Predicate {
	return predicate.Funcs{
		UpdateFunc: func(e event.UpdateEvent) bool {
			oldPool, oldOK := e.ObjectOld.(*ipamv1alpha1.UnifiIPPool)
			newPool, newOK := e.ObjectNew.(*ipamv1alpha1.UnifiIPPool)

			if !oldOK || !newOK {
				return false
			}

			if oldPool.Status.Addresses == nil || newPool.Status.Addresses == nil {
				return false
			}

			// Trigger if old had 0 free and new has > 0 free
			return oldPool.Status.Addresses.Free == 0 && newPool.Status.Addresses.Free > 0
		},
	}
}

// ResourceNotPaused returns a predicate that filters out paused resources.
func ResourceNotPaused() predicate.Predicate {
	return predicate.Funcs{
		CreateFunc: func(e event.CreateEvent) bool {
			return !annotations.HasPaused(e.Object)
		},
		UpdateFunc: func(e event.UpdateEvent) bool {
			return !annotations.HasPaused(e.ObjectNew)
		},
		GenericFunc: func(e event.GenericEvent) bool {
			return !annotations.HasPaused(e.Object)
		},
	}
}
