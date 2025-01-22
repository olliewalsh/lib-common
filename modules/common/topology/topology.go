/*
Copyright 2025 Red Hat

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

package topology

import (
	"context"
	"fmt"

	topologyv1 "github.com/openstack-k8s-operators/infra-operator/apis/topology/v1beta1"
	"github.com/openstack-k8s-operators/lib-common/modules/common/helper"
	"github.com/openstack-k8s-operators/lib-common/modules/common/util"
	k8s_errors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

// Standard MatchLabelKeys labels for Deployments/Statefulsets
const (
	StatefulsetMatchLabelKey = "controller-revision-hash"
	DeploymentMatchLabelKey  = "pod-template-hash"
)

// EnsureTopologyRef - retrieve the Topology CR referenced and add a finalizer
func EnsureTopologyRef(
	ctx context.Context,
	h *helper.Helper,
	topologyRef *Topology,
	finalizer string,
	defaultLabelSelector metav1.LabelSelector,
	defaultMatchLabelKeys []string,

) (*topologyv1.Topology, string, error) {

	var err error
	var hash string

	// no Topology is passed at all or it is missing some data
	if topologyRef == nil || (topologyRef.Name == "" || topologyRef.Namespace == "") {
		return nil, "", fmt.Errorf("No valid TopologyRef input passed")
	}

	topology, _, err := topologyv1.GetTopologyByName(
		ctx,
		h,
		topologyRef.Name,
		topologyRef.Namespace,
	)
	if err != nil {
		return topology, hash, err
	}

	// Add finalizer (if not present) to the resource consumed by the Service
	if controllerutil.AddFinalizer(topology, fmt.Sprintf("%s-%s", h.GetFinalizer(), finalizer)) {
		if err := h.GetClient().Update(ctx, topology); err != nil {
			return topology, hash, err
		}
	}

	// Set default LabelSelector and MatchLabelKeys if applicable
	topology = topology.DeepCopy()

	topologyConstraints := topology.Spec.TopologySpreadConstraints
	if topologyConstraints != nil {
		for i := 0; i < len(*topologyConstraints); i++ {
			current := &(*topologyConstraints)[i]
			if current.LabelSelector == nil {
				current.LabelSelector = &defaultLabelSelector
			}
			if len(current.MatchLabelKeys) == 0 {
				current.MatchLabelKeys = defaultMatchLabelKeys
			}
		}
	}
	affinity := topology.Spec.Affinity
	if affinity != nil {
		podAffinity := affinity.PodAffinity
		if podAffinity != nil {
			for i := range podAffinity.RequiredDuringSchedulingIgnoredDuringExecution {
				current := &podAffinity.RequiredDuringSchedulingIgnoredDuringExecution[i]
				if current.LabelSelector == nil {
					current.LabelSelector = &defaultLabelSelector
				}
				// Coming in kubernetes 1.31 (4.18?) https://kubernetes.io/blog/2024/08/16/matchlabelkeys-podaffinity/
				// Should we set it anyway ... not sure
				if len(current.MatchLabelKeys) == 0 {
					current.MatchLabelKeys = defaultMatchLabelKeys
				}
			}
			for i := range podAffinity.PreferredDuringSchedulingIgnoredDuringExecution {
				current := &podAffinity.PreferredDuringSchedulingIgnoredDuringExecution[i].PodAffinityTerm
				if current.LabelSelector == nil {
					current.LabelSelector = &defaultLabelSelector
				}
				// Coming in kubernetes 1.31 (4.18?) https://kubernetes.io/blog/2024/08/16/matchlabelkeys-podaffinity/
				// Should we set it anyway ... not sure
				if len(current.MatchLabelKeys) == 0 {
					current.MatchLabelKeys = defaultMatchLabelKeys
				}
			}
		}
		podAntiAffinity := affinity.PodAntiAffinity
		if podAntiAffinity != nil {
			for i := range podAntiAffinity.RequiredDuringSchedulingIgnoredDuringExecution {
				current := &podAntiAffinity.RequiredDuringSchedulingIgnoredDuringExecution[i]
				if current.LabelSelector == nil {
					current.LabelSelector = &defaultLabelSelector
				}
				// Coming in kubernetes 1.31 (4.18?) https://kubernetes.io/blog/2024/08/16/matchlabelkeys-podaffinity/
				// Should we set it anyway ... not sure
				if len(current.MatchLabelKeys) == 0 {
					current.MatchLabelKeys = defaultMatchLabelKeys
				}
			}
			for i := range podAntiAffinity.PreferredDuringSchedulingIgnoredDuringExecution {
				current := &podAntiAffinity.PreferredDuringSchedulingIgnoredDuringExecution[i].PodAffinityTerm
				if current.LabelSelector == nil {
					current.LabelSelector = &defaultLabelSelector
				}
				// Coming in kubernetes 1.31 (4.18?) https://kubernetes.io/blog/2024/08/16/matchlabelkeys-podaffinity/
				// Should we set it anyway ... not sure
				if len(current.MatchLabelKeys) == 0 {
					current.MatchLabelKeys = defaultMatchLabelKeys
				}
			}
		}
	}

	hash, err = util.ObjectHash(topology.Spec)
	if err != nil {
		return topology, hash, err
	}

	return topology, hash, nil
}

// EnsureDeletedTopologyRef - remove the finalizer (passed as input) from the
// referenced topology CR
func EnsureDeletedTopologyRef(
	ctx context.Context,
	h *helper.Helper,
	c client.Client,
	topologyRef *Topology,
	finalizer string,
) (ctrl.Result, error) {

	// no Topology is passed at all or some data is missing
	if topologyRef == nil || (topologyRef.Name == "" || topologyRef.Namespace == "") {
		return ctrl.Result{}, nil
	}

	// Remove the finalizer from the Topology CR
	topology, _, err := topologyv1.GetTopologyByName(
		ctx,
		h,
		topologyRef.Name,
		topologyRef.Namespace,
	)

	if err != nil && !k8s_errors.IsNotFound(err) {
		return ctrl.Result{}, err
	}
	if !k8s_errors.IsNotFound(err) {
		if controllerutil.RemoveFinalizer(topology, fmt.Sprintf("%s-%s", h.GetFinalizer(), finalizer)) {
			err = c.Update(ctx, topology)
			if err != nil && !k8s_errors.IsNotFound(err) {
				return ctrl.Result{}, err
			}
			util.LogForObject(h, "Removed finalizer from Topology", topology)
		}
	}
	return ctrl.Result{}, nil
}
