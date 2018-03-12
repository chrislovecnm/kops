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

package gke

import (
	"fmt"

	"k8s.io/api/core/v1"
	"k8s.io/kops/pkg/apis/kops"
	"k8s.io/kops/pkg/cloudinstances"
)

// DeleteGroup deletes a cloud of instances controlled by an Instance Group Manager
func (c *gkeCloudImplementation) DeleteGroup(g *cloudinstances.CloudInstanceGroup) error {
	return deleteCloudInstanceGroup(c, g)
}

// deleteCloudInstanceGroup deletes the InstanceGroupManager and current InstanceTemplate
func deleteCloudInstanceGroup(c GKECloud, g *cloudinstances.CloudInstanceGroup) error {
	return fmt.Errorf("not implemented yet")
}

// DeleteInstance deletes a GCE instance
func (c *gkeCloudImplementation) DeleteInstance(i *cloudinstances.CloudInstanceGroupMember) error {
	return recreateCloudInstanceGroupMember(c, i)
}

// recreateCloudInstanceGroupMember recreates the specified instances, managed by an InstanceGroupManager
func recreateCloudInstanceGroupMember(c GKECloud, i *cloudinstances.CloudInstanceGroupMember) error {
	return fmt.Errorf("not implemented yet")
}

// GetCloudGroups returns a map of CloudGroup that backs a list of instance groups
func (c *gkeCloudImplementation) GetCloudGroups(cluster *kops.Cluster, instancegroups []*kops.InstanceGroup, warnUnmatched bool, nodes []v1.Node) (map[string]*cloudinstances.CloudInstanceGroup, error) {
	return getCloudGroups(c, cluster, instancegroups, warnUnmatched, nodes)
}

func getCloudGroups(c GKECloud, cluster *kops.Cluster, instancegroups []*kops.InstanceGroup, warnUnmatched bool, nodes []v1.Node) (map[string]*cloudinstances.CloudInstanceGroup, error) {
	return nil, fmt.Errorf("not implemented yet")
}
