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
type NodePoolModelBuilder struct {
	*model.KopsModelContext
	Lifecycle *fi.Lifecycle
}

var _ fi.ModelBuilder = &NodePoolModelBuilder{}

func (b *NodePoolModelBuilder) Build(c *fi.ModelBuilderContext) (err error) {
	for _, ig := range b.InstanceGroups {
		volumeSize := fi.Int32Value(ig.Spec.RootVolumeSize)
		if volumeSize == 0 {
			volumeSize, err = defaults.DefaultInstanceGroupVolumeSize(ig.Spec.Role)
			if err != nil {
				return err
			}
		}
		t := &gketasks.NodePool{
			Name:           fi.String(ig.ObjectMeta.Name),
			BootDiskSizeGB: fi.Int64(int64(volumeSize)),
			MachineType:    fi.String(ig.Spec.MachineType),
			InitialCount:   fi.Int64(int64(fi.Int32Value(ig.Spec.MinSize))),
			BootDiskImage:  fi.String(ig.Spec.Image),

			Cluster: &gketasks.GKECluster{
				Name: fi.String(b.Cluster.ObjectMeta.Name),
			},
		}
		if err = c.EnsureTask(t); err != nil {
			return err
		}
	}
	return nil
}
