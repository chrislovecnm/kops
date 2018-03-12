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

package gketasks

import (
	"fmt"

	"google.golang.org/api/container/v1"
	"k8s.io/kops/upup/pkg/fi"
	"k8s.io/kops/upup/pkg/fi/cloudup/gcp"
	"k8s.io/kops/upup/pkg/fi/cloudup/gke"
	"k8s.io/kops/upup/pkg/fi/cloudup/terraform"
)

//go:generate fitask -type=NodePool
type NodePool struct {
	Name      *string
	Lifecycle *fi.Lifecycle

	BootDiskImage  *string
	BootDiskSizeGB *int64
	BootDiskType   *string

	Scopes []string

	Metadata    map[string]*fi.ResourceHolder
	MachineType *string

	Tags []string

	InitialCount *int64

	Cluster *GKECluster
}

var _ fi.CompareWithID = &NodePool{}

func (e *NodePool) CompareWithID() *string {
	return e.Name
}

func (e *NodePool) Find(c *fi.Context) (*NodePool, error) {

	cloud := c.Cloud.(gke.GKECloud)
	clusterName := gcp.SafeClusterName(fi.StringValue(e.Cluster.Name))
	// TODO how do I get the zone here??
	resp, err := cloud.Container().Clusters.NodePools.Get(cloud.Project(), "us-central1-a", clusterName, fi.StringValue(e.Name)).Do()
	// TODO how do we filter here by name?
	if err != nil {
		if gcp.IsNotFound(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("error getting gke cluster: %v", err)
	}

	actual := &NodePool{
		Name:           e.Name,
		Cluster:        &GKECluster{Name: fi.String(clusterName)},
		InitialCount:   fi.Int64(resp.InitialNodeCount),
		Tags:           resp.Config.Tags,
		MachineType:    fi.String(resp.Config.MachineType),
		BootDiskSizeGB: fi.Int64(resp.Config.DiskSizeGb),
		// TODO fill out mre
	}

	return actual, nil
}

func (e *NodePool) Run(c *fi.Context) error {
	return fi.DefaultDeltaRunMethod(e, c)
}

func (_ *NodePool) CheckChanges(a, e, changes *NodePool) error {
	if a != nil {
		if e.Name == nil {
			return fi.RequiredField("Name")
		}
	}
	return nil
}

func (_ *NodePool) RenderGKE(t *gke.GKEAPITarget, a, e, changes *NodePool) error {
	cloud := t.Cloud
	// TODO should we check that the cluster exists?? Or should we have a link to the task??
	if a == nil {
		clusterName := gcp.SafeClusterName(fi.StringValue(e.Cluster.Name))
		request := &container.CreateNodePoolRequest{
			NodePool: &container.NodePool{
				InitialNodeCount: fi.Int64Value(e.InitialCount),
				Name:             fi.StringValue(e.Name),
				Config: &container.NodeConfig{
					DiskSizeGb:  fi.Int64Value(e.BootDiskSizeGB),
					MachineType: fi.StringValue(e.MachineType),
					Tags:        e.Tags,
					// TODO add field
					Preemptible: true,
				},
			},
		}

		// FIXME zone needs to be correct
		op, err := cloud.Container().Clusters.NodePools.Create(cloud.Project(), "us-central1-a", clusterName, request).Do()
		if err != nil {
			return fmt.Errorf("error creating nodepool: %v", err)
		}

		if err := gcp.WaitForContainerOp(cloud.Container(), op); err != nil {
			return fmt.Errorf("error running nodepool operation: %v", err)
		}
	}

	return nil
}

type terraformNodeGroup struct {
	Name *string `json:"name"`
}

func (_ *NodePool) RenderNodeGroup(t *terraform.TerraformTarget, a, e, changes *NodePool) error {
	tf := &terraformNodeGroup{
		Name: e.Name,
	}
	return t.RenderResource("google_compute_subnetwork", *e.Name, tf)
}

func (i *NodePool) TerraformName() *terraform.Literal {
	return terraform.LiteralProperty("google_compute_nodeupgroup", *i.Name, "name")
}
