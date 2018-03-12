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
	"math/rand"
	"testing"
	"time"

	"fmt"
	"k8s.io/kops/upup/pkg/fi"
	"k8s.io/kops/upup/pkg/fi/cloudup/gcp"
	"k8s.io/kops/upup/pkg/fi/cloudup/gke"
	"strings"
)

func Test_CreateCluster(t *testing.T) {
	t.Skip("needs to be mocked out")

	createGKECluster(t)

}

func newTarget(t *testing.T) *gke.GKEAPITarget {
	var cloud fi.Cloud
	project, err := gcp.DefaultProject()
	if err != nil {
		t.Fatalf("unable to get project: %v", err)
	}
	cloud, err = gke.NewGKECloud("us-central1-a", project, nil)
	if err != nil {
		t.Fatalf("error: %v", err)
	}

	target := gke.NewGKEAPITarget(cloud.(gke.GKECloud))

	if target == nil {
		t.Fatalf("error unable to build GKE cloud target")
	}

	return target
}

func createGKECluster(t *testing.T) (string, *gke.GKEAPITarget) {
	target := newTarget(t)

	name := fmt.Sprintf("test-cluster-%s", strings.ToLower(RandStringBytesMaskImprSrc(8)))

	cluster := &GKECluster{
		Name:                fi.String(name),
		Region:              fi.String("us-central1-a"),
		DefaultNodePoolName: fi.String("nodes"),
		BootDiskSizeGB:      fi.Int64(42),
		MachineType:         fi.String("n1-standard-1"),
		InitialCount:        fi.Int64(1),
	}

	err := cluster.RenderGKE(target, nil, cluster, nil)
	if err != nil {
		t.Fatalf("error rendering gke cluster: %v", err)
	}

	return name, target
}

const letterBytes = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"
const (
	letterIdxBits = 6                    // 6 bits to represent a letter index
	letterIdxMask = 1<<letterIdxBits - 1 // All 1-bits, as many as letterIdxBits
	letterIdxMax  = 63 / letterIdxBits   // # of letter indices fitting in 63 bits
)

var src = rand.NewSource(time.Now().UnixNano())

func RandStringBytesMaskImprSrc(n int) string {
	b := make([]byte, n)
	// A src.Int63() generates 63 random bits, enough for letterIdxMax characters!
	for i, cache, remain := n-1, src.Int63(), letterIdxMax; i >= 0; {
		if remain == 0 {
			cache, remain = src.Int63(), letterIdxMax
		}
		if idx := int(cache & letterIdxMask); idx < len(letterBytes) {
			b[i] = letterBytes[idx]
			i--
		}
		cache >>= letterIdxBits
		remain--
	}

	return string(b)
}
