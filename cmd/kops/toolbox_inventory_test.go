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

package main

import (
	"bytes"
	"path"
	"testing"

	"k8s.io/kops/cmd/kops/util"
	"k8s.io/kops/upup/pkg/fi/cloudup"
)

func TestAllContainers(t *testing.T) {

	inputYAML := "in-v1alpha2-1.6.3.yaml"

	factoryOptions := &util.FactoryOptions{}
	factoryOptions.RegistryPath = "memfs://tests"

	h := NewIntegrationTestHarness(t)
	defer h.Close()

	h.SetupMockAWS()

	srcDir := "testdata"

	factory := util.NewFactory(factoryOptions)

	{
		options := &ToolboxInventoryOptions{}
		options.InitDefaults()
		options.ClusterName = "privateweave.example.com"
		options.Filenames = []string{path.Join(srcDir, inputYAML)}

		c, ig, err := readFiles(options)
		if err != nil {
			t.Fatalf("error running %s/%s files: %v", srcDir, inputYAML, err)
		}

		cs, err := getClientSet(factory)
		if err != nil {
			t.Fatalf("unable to get client set %v", err)
		}

		// TODO channel needs to be a file ... it is getting
		// github.com channel
		applyClusterCmd := &cloudup.ApplyClusterCmd{
			Clientset:      cs,
			DryRun:         true,
			Cluster:        c,
			InstanceGroups: ig,
			TargetName: cloudup.TargetDryRun,
//			Models:         []string{"config", "proto", },
		}

		err = applyClusterCmd.Run()

		if err != nil {
			t.Fatalf("error applying cluster build: %v", err)
		}

		a := applyClusterCmd.InventoryAssets

		if err != nil {
			t.Fatalf("error building inventory assests: %v", err)
		}

		assets := []string{
			"https://kubeupv2.s3.amazonaws.com/kops/1.5.0/images/protokube.tar.gz",
			"gcr.io/google_containers/k8s-dns-dnsmasq-nanny-amd64:1.14.1",
			"gcr.io/google_containers/kube-proxy:v1.6.3",
			"gcr.io/google_containers/pause-amd64:3.0",
			"https://storage.googleapis.com/kubernetes-release/release/v1.6.3/bin/linux/amd64/kubectl",
			"https://storage.googleapis.com/kubernetes-release/release/v1.6.3/bin/linux/amd64/kubectl.sha1",
			"https://kubeupv2.s3.amazonaws.com/kops/1.5.0/linux/amd64/utils.tar.gz",
			"https://kubeupv2.s3.amazonaws.com/kops/1.5.0/linux/amd64/utils.tar.gz.sha1",
			"https://storage.googleapis.com/kubernetes-release/release/v1.6.3/bin/linux/amd64/kubelet",
			"https://storage.googleapis.com/kubernetes-release/release/v1.6.3/bin/linux/amd64/kubelet.sha1",
			"https://kubeupv2.s3.amazonaws.com/kops/1.5.0/linux/amd64/nodeup",
			"https://kubeupv2.s3.amazonaws.com/kops/1.5.0/linux/amd64/nodeup.sha1",
			"https://raw.githubusercontent.com/kubernetes/kops/master/channels/stable",
			"https://storage.googleapis.com/kubernetes-release/network-plugins/cni-0799f5732f2a11b329d9e3d51b9c8f2e3759f2ff.tar.gz",
			"https://storage.googleapis.com/kubernetes-release/network-plugins/cni-0799f5732f2a11b329d9e3d51b9c8f2e3759f2ff.tar.gz.sha1",
			"gcr.io/google_containers/cluster-proportional-autoscaler-amd64:1.1.1",
			"gcr.io/google_containers/etcd:2.2.1",
			"gcr.io/google_containers/k8s-dns-dnsmasq-nanny-amd64:1.14.1",
			"gcr.io/google_containers/k8s-dns-kube-dns-amd64:1.14.1",
			"gcr.io/google_containers/k8s-dns-sidecar-amd64:1.14.1",
			"gcr.io/google_containers/kube-apiserver:v1.6.3",
			"gcr.io/google_containers/kube-controller-manager:v1.6.3",
			"gcr.io/google_containers/kube-proxy:v1.6.3",
			"gcr.io/google_containers/kube-scheduler:v1.6.3",
			"kope/dns-controller:1.6.1",
			"weaveworks/weave-kube:1.9.4",
			"weaveworks/weave-npc:1.9.4",
			"gcr.io/google_containers/pause-amd64:3.0",
		}

		var missing []string
		for _, asset := range assets {
			notFound := true
			for _, j := range a {
				if asset == j.Data {
					notFound = false
				}
			}

			if notFound {
				missing = append(missing, asset)
			}

		}

		if len(missing) != 0 {
			t.Fatalf("containers not found %s", missing)
		}

	}

}

func TestMinimalInventoryFull(t *testing.T) {
	runTestInventory(t, "privateweave.example.com", "testdata", "v1alpha2", false, 1)
}

func runTestInventory(t *testing.T, clusterName string, srcDir string, version string, private bool, zones int) {
	var stdout bytes.Buffer

	inputYAML := "in-" + version + ".yaml"

	factoryOptions := &util.FactoryOptions{}
	factoryOptions.RegistryPath = "memfs://tests"

	h := NewIntegrationTestHarness(t)
	defer h.Close()

	h.SetupMockAWS()

	factory := util.NewFactory(factoryOptions)

	{
		options := &ToolboxInventoryOptions{}
		options.InitDefaults()
		options.ClusterName = clusterName
		options.Filenames = []string{path.Join(srcDir, inputYAML)}

		err := RunToolboxInventory(factory, &stdout, options)
		if err != nil {
			t.Fatalf("error running %s/%s inventory tool name: %v", srcDir, inputYAML, err)
		}

	}

}
