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
	"fmt"
	"io"

	"github.com/spf13/cobra"
	"k8s.io/kops/cmd/kops/util"
	api "k8s.io/kops/pkg/apis/kops"
	"k8s.io/kops/upup/pkg/fi/cloudup"
	"k8s.io/kubernetes/pkg/kubectl/cmd/templates"
	"k8s.io/kubernetes/pkg/util/i18n"
	"strings"
)

type ToolboxStageInventoryOptions struct {
	// Maybe we may this a sub command then?
	*ToolboxInventoryOptions
	Repository      string
	FileDestination string
	StageFiles      bool
	StageContainers bool
}

func (o *ToolboxStageInventoryOptions) InitDefaults() {
	o.Channel = api.DefaultChannel
	o.Output = OutputTable
	o.Channel = "stable"
	o.StageContainers = true
	o.StageFiles = true
}

var (
	toolbox_stage_inventory_long = templates.LongDesc(i18n.T(`
		Stage inventory files to specified destinations(Repository/FileDestination).
		
		Note: 
		
		1. This command assumes Docker is installed and the user has the privileges to load and push images.
		2. User is authenticated to the provided Docker repository.`))

	toolbox_stage_inventory_example = templates.Examples(i18n.T(`
		# Stage inventory files from a yaml file
		kops toolbox stage-inventory --repository quay.io/vorstella --fileDestination s3://mybucket -f mycluster.yaml

		`))

	toolbox_stage_inventory_short = i18n.T(`Stage inventory files to the specified destinations(Repository/FileDestination).`)
	toolbox_stage_inventory_use   = i18n.T("stage-inventory")
)

// TODO need to document all of the public methods. Follow go standards.

func NewCmdToolboxStageInventory(f *util.Factory, out io.Writer) *cobra.Command {
	options := &ToolboxStageInventoryOptions{
		ToolboxInventoryOptions: &ToolboxInventoryOptions{},
	}
	options.InitDefaults()

	options.ClusterName = rootCommand.ClusterName()

	cmd := &cobra.Command{
		Use:     toolbox_stage_inventory_use,
		Short:   toolbox_stage_inventory_short,
		Example: toolbox_stage_inventory_example,
		Long:    toolbox_stage_inventory_long,
		Run: func(cmd *cobra.Command, args []string) {
			if err := rootCommand.ProcessArgs(args); err != nil {
				exitWithError(err)
			}

			err := rootCommand.ProcessArgs(args)

			if err != nil {
				exitWithError(err)
				return
			}

			options.ClusterName = rootCommand.clusterName

			if len(options.Filenames) == 0 && options.ClusterName == "" {
				exitWithError(fmt.Errorf("--filename or --name option must be used to supply cluster information."))
				return
			}

			if options.FileDestination == "" && options.StageFiles {
				exitWithError(fmt.Errorf("Please provide s3 location via --file-destination flag."))
				return
			}

			if options.Repository == "" && options.StageContainers {
				exitWithError(fmt.Errorf("Please provide repository location via --repository flag."))
				return
			}

			if !options.StageFiles && !options.StageContainers {
				exitWithError(fmt.Errorf("Please choose at least one of --stage-containers or --stage-files flag(s)."))
				return
			}

			err = RunToolboxStageInventory(f, out, options)

			if err != nil {
				exitWithError(err)
				return
			}
		},
	}

	cmd.Flags().StringVarP(&options.Channel, "channel", "c", options.Channel, "Channel for default versions and configuration to use")
	cmd.Flags().StringVarP(&options.KubernetesVersion, "kubernetes-version", "k", options.KubernetesVersion, "Version of kubernetes to run (defaults to version in channel)")
	cmd.Flags().StringArrayVarP(&options.Filenames, "filename", "f", options.Filenames, "Filename to use to create the resource")
	cmd.Flags().StringVarP(&options.Repository, "repository", "r", options.Repository, "Repository location used to stage inventory containers")
	cmd.Flags().StringVarP(&options.FileDestination, "file-destination", "d", options.FileDestination, "FileDestination location used to stage inventory files")
	cmd.Flags().BoolVar(&options.StageContainers, "stage-containers", options.StageContainers, "Stage containers")
	cmd.Flags().BoolVar(&options.StageFiles, "stage-files", options.StageFiles, "Stage files")
	cmd.MarkFlagRequired("file-destination")
	cmd.MarkFlagRequired("repository")

	return cmd
}

// RunToolboxStageInventory executes the business logic to stage inventory files to the specified repositories.
func RunToolboxStageInventory(f *util.Factory, out io.Writer, options *ToolboxStageInventoryOptions) error {

	assets, _, err := extractAssets(f, options.ToolboxInventoryOptions)
	if err != nil {
		return fmt.Errorf("Error extracting assets file(s) %q, %v", options.Filenames, err)
	}

	options.FileDestination = strings.TrimSuffix(options.FileDestination, "/")

	// FIXME refactor too many parameters now :(
	stageInventory := cloudup.NewStageInventory(options.FileDestination, options.StageFiles, options.Repository, options.StageContainers, assets)
	err = stageInventory.Run()
	if err != nil {
		return fmt.Errorf("Error processing assets file(s) %q, %v", options.Filenames, err)
	}

	return nil
}
