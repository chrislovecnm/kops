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

// Extracted from the k8s GCE cloud provider
// The file contains functions that deal with waiting for GCE operations to complete

import (
	"fmt"
	"time"

	"net/url"
	"regexp"

	"github.com/golang/glog"
	compute "google.golang.org/api/compute/v0.beta"
	container "google.golang.org/api/container/v1"
	"google.golang.org/api/googleapi"
	"k8s.io/apimachinery/pkg/util/wait"
)

const (
	operationPollInterval        = 3 * time.Second
	operationPollTimeoutDuration = 30 * time.Minute
)

func WaitForOp(client *compute.Service, op *compute.Operation) error {
	u, err := ParseGoogleCloudURL(op.SelfLink)
	if err != nil {
		return fmt.Errorf("error parsing operation URL %q: %v", op.SelfLink, err)
	}

	if u.Zone != "" {
		return waitForZoneOp(client, op)
	}

	if u.Region != "" {
		return waitForRegionOp(client, op)
	}

	return waitForGlobalOp(client, op)
}

func WaitForContainerOp(client *container.ProjectsZonesService, op *container.Operation) error {
	return waitForContainerZoneOp(client, op)
}

var re = regexp.MustCompile(`v1/projects/(\d+)/zones/`)

func getProject(projectURL string) (string, error) {
	u, err := url.Parse(projectURL)
	if err != nil {
		return "", err
	}

	a := re.FindAllStringSubmatch(u.Path, 1)
	if len(a) != 1 {
		return "", fmt.Errorf("wrong length of match from url: %s", projectURL)
	}

	if len(a[0]) != 2 {
		return "", fmt.Errorf("wrong length of submatch from url: %s", projectURL)
	}
	return a[0][1], nil
}

func waitForContainerZoneOp(client *container.ProjectsZonesService, op *container.Operation) error {
	project, err := getProject(op.SelfLink)
	if err != nil {
		return err
	}

	return waitForContainerOp(op, func(operationName string) (*container.Operation, error) {
		return client.Operations.Get(project, op.Zone, op.Name).Do()
	})
}

func waitForZoneOp(client *compute.Service, op *compute.Operation) error {
	u, err := ParseGoogleCloudURL(op.SelfLink)
	if err != nil {
		return fmt.Errorf("error parsing operation URL %q: %v", op.SelfLink, err)
	}

	return waitForOp(op, func(operationName string) (*compute.Operation, error) {
		return client.ZoneOperations.Get(u.Project, u.Zone, operationName).Do()
	})
}

func waitForRegionOp(client *compute.Service, op *compute.Operation) error {
	u, err := ParseGoogleCloudURL(op.SelfLink)
	if err != nil {
		return fmt.Errorf("error parsing operation URL %q: %v", op.SelfLink, err)
	}

	return waitForOp(op, func(operationName string) (*compute.Operation, error) {
		return client.RegionOperations.Get(u.Project, u.Region, operationName).Do()
	})
}

func waitForGlobalOp(client *compute.Service, op *compute.Operation) error {
	u, err := ParseGoogleCloudURL(op.SelfLink)
	if err != nil {
		return fmt.Errorf("error parsing operation URL %q: %v", op.SelfLink, err)
	}

	return waitForOp(op, func(operationName string) (*compute.Operation, error) {
		return client.GlobalOperations.Get(u.Project, operationName).Do()
	})
}

func opContainerIsDone(op *container.Operation) bool {
	return op != nil && op.Status == "DONE"
}
func opIsDone(op *compute.Operation) bool {
	return op != nil && op.Status == "DONE"
}

// TODO reface waitForContainerOp to accept either an interface or functions
// we have dupicate code

func waitForContainerOp(op *container.Operation, getOperation func(operationName string) (*container.Operation, error)) error {
	if op == nil {
		return fmt.Errorf("operation must not be nil")
	}

	if opContainerIsDone(op) {
		return getErrorFromContainerOp(op)
	}

	opStart := time.Now()
	opName := op.Name
	return wait.Poll(operationPollInterval, operationPollTimeoutDuration, func() (bool, error) {
		start := time.Now()
		//gce.operationPollRateLimiter.Accept()
		duration := time.Now().Sub(start)
		if duration > 5*time.Second {
			glog.Infof("pollOperation: throttled %v for %v", duration, opName)
		}
		pollOp, err := getOperation(opName)
		if err != nil {
			glog.Warningf("GCE poll operation %s failed: pollOp: [%v] err: [%v] getErrorFromOp: [%v]", opName, pollOp, err, getErrorFromContainerOp(pollOp))
		}
		done := opContainerIsDone(pollOp)
		if done {
			duration := time.Now().Sub(opStart)
			if duration > 1*time.Minute {
				// Log the JSON. It's cleaner than the %v structure.
				enc, err := pollOp.MarshalJSON()
				if err != nil {
					glog.Warningf("waitForOperation: long operation (%v): %v (failed to encode to JSON: %v)", duration, pollOp, err)
				} else {
					glog.Infof("waitForOperation: long operation (%v): %v", duration, string(enc))
				}
			}
		}
		return done, getErrorFromContainerOp(pollOp)
	})
}

func getErrorFromContainerOp(op *container.Operation) error {

	// FIXME how do we know if this has failed?  We do not have an error object here

	if op != nil && op.HTTPStatusCode != 200 {
		err := &googleapi.Error{
			Code:   op.HTTPStatusCode,
			Header: op.Header,
			Body:   op.StatusMessage,
		}
		glog.Errorf("GKE  container operation failed: %v", err)
		return err
	}

	return nil
}

func waitForOp(op *compute.Operation, getOperation func(operationName string) (*compute.Operation, error)) error {
	if op == nil {
		return fmt.Errorf("operation must not be nil")
	}

	if opIsDone(op) {
		return getErrorFromOp(op)
	}

	opStart := time.Now()
	opName := op.Name
	return wait.Poll(operationPollInterval, operationPollTimeoutDuration, func() (bool, error) {
		start := time.Now()
		//gce.operationPollRateLimiter.Accept()
		duration := time.Now().Sub(start)
		if duration > 5*time.Second {
			glog.Infof("pollOperation: throttled %v for %v", duration, opName)
		}
		pollOp, err := getOperation(opName)
		if err != nil {
			glog.Warningf("GCE poll operation %s failed: pollOp: [%v] err: [%v] getErrorFromOp: [%v]", opName, pollOp, err, getErrorFromOp(pollOp))
		}
		done := opIsDone(pollOp)
		if done {
			duration := time.Now().Sub(opStart)
			if duration > 1*time.Minute {
				// Log the JSON. It's cleaner than the %v structure.
				enc, err := pollOp.MarshalJSON()
				if err != nil {
					glog.Warningf("waitForOperation: long operation (%v): %v (failed to encode to JSON: %v)", duration, pollOp, err)
				} else {
					glog.Infof("waitForOperation: long operation (%v): %v", duration, string(enc))
				}
			}
		}
		return done, getErrorFromOp(pollOp)
	})
}

func getErrorFromOp(op *compute.Operation) error {
	if op != nil && op.Error != nil && len(op.Error.Errors) > 0 {
		err := &googleapi.Error{
			Code:    int(op.HttpErrorStatusCode),
			Message: op.Error.Errors[0].Message,
		}
		glog.Errorf("GCE operation failed: %v", err)
		return err
	}

	return nil
}
