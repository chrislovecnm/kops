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
	"strings"
	"testing"

	"k8s.io/kops/upup/pkg/fi"
)

func Test_CreateNodePool(t *testing.T) {
	t.Skip("needs to be mocked out")

	clusterName, target := createGKECluster(t)

	nodePool := &NodePool{
		Name:           fi.String(strings.ToLower(RandStringBytesMaskImprSrc(8))),
		BootDiskSizeGB: fi.Int64(42),
		MachineType:    fi.String("n1-standard-1"),
		InitialCount:   fi.Int64(1),
		Cluster:        &GKECluster{Name: fi.String(clusterName)},
	}

	err := nodePool.RenderGKE(target, nil, nodePool, nil)
	if err != nil {
		t.Fatalf("error creating nodepool: %v", err)
	}

}
