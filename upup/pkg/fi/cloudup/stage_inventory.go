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

package cloudup

import (
	"bytes"
	"crypto/rand"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path"
	"strings"

	"k8s.io/kops/util/pkg/vfs"

	"github.com/golang/glog"
)

const (
	dockerExec = "docker"
)

type AssetTransferer interface {
	Transfer(asset *InventoryAsset) error
}

type FileAssetTransferer struct {
	fileRepo string
}

type ContainerAssetTransferer struct {
	containerRepo string
}

type ContainerFileAssetTransferer struct {
	containerRepo string
}

type StageInventory struct {
	assetTransferers map[string]AssetTransferer
	assets           []*InventoryAsset
}

func NewStageInventory(fileRepo, containerRepo string, assets []*InventoryAsset) *StageInventory {
	assetTransferers := map[string]AssetTransferer{
		AssetBinary: &FileAssetTransferer{
			fileRepo: fileRepo,
		},
		AssetContainer: &ContainerAssetTransferer{
			containerRepo: containerRepo,
		},
		AssetContainerBinary: &ContainerFileAssetTransferer{
			containerRepo: containerRepo,
		},
	}
	return &StageInventory{
		assetTransferers: assetTransferers,
		assets:           assets,
	}
}

func (i *StageInventory) Run() error {

	for _, asset := range i.assets {
		err := i.processAsset(asset)
		if err != nil {
			return fmt.Errorf("Error StageInventory.Run - Type:%s Data:%s - %v", asset.Type, asset.Data, err)
		}
	}

	return nil

}

func (i *StageInventory) processAsset(asset *InventoryAsset) error {

	assetTransferer := i.assetTransferers[asset.Type]

	glog.Infof("processing transferer: %#v - asset: %#v\n", assetTransferer, asset)

	err := assetTransferer.Transfer(asset)
	if err != nil {
		return fmt.Errorf("Error Transfering Asset - Type:%s Data:%s - %v", asset.Type, asset.Data, err)
	}

	return nil
}

func (f *FileAssetTransferer) Transfer(asset *InventoryAsset) error {
	glog.Infof("FileAssetTransferer.Transfer: %s - %s\n", asset.Type, asset.Data)

	glog.Infoln("FileAssetTransferer.Transfer: reading data...")
	data, err := vfs.Context.ReadFile(asset.Data)
	if err != nil {
		return fmt.Errorf("Error FileAssetTransferer.Transfer  unable to read path %q: %v", asset.Data, err)
	}

	filePath := strings.Split(asset.Data, "/")
	s3Path := fmt.Sprintf("%s/%s", f.fileRepo, filePath[len(filePath)-1])
	glog.Infof("FileAssetTransferer.Transfer: s3Path: %s\n", s3Path)
	destinationRegistry, err := vfs.Context.BuildVfsPath(s3Path)
	if err != nil {
		return fmt.Errorf("Error FileAssetTransferer.Transfer parsing registry path %q: %v", f.fileRepo, err)
	}

	glog.Infoln("FileAssetTransferer.Transfer: writing data...")
	err = destinationRegistry.WriteFile(data)
	if err != nil {
		return fmt.Errorf("Error FileAssetTransferer.Transfer destination path %q: %v", f.fileRepo, err)
	}

	return nil
}

func (c *ContainerAssetTransferer) Transfer(asset *InventoryAsset) error {

	glog.Infof("ContainerAssetTransferer.Transfer: %s - %s\n", asset.Type, asset.Data)

	// Download image
	glog.Infof("Downloading container image %s\n", asset.Data)
	args := []string{"pull", asset.Data}
	err := performExec(dockerExec, args)
	if err != nil {
		return err
	}

	//Tag image with new Repo
	dockerImageVersion := strings.Split(asset.Data, ":")
	originalImageName := dockerImageVersion[0]
	imageVersion := dockerImageVersion[1]
	imagePath := strings.Split(originalImageName, "/")
	tagName := fmt.Sprintf("%s/%s:%s", c.containerRepo, imagePath[len(imagePath)-1], imageVersion)
	glog.Infof("Tagging local image tagName[-]%s\n", tagName)
	err = tagAndPushToDocker(tagName, asset.Data)
	if err != nil {
		return fmt.Errorf("Error pushing docker image with tagName-'%s' baseDockerImageId-'%s': %v", tagName, asset.Data, err)
	}

	err = cleanUpDockerImages(tagName, asset.Data)
	if err != nil {
		return fmt.Errorf("Error cleanup images with tagName-'%s' baseDockerImageId-'%s': %v", tagName, asset.Data, err)
	}

	return nil
}

func (c *ContainerFileAssetTransferer) Transfer(asset *InventoryAsset) error {

	glog.Infof("ContainerFileAssetTransferer.Transfer starting: %s - %s\n", asset.Type, asset.Data)

	uuid, err := NewUUID()
	if err != nil {
		return fmt.Errorf("Error getting UUID for file '%s': %v", asset.Data, err)
	}

	pathParts := strings.Split(asset.Data, "/")
	localFile := fmt.Sprintf("/tmp/%s-%s", uuid, pathParts[len(pathParts)-1])

	glog.Infof("Local file: %s\n", localFile)

	dirMode := os.FileMode(0755)
	err = downloadFile(asset.Data, localFile, dirMode)
	if err != nil {
		return fmt.Errorf("Error downloading file '%s': %v", asset.Data, err)
	}
	// File Cleanup
	defer func() {
		err = os.Remove(localFile)
		if err != nil {
			glog.Warningf("Error Removing file-'%s': %v", localFile, err)
		}

	}()

	// Load the image into docker
	args := []string{"docker", "load", "-i", localFile}
	human := strings.Join(args, " ")

	glog.Infof("ContainerFileAssetTransferer.Transfer  Running command %s\n", human)
	cmd := exec.Command(args[0], args[1:]...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("error loading docker image with '%s': %v: %s", human, err, string(output))
	}

	dockerImageId := extractImageIdFromOutput(string(output))
	glog.Infof("ContainerFileAssetTransferer.Transfer Loaded image id: %s\n", dockerImageId)

	tagName := fmt.Sprintf("%s/%s", c.containerRepo, dockerImageId)
	err = tagAndPushToDocker(tagName, dockerImageId)
	if err != nil {
		return fmt.Errorf("Error pushing docker image with tagName-'%s' baseDockerImageId-'%s': %v", tagName, dockerImageId, err)
	}

	err = cleanUpDockerImages(tagName, dockerImageId)
	if err != nil {
		return fmt.Errorf("Error cleanup images with tagName-'%s' baseDockerImageId-'%s': %v", tagName, dockerImageId, err)
	}

	return nil
}

func extractImageIdFromOutput(output string) string {
	// Assumes oputput format is 'Loaded image: <imageId>'
	outputValues := strings.Split(string(output), "Loaded image: ")
	return strings.Trim(outputValues[1], "\n")
}

func downloadFile(url string, destPath string, dirMode os.FileMode) error {
	err := os.MkdirAll(path.Dir(destPath), dirMode)
	if err != nil {
		return fmt.Errorf("error creating directories for destination file %q: %v", destPath, err)
	}

	output, err := os.Create(destPath)
	if err != nil {
		return fmt.Errorf("error creating file for download %q: %v", destPath, err)
	}
	defer output.Close()

	glog.Infof("Downloading %q", url)

	response, err := http.Get(url)
	if err != nil {
		return fmt.Errorf("error doing HTTP fetch of %q: %v", url, err)
	}
	defer response.Body.Close()

	_, err = io.Copy(output, response.Body)
	if err != nil {
		return fmt.Errorf("error downloading HTTP content from %q: %v", url, err)
	}
	return nil
}

// Stolen from: https://play.golang.org/p/4FkNSiUDMg
// newUUID generates a random UUID according to RFC 4122
func NewUUID() (string, error) {
	uuid := make([]byte, 16)
	n, err := io.ReadFull(rand.Reader, uuid)
	if n != len(uuid) || err != nil {
		return "", err
	}
	// variant bits; see section 4.1.1
	uuid[8] = uuid[8]&^0xc0 | 0x80
	// version 4 (pseudo-random); see section 4.1.3
	uuid[6] = uuid[6]&^0xf0 | 0x40
	return fmt.Sprintf("%x-%x-%x-%x-%x", uuid[0:4], uuid[4:6], uuid[6:8], uuid[8:10], uuid[10:]), nil
}

func tagAndPushToDocker(tagName, imageId string) error {

	glog.Infof("Tagging local image tagName[-]%s\n", tagName)
	args := []string{"tag", imageId, tagName}
	err := performExec(dockerExec, args)
	if err != nil {
		return fmt.Errorf("Docker Error - tagging tagName '%s' - imageId '%s' : %v", tagName, imageId, err)
	}

	// Push image to new Repo
	glog.Infof("Pushing image tagName[-]%s\n", tagName)
	args = []string{"push", tagName}
	err = performExec(dockerExec, args)
	if err != nil {
		return fmt.Errorf("Docker Error - pushing tagName '%s': %v", tagName, err)
	}

	return nil
}

func cleanUpDockerImages(pushedImageId, baseImageId string) error {

	args := []string{"rmi", "-f", pushedImageId}
	glog.Infof("Removing pushed container image %s\n", strings.Join(args, " "))
	err := performExec(dockerExec, args)
	if err != nil {
		return fmt.Errorf("Docker Error - removing pushed container image '%s': %v", pushedImageId, err)
	}

	args = []string{"rmi", "-f", baseImageId}
	glog.Infof("Removing base container image %s\n", strings.Join(args, " "))
	err = performExec(dockerExec, args)
	if err != nil {
		return fmt.Errorf("Docker Error - removing base container image '%s': %v", baseImageId, err)
	}

	return nil
}

func performExec(cmdStr string, args []string) error {

	binary, err := exec.LookPath(cmdStr)
	if err != nil {
		return fmt.Errorf("Error finding executable file: %s - %v", cmdStr, err)
	}

	cmd := exec.Command(binary, args...)
	var out bytes.Buffer
	var errOut bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &errOut
	err = cmd.Run()
	if err != nil {
		err = fmt.Errorf("%v -- %s\n", err, errOut.String())
		return err
	}
	return nil
}
