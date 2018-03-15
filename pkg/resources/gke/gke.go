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

package gke

import (
	"fmt"
	"github.com/golang/glog"
	"google.golang.org/api/container/v1"
	"k8s.io/kops/pkg/resources"
	"k8s.io/kops/upup/pkg/fi"
	"k8s.io/kops/upup/pkg/fi/cloudup/gke"
)

type gkeListFn func() ([]*resources.Resource, error)

const (
	typeGKECluster = "GKECluster"
)

func ListResourcesGKE(gkeCloud gke.GKECloud, clusterName string, region string) (map[string]*resources.Resource, error) {
	if region == "" {
		region = gkeCloud.Region()
	}

	resources := make(map[string]*resources.Resource)

	d := &clusterDiscoveryGKE{
		cloud:       gkeCloud,
		gkeCloud:    gkeCloud,
		clusterName: clusterName,
		// FIXME get this somehow
		region: "us-central1-a",
	}

	listFunctions := []gkeListFn{
		d.listGKECluster,
	}
	for _, fn := range listFunctions {
		resourceTrackers, err := fn()
		if err != nil {
			return nil, err
		}
		for _, t := range resourceTrackers {
			resources[t.Type+":"+t.ID] = t
		}
	}

	for k, t := range resources {
		if t.Done {
			delete(resources, k)
		}
	}
	return resources, nil
}

type clusterDiscoveryGKE struct {
	cloud       fi.Cloud
	gkeCloud    gke.GKECloud
	clusterName string

	cluster *container.Cluster
	zones   []string
	region  string
}

func (d *clusterDiscoveryGKE) findGKECluster() (*container.Cluster, error) {

	gkeCluster, err := gke.FindGKECluster(d.gkeCloud, d.clusterName, d.region)
	if err != nil {
		return nil, err
	}

	if gkeCluster == nil {
		return nil, fmt.Errorf("unable to find cluster %q", d.clusterName)
	}

	d.cluster = gkeCluster
	return d.cluster, nil
}

func (d *clusterDiscoveryGKE) listGKECluster() ([]*resources.Resource, error) {
	var resourceTrackers []*resources.Resource

	t, err := d.findGKECluster()
	if err != nil {
		return nil, err
	}
	resourceTracker := &resources.Resource{
		Name: t.Name,
		ID:   t.Name,
		Type: typeGKECluster,
		Deleter: func(cloud fi.Cloud, r *resources.Resource) error {
			// FIXME we need zone again
			return gke.DeleteGKECluster(d.gkeCloud, t.Name, d.region)
		},
		Obj: t,
	}

	glog.V(4).Infof("Found resource: %s", t.SelfLink)
	resourceTrackers = append(resourceTrackers, resourceTracker)

	return resourceTrackers, nil
}
