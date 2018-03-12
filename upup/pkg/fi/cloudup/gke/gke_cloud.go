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
	"net/http"
	"os"

	"github.com/golang/glog"
	"golang.org/x/net/context"
	"golang.org/x/oauth2/google"
	compute "google.golang.org/api/compute/v0.beta"
	container "google.golang.org/api/container/v1"
	"google.golang.org/api/iam/v1"
	oauth2 "google.golang.org/api/oauth2/v2"
	"google.golang.org/api/storage/v1"
	"k8s.io/kops/dnsprovider/pkg/dnsprovider"
	"k8s.io/kops/dnsprovider/pkg/dnsprovider/providers/google/clouddns"
	"k8s.io/kops/pkg/apis/kops"
	"k8s.io/kops/upup/pkg/fi"
	"k8s.io/kops/upup/pkg/fi/cloudup/gcp"
)

type GKECloud interface {
	fi.Cloud
	Compute() *compute.Service
	Container() *container.ProjectsZonesService
	IAM() *iam.Service

	Region() string
	Project() string
	WaitForOp(op *container.Operation) error
	Labels() map[string]string

	// FindClusterStatus gets the status of the cluster as it exists in GCE, inferred from volumes
	// FindClusterStatus(cluster *kops.Cluster) (*kops.ClusterStatus, error)

	Zones() ([]string, error)

	// ServiceAccount returns the email for the service account that the instances will run under
	ServiceAccount() (string, error)
}

type gkeCloudImplementation struct {
	compute *compute.Service
	storage *storage.Service
	iam     *iam.Service

	container *container.Service

	region  string
	project string

	// projectInfo caches the project info from the compute API
	projectInfo *compute.Project

	labels map[string]string
}

var _ fi.Cloud = &gkeCloudImplementation{}

func (c *gkeCloudImplementation) ProviderID() kops.CloudProviderID {
	return kops.CloudProviderGKE
}

var gkeCloudInstances = make(map[string]GKECloud)

func NewGKECloud(region string, project string, labels map[string]string) (GKECloud, error) {
	i := gkeCloudInstances[region+"::"+project]
	if i != nil {
		return i.(gkeCloudInternal).WithLabels(labels), nil
	}

	c := &gkeCloudImplementation{region: region, project: project}

	ctx := context.Background()

	if os.Getenv("GOOGLE_APPLICATION_CREDENTIALS") != "" {
		glog.Infof("Will load GOOGLE_APPLICATION_CREDENTIALS from %s", os.Getenv("GOOGLE_APPLICATION_CREDENTIALS"))
	}

	// TODO: should we create different clients with per-service scopes?
	client, err := google.DefaultClient(ctx, compute.CloudPlatformScope)
	if err != nil {
		return nil, fmt.Errorf("error building google API client: %v", err)
	}

	computeService, err := compute.New(client)
	if err != nil {
		return nil, fmt.Errorf("error building compute API client: %v", err)
	}
	c.compute = computeService

	storageService, err := storage.New(client)
	if err != nil {
		return nil, fmt.Errorf("error building storage API client: %v", err)
	}
	c.storage = storageService

	iamService, err := iam.New(client)
	if err != nil {
		return nil, fmt.Errorf("error building IAM API client: %v", err)
	}
	c.iam = iamService

	containerService, err := container.New(client)
	if err != nil {
		return nil, fmt.Errorf("error building Container API client: %v", err)
	}
	c.container = containerService

	gkeCloudInstances[region+"::"+project] = c

	{
		// Attempt to log the current GCE service account in user, for diagnostic purposes
		// At least until we get e2e running, we're doing this always
		tokenInfo, err := c.getTokenInfo(client)
		if err != nil {
			glog.Infof("unable to get token info: %v", err)
		} else {
			glog.V(2).Infof("running with GCE credentials: email=%s, scope=%s", tokenInfo.Email, tokenInfo.Scope)
		}
	}

	return c.WithLabels(labels), nil
}

// gkeCloudInternal is an interface for private functions for a gceCloudImplemention or mockGKECloud
type gkeCloudInternal interface {
	// WithLabels returns a copy of the GKECloud, bound to the specified labels
	WithLabels(labels map[string]string) GKECloud
}

// WithLabels returns a copy of the GKECloud, bound to the specified labels
func (c *gkeCloudImplementation) WithLabels(labels map[string]string) GKECloud {
	i := &gkeCloudImplementation{}
	*i = *c
	i.labels = labels
	return i
}

// Compute returns private struct element compute.
func (c *gkeCloudImplementation) Compute() *compute.Service {
	return c.compute
}

// Container returns private struct element compute.
func (c *gkeCloudImplementation) Container() *container.ProjectsZonesService {
	return container.NewProjectsZonesService(c.container)
}

// IAM returns the IAM client
func (c *gkeCloudImplementation) IAM() *iam.Service {
	return c.iam
}

// Region returns private struct element region.
func (c *gkeCloudImplementation) Region() string {
	return c.region
}

// Project returns private struct element project.
func (c *gkeCloudImplementation) Project() string {
	return c.project
}

// ServiceAccount returns the email address for the service account that the instances will run under.
func (c *gkeCloudImplementation) ServiceAccount() (string, error) {
	if c.projectInfo == nil {
		// Find the project info from the compute API, which includes the default service account
		glog.V(2).Infof("fetching project %q from compute API", c.project)
		p, err := c.compute.Projects.Get(c.project).Do()
		if err != nil {
			return "", fmt.Errorf("error fetching info for project %q: %v", c.project, err)
		}

		c.projectInfo = p
	}

	if c.projectInfo.DefaultServiceAccount == "" {
		return "", fmt.Errorf("compute project %q did not have DefaultServiceAccount", c.project)
	}

	return c.projectInfo.DefaultServiceAccount, nil
}

func (c *gkeCloudImplementation) DNS() (dnsprovider.Interface, error) {
	provider, err := clouddns.CreateInterface(c.project, nil)
	if err != nil {
		return nil, fmt.Errorf("error building (k8s) DNS provider: %v", err)
	}
	return provider, nil
}

func (c *gkeCloudImplementation) FindVPCInfo(id string) (*fi.VPCInfo, error) {
	glog.Warningf("FindVPCInfo not (yet) implemented on GCE")
	return nil, nil
}

func (c *gkeCloudImplementation) Labels() map[string]string {
	// Defensive copy
	tags := make(map[string]string)
	for k, v := range c.labels {
		tags[k] = v
	}
	return tags
}

// TODO refactor this out of resources
// this is needed for delete groups and other new methods

// Zones returns the zones in a region
func (c *gkeCloudImplementation) Zones() ([]string, error) {
	var zones []string
	// TODO: Only zones in api.Cluster object, if we have one?
	gceZones, err := c.Compute().Zones.List(c.Project()).Do()
	if err != nil {
		return nil, fmt.Errorf("error listing zones: %v", err)
	}
	for _, gceZone := range gceZones.Items {
		u, err := gcp.ParseGoogleCloudURL(gceZone.Region)
		if err != nil {
			return nil, err
		}
		if u.Name != c.Region() {
			continue
		}
		zones = append(zones, gceZone.Name)
	}
	if len(zones) == 0 {
		return nil, fmt.Errorf("unable to determine zones in region %q", c.Region())
	}

	glog.Infof("Scanning zones: %v", zones)
	return zones, nil
}

func (c *gkeCloudImplementation) WaitForOp(op *container.Operation) error {
	return gcp.WaitForContainerOp(c.Container(), op)
}

// logTokenInfo returns information about the active credential
func (c *gkeCloudImplementation) getTokenInfo(client *http.Client) (*oauth2.Tokeninfo, error) {
	tokenSource, err := google.DefaultTokenSource(context.TODO(), compute.CloudPlatformScope)
	if err != nil {
		return nil, fmt.Errorf("error building token source: %v", err)
	}

	token, err := tokenSource.Token()
	if err != nil {
		return nil, fmt.Errorf("error getting token: %v", err)
	}

	// Note: do not log token or any portion of it

	service, err := oauth2.New(client)
	if err != nil {
		return nil, fmt.Errorf("error creating oauth2 service client: %v", err)
	}

	tokenInfo, err := service.Tokeninfo().AccessToken(token.AccessToken).Do()
	if err != nil {
		return nil, fmt.Errorf("error fetching oauth2 token info: %v", err)
	}

	return tokenInfo, nil
}
