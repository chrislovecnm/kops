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

package gcp

import (
	"fmt"
	"os/exec"
	"strings"

	"bytes"
	"os"

	"github.com/golang/glog"
	"google.golang.org/api/googleapi"
)

func IsNotFound(err error) bool {
	apiErr, ok := err.(*googleapi.Error)
	if !ok {
		return false
	}

	// We could also check for Errors[].Resource == "notFound"
	//glog.Info("apiErr: %v", apiErr)

	return apiErr.Code == 404
}

func IsNotReady(err error) bool {
	apiErr, ok := err.(*googleapi.Error)
	if !ok {
		return false
	}
	for _, e := range apiErr.Errors {
		if e.Reason == "resourceNotReady" {
			return true
		}
	}
	return false
}

func SafeClusterName(clusterName string) string {
	// GCE does not support . in tags / names
	safeClusterName := strings.Replace(clusterName, ".", "-", -1)
	return safeClusterName
}

// SafeObjectName returns the object name and cluster name escaped for GCE
func SafeObjectName(name string, clusterName string) string {
	gceName := name + "-" + clusterName

	// TODO: If the cluster name > some max size (32?) we should curtail it
	return SafeClusterName(gceName)
}

// LastComponent returns the last component of a URL, i.e. anything after the last slash
// If there is no slash, returns the whole string
func LastComponent(s string) string {
	lastSlash := strings.LastIndex(s, "/")
	if lastSlash != -1 {
		s = s[lastSlash+1:]
	}
	return s
}

// ZoneToRegion maps a GCE zone name to a GCE region name, returning an error if it cannot be mapped
func ZoneToRegion(zone string) (string, error) {
	tokens := strings.Split(zone, "-")
	if len(tokens) <= 2 {
		return "", fmt.Errorf("invalid GCE Zone: %v", zone)
	}
	region := tokens[0] + "-" + tokens[1]
	return region, nil
}

// DefaultProject returns the current project configured in the gcloud SDK, ("", nil) if no project was set
func DefaultProject() (string, error) {
	// The default project isn't usually defined by the google cloud APIs,
	// for example the Application Default Credential won't have ProjectID set.
	// If we're running on a GCP instance, we can get it from the metadata service,
	// but the normal kops CLI usage is running locally with gcloud configuration with a project,
	// so we use that value.
	cmd := exec.Command("gcloud", "config", "get-value", "project")

	env := os.Environ()
	cmd.Env = env
	var stdout bytes.Buffer
	cmd.Stdout = &stdout

	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	human := strings.Join(cmd.Args, " ")
	glog.V(2).Infof("Running command: %s", human)
	err := cmd.Run()
	if err != nil {
		glog.Infof("error running %s", human)
		glog.Info(stdout.String())
		glog.Info(stderr.String())
		return "", fmt.Errorf("error running %s: %v", human, err)
	}

	projectID := strings.TrimSpace(stdout.String())
	return projectID, err
}
