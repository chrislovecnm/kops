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

import "testing"

func Test_Get_Project(t *testing.T) {

	projectURL := "https://container.googleapis.com/v1/projects/510657513523/zones/us-central1-a/operations/operation-1520960668616-b3fcea02"
	project, err := getProject(projectURL)

	if err != nil {
		t.Fatalf("error parsing: %v", err)
	}

	if project != "510657513523" {
		t.Fatalf("wrong project name found, expected 510657513523 found: %s", project)
	}

}
