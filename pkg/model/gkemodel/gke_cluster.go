/*
Copyright 2016 The Kubernetes Authors.

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

package gkemodel

import (
	"k8s.io/kops/pkg/model"
	"k8s.io/kops/pkg/model/defaults"
	"k8s.io/kops/upup/pkg/fi"
	"k8s.io/kops/upup/pkg/fi/cloudup/gketasks"
)

// AutoscalingGroupModelBuilder configures AutoscalingGroup objects
type GKEClusterModelBuilder struct {
	*model.KopsModelContext
	Lifecycle *fi.Lifecycle
}

var _ fi.ModelBuilder = &GKEClusterModelBuilder{}

func (b *GKEClusterModelBuilder) Build(c *fi.ModelBuilderContext) error {

	// if we can create a cluster without node pools then we do not have to do this
	ig := b.InstanceGroups[0]

	volumeSize := fi.Int32Value(ig.Spec.RootVolumeSize)
	var err error
	if volumeSize == 0 {
		volumeSize, err = defaults.DefaultInstanceGroupVolumeSize(ig.Spec.Role)
		if err != nil {
			return err
		}
	}

	zones, err := b.FindZonesForInstanceGroup(ig)

	if err != nil {
		return err
	}

	t := &gketasks.GKECluster{
		Name:                fi.String(b.Cluster.Name),
		Region:              fi.String(b.Region),
		Lifecycle:           b.Lifecycle,
		DefaultNodePoolName: fi.String(ig.ObjectMeta.Name),
		BootDiskSizeGB:      fi.Int64(int64(volumeSize)),
		MachineType:         fi.String(ig.Spec.MachineType),
		InitialCount:        fi.Int64(int64(fi.Int32Value(ig.Spec.MinSize))),
		BootDiskImage:       fi.String(ig.Spec.Image),
		Locations:           zones,
	}

	c.AddTask(t)

	return nil
}
