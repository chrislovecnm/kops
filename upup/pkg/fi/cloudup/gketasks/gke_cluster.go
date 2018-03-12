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

	"strings"

	container "google.golang.org/api/container/v1"
	"k8s.io/kops/upup/pkg/fi"
	"k8s.io/kops/upup/pkg/fi/cloudup/gcp"
	"k8s.io/kops/upup/pkg/fi/cloudup/gke"
	"k8s.io/kops/upup/pkg/fi/cloudup/terraform"
)

//go:generate fitask -type=GKECluster
type GKECluster struct {
	Name      *string
	Lifecycle *fi.Lifecycle
	Region    *string
	Zone      *string
	Version   *string
	Locations []string

	DefaultNodePoolName *string

	BootDiskImage  *string
	BootDiskSizeGB *int64
	BootDiskType   *string

	Scopes []string

	Metadata    map[string]*fi.ResourceHolder
	MachineType *string

	Tags []string

	InitialCount *int64
}

var _ fi.CompareWithID = &GKECluster{}

func (e *GKECluster) CompareWithID() *string {
	return e.Name
}

func (e *GKECluster) Find(c *fi.Context) (*GKECluster, error) {
	cloud := c.Cloud.(gke.GKECloud)
	clusterName := strings.ToLower(gcp.SafeClusterName(c.Cluster.ObjectMeta.Name))

	// TODO how do we get the zone here??
	resp, err := cloud.Container().Clusters.Get(cloud.Project(), "us-central1-a", clusterName).Do()
	if err != nil {
		if gcp.IsNotFound(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("error getting gke cluster: %v", err)
	}

	actual := &GKECluster{
		Name: &resp.Name,
	}
	return actual, nil
}

func (e *GKECluster) Run(c *fi.Context) error {
	return fi.DefaultDeltaRunMethod(e, c)
}

func (_ *GKECluster) CheckChanges(a, e, changes *GKECluster) error {
	if a != nil {
		if e.Name == nil {
			return fi.RequiredField("Name")
		}
	}
	return nil
}

func (_ *GKECluster) RenderGKE(t *gke.GKEAPITarget, a, e, changes *GKECluster) error {
	cloud := t.Cloud
	// TODO where a != nil
	if a == nil {

		clusterName := strings.ToLower(gcp.SafeClusterName(fi.StringValue(e.Name)))
		// first ig listed
		// TODO can we create a cluster w/o a nodepool?
		nodePools := []*container.NodePool{
			{
				InitialNodeCount: fi.Int64Value(e.InitialCount),
				Name:             gcp.SafeClusterName(fi.StringValue(e.DefaultNodePoolName)),
				Config: &container.NodeConfig{
					DiskSizeGb:  fi.Int64Value(e.BootDiskSizeGB),
					MachineType: fi.StringValue(e.MachineType),
					Tags:        e.Tags,
					Preemptible: false,
				},
			},
		}

		request := &container.CreateClusterRequest{
			Cluster: &container.Cluster{
				Name:                  clusterName,
				NodePools:             nodePools,
				InitialClusterVersion: fi.StringValue(e.Version),
				// TODO
				// Locations:
			},
		}
		// FIXME zone
		op, err := cloud.Container().Clusters.Create(cloud.Project(), "us-central1-a", request).Do()
		if err != nil {
			return fmt.Errorf("error creating gke cluster: %v", err)
		}

		if err := gcp.WaitForContainerOp(cloud.Container(), op); err != nil {
			return fmt.Errorf("error setting metadata on instance: %v", err)
		}
	}

	return nil
}

type terraformGKECluster struct {
	Name   *string `json:"name"`
	Region *string `json:"region"`
}

func (_ *GKECluster) RenderGKECluster(t *terraform.TerraformTarget, a, e, changes *GKECluster) error {
	tf := &terraformGKECluster{
		Name:   e.Name,
		Region: e.Region,
	}
	return t.RenderResource("google_compute_gke_cluster", *e.Name, tf)
}

func (i *GKECluster) TerraformName() *terraform.Literal {
	return terraform.LiteralProperty("google_compute_gke_cluster", *i.Name, "name")
}
