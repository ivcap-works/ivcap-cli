// Copyright 2024 Commonwealth Scientific and Industrial Research Organisation (CSIRO) ABN 41 687 119 230
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package cmd

import (
	"context"
	"fmt"

	sdk "github.com/ivcap-works/ivcap-cli/pkg"
	"github.com/spf13/cobra"
)

var forcePush, localImage bool

func init() {
	rootCmd.AddCommand(pkgCmd)

	pkgCmd.AddCommand(listPackageCmd)
	pkgCmd.AddCommand(pushPackageCmd)
	pushPackageCmd.Flags().BoolVarP(&forcePush, "force", "f", false, "Push packages even it already exists")
	pushPackageCmd.Flags().BoolVarP(&localImage, "local", "l", false, "Push packages from local docker daemon")
	pkgCmd.AddCommand(pullPackageCmd)
	pkgCmd.AddCommand(removePackageCmd)
}

var (
	pkgCmd = &cobra.Command{
		Use:     "package",
		Aliases: []string{"pkg", "pkgs", "packages"},
		Short:   "Push/pull and manage service packages",
	}

	listPackageCmd = &cobra.Command{
		Use:     "list [tag]",
		Aliases: []string{"ls"},
		Short:   "list service packages",
		Long:    `List the service packages under current account, or other accounts if you know the account id.`,
		Args:    cobra.RangeArgs(0, 1),
		RunE: func(cmd *cobra.Command, args []string) (err error) {
			ctxt := context.Background()
			var tag string
			if len(args) > 0 {
				tag = args[0]
			}
			res, err := sdk.ListPackages(ctxt, tag, CreateAdapter(true), logger)
			if err != nil {
				return err
			}
			if res != nil {
				for _, tag := range res.Items {
					fmt.Printf("%s\n", tag)
				}
			}
			return nil
		},
	}

	pushPackageCmd = &cobra.Command{
		Use:   "push [flags] tag",
		Short: "Push service package(docker image) to repository",
		Long:  `Before/After creating service, push the service package to a docker registry that the service can reference.`,
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) (err error) {
			srcPackageTag := args[0]
			_, err = sdk.PushServicePackage(srcPackageTag, forcePush, localImage, CreateAdapter(true), logger)
			if err != nil {
				return err
			}
			return nil
		},
	}

	pullPackageCmd = &cobra.Command{
		Use:   "pull tag",
		Short: "pull service package by tag",
		Long:  `Pull the service package by tag, from the ivcap service repository`,
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) (err error) {
			ctxt := context.Background()
			tag := args[0]
			err = sdk.PullPackage(ctxt, tag, CreateAdapter(true), logger)
			if err != nil {
				return err
			}
			return nil
		},
	}

	removePackageCmd = &cobra.Command{
		Use:     "remove tag",
		Aliases: []string{"rm", "delete"},
		Short:   "remove service package by tag",
		Long:    `Remove the service package by tag, from the ivcap service repository`,
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) (err error) {
			ctxt := context.Background()
			tag := args[0]
			err = sdk.RemovePackage(ctxt, tag, CreateAdapter(true), logger)
			if err != nil {
				return err
			}
			fmt.Printf("package removed\n")
			return nil
		},
	}
)
