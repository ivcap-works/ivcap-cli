// Copyright 2023 Commonwealth Scientific and Industrial Research Organisation (CSIRO) ABN 41 687 119 230
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package cmd

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	sdk "github.com/ivcap-works/ivcap-cli/pkg"
	a "github.com/ivcap-works/ivcap-cli/pkg/adapter"
	api "github.com/ivcap-works/ivcap-core-api/http/secret"
	"github.com/jedib0t/go-pretty/v6/table"
	"github.com/jedib0t/go-pretty/v6/text"

	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(secretCmd)

	// LIST
	secretCmd.AddCommand(listSecretsCmd)
	flags := listSecretsCmd.Flags()
	flags.StringVarP(&secPage, "page", "p", "", "page cursor")
	flags.StringVarP(&secOffset, "offset", "", "", "offset token")
	flags.StringVarP(&secFilter, "filter", "", "", "regexp filter by name")

	// GET
	secretCmd.AddCommand(getSecretCmd)

	// SET
	secretCmd.AddCommand(setSecretCmd)
	flags = setSecretCmd.Flags()
	flags.StringVarP(&secFile, "file", "f", "", "read secret from file")
	flags.StringVarP(&secExpires, "expire", "e", "", "secret expires in the format of '6h', '5d', '100m', '1040s'")
}

var (
	secPage, secOffset, secFilter, secFile, secExpires string
)

var (
	secretCmd = &cobra.Command{
		Use:     "secret",
		Aliases: []string{"secrets"},
		Short:   "Set and list secrets ",
	}

	listSecretsCmd = &cobra.Command{
		Use:     "list",
		Aliases: []string{"l"},
		Short:   "List existing secrets",

		RunE: func(cmd *cobra.Command, args []string) error {
			reqHost, err := getSecretHost()
			if err != nil {
				return err
			}
			adpr := CreateAdapter(true)
			req := &sdk.ListSecretsRequest{
				Page:        page,
				Limit:       limit,
				OffsetToken: secOffset,
				Filter:      filter,
			}

			switch outputFormat {
			case "json", "yaml":
				if res, err := sdk.ListSecretsRaw(context.Background(), reqHost, req, adpr, logger); err == nil {
					return a.ReplyPrinter(res, outputFormat == "yaml")
				} else {
					return err
				}
			default:
				if list, err := sdk.ListSecrets(context.Background(), reqHost, req, adpr, logger); err == nil {
					printSecretsTable(list)
				} else {
					return err
				}
			}
			return nil
		},
	}

	getSecretCmd = &cobra.Command{
		Use:     "get [flags] secret-name",
		Aliases: []string{"g"},
		Short:   "Get single secret, show its expiry time and sha1 value",
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			reqHost, err := getSecretHost()
			if err != nil {
				return err
			}
			adpr := CreateAdapter(true)

			req := &sdk.GetSecretRequest{
				SecretName: args[0],
			}

			switch outputFormat {
			case "json", "yaml":
				res, err := sdk.GetSecretRaw(context.Background(), reqHost, req, adpr, logger)
				if err != nil {
					return err
				}
				return a.ReplyPrinter(res, outputFormat == "yaml")
			default:
				secret, err := sdk.GetSecret(context.Background(), reqHost, req, adpr, logger)
				if err != nil {
					return err
				}
				printSecret(secret)
			}
			return nil
		},
	}

	setSecretCmd = &cobra.Command{
		Use:     "set [flags] secret-name  secret-value",
		Aliases: []string{"s"},
		Short:   "Set a single secret value, overwrite if already exists",
		Args:    cobra.RangeArgs(1, 2),
		RunE: func(cmd *cobra.Command, args []string) error {
			if secFile == "" && len(args) == 1 {
				return errors.New("need to specify secret value or secret value file")
			}
			var secValue string
			if len(args) >= 2 {
				secValue = args[1]
			}
			if secFile != "" {
				if _, err := os.Stat(secFile); errors.Is(err, os.ErrNotExist) {
					return fmt.Errorf("file %s not exists", secFile)
				}
				data, err := os.ReadFile(filepath.Clean(secFile))
				if err != nil {
					return fmt.Errorf("failed to read file: %s, err: %w", secFile, err)
				}
				secValue = strings.TrimSpace(string(data))
			}
			var expiresAt int64
			var err error
			if secExpires != "" {
				if expiresAt, err = parseSimpleTime(secExpires); err != nil {
					return fmt.Errorf("invalid expires time format: %w", err)
				}
			}
			reqHost, err := getSecretHost()
			if err != nil {
				return err
			}
			adpr := CreateAdapter(true)

			req := &api.SetRequestBody{
				SecretName:  args[0],
				SecretValue: secValue,
				ExpiryTime:  expiresAt,
			}

			if err = sdk.SetSecret(context.Background(), reqHost, req, adpr, logger); err != nil {
				return fmt.Errorf("sdk failed to set secret: %w", err)
			}

			hash256 := sha256.New()
			if _, err = io.WriteString(hash256, secValue); err != nil {
				return fmt.Errorf("failed to create hash: %w", err)
			}

			fmt.Printf("secret set, hash: %s\n", hex.EncodeToString(hash256.Sum(nil))[0:10])
			return nil
		},
	}
)

func printSecretsTable(list *api.ListResponseBody) {
	rows := make([]table.Row, len(list.Items))
	for i, item := range list.Items {
		expiresAt := "N/A"
		if item.ExpiryTime != nil && *item.ExpiryTime != 0 {
			expiresAt = time.Unix(*item.ExpiryTime, 0).Format(time.RFC3339)
		}
		rows[i] = table.Row{*item.SecretName, expiresAt}
	}
	if next := findNextSecretsPage(list.Links); next != "" {
		rows = append(rows, table.Row{"next page", next})
	}

	t := table.NewWriter()
	t.SetOutputMirror(os.Stdout)
	t.AppendHeader(table.Row{"Name", "Expires at"})
	t.AppendRows(rows)
	t.Render()
}

func printSecret(secret *api.GetResponseBody) {
	tw := table.NewWriter()
	tw.SetStyle(table.StyleLight)
	tw.Style().Options.SeparateColumns = false
	tw.Style().Options.SeparateRows = false
	tw.Style().Options.DrawBorder = false
	tw.SetColumnConfigs([]table.ColumnConfig{
		{Number: 1, Align: text.AlignRight},
	})

	rows := []table.Row{
		{"Name", *secret.SecretName},
		{"Hash", *secret.SecretValue},
	}
	if secret.ExpiryTime != nil && *secret.ExpiryTime != 0 {
		expiresAt := time.Unix(*secret.ExpiryTime, 0).Format(time.RFC3339)
		rows = append(rows, table.Row{"Expires at", expiresAt})
	}
	tw.AppendRows(rows)
	fmt.Printf("\n%s\n\n", tw.Render())
}

func findNextSecretsPage(links []*api.LinkTResponseBody) string {
	if links == nil {
		return ""
	}
	for _, l := range links {
		if l.Rel != nil && *l.Rel == "next" {
			return *l.Href
		}
	}
	return ""
}

func getSecretHost() (string, error) {
	ctxt, err := GetContextWithError("", true)
	if err != nil {
		return "", fmt.Errorf("can not get active context: %w", err)
	}

	u, err := url.Parse(ctxt.URL)
	if err != nil {
		return "", fmt.Errorf("invalid context URL: %s", ctxt.URL)
	}

	return u.Host, nil
}

func parseSimpleTime(input string) (int64, error) {
	if len(input) < 2 {
		return 0, fmt.Errorf("invalid input time format: %s", input)
	}

	unit := input[len(input)-1:]
	value := input[:len(input)-1]
	number, err := strconv.Atoi(value)
	if err != nil {
		return 0, fmt.Errorf("invalid number format: %s", value)
	}

	now := time.Now()
	var rs time.Time

	switch unit {
	case "d": // Days
		rs = now.Add(time.Duration(number) * 24 * time.Hour)
	case "m": // Minutes
		rs = now.Add(time.Duration(number) * 24 * time.Minute)
	case "s": // Seconds
		rs = now.Add(time.Duration(number) * 24 * time.Second)

	default:
		return 0, errors.New("unknown time unit")
	}

	return rs.Unix(), nil
}
