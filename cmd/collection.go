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
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/araddon/dateparse"
	sdk "github.com/ivcap-works/ivcap-cli/pkg"
	a "github.com/ivcap-works/ivcap-cli/pkg/adapter"
	api "github.com/ivcap-works/ivcap-core-api/http/aspect"
	"github.com/jedib0t/go-pretty/v6/table"
	"github.com/jedib0t/go-pretty/v6/text"

	"github.com/spf13/cobra"
)

const CollectionSchema = "urn:ivcap:schema:artifact-collection.1"

const DEF_MAX_COLLECTION_ITEMS = 10

var (
	maxCollectionItems int
	collectionDir      string
)

func init() {
	rootCmd.AddCommand(collectionCmd)

	// LIST
	collectionCmd.AddCommand(listCollectionCmd)
	addListFlags(listCollectionCmd)

	// CREATE
	collectionCmd.AddCommand(createArtifactCollectionCmd)
	createArtifactCollectionCmd.Flags().StringVar(&collectionDir, "dir", "", "Path to directory containing files to add to collection")

	// collectionCmd.AddCommand(collectionAddCmd)
	// addFlags(collectionAddCmd, []Flag{Schema, InputFormat, Policy})
	// collectionAddCmd.Flags().StringVarP(&collectionFile, "file", "f", "", "Path to file containing collection content")

	// collectionCmd.AddCommand(collectionUpdateCmd)
	// addFlags(collectionUpdateCmd, []Flag{Schema, InputFormat, Policy})
	// collectionUpdateCmd.Flags().StringVarP(&collectionFile, "file", "f", "", "Path to file containing metdata")

	collectionCmd.AddCommand(collectionGetCmd)
	addFlags(collectionGetCmd, []Flag{AtTime})
	collectionGetCmd.Flags().IntVarP(&maxCollectionItems, "max-items", "l", DEF_MAX_COLLECTION_ITEMS, "max number of items shown")

	// collectionCmd.AddCommand(collectionQueryCmd)
	// addFlags(collectionQueryCmd, []Flag{Schema, Entity})
	// collectionQueryCmd.Flags().StringVarP(&collectionJsonFilter, "content-path", "c", "", "json path filter on collection's content ('$.images[*] ? (@.size > 10000)')")
	// collectionQueryCmd.Flags().BoolVar(&collectionIncludeContent, "include-content", false, "if set, also include collection's content in list")
	// addListFlags(collectionQueryCmd)

	// collectionCmd.AddCommand(collectionRetractCmd)
}

type CollectionContent struct {
	CollectionID string   `json:"collection"`
	Artifacts    []string `json:"artifacts"`
}

var (
	collectionCmd = &cobra.Command{
		Use:     "collection",
		Aliases: []string{"c", "collection"},
		Short:   "Create and manage collections",
	}

	listCollectionCmd = &cobra.Command{
		Use:     "list",
		Aliases: []string{"l"},
		Short:   "List defined collections",

		RunE: func(cmd *cobra.Command, args []string) error {
			selector := sdk.AspectSelector{
				SchemaPrefix:   CollectionSchema,
				ListRequest:    *createListRequest(),
				IncludeContent: false,
			}
			ctxt := context.Background()
			if list, res, err := sdk.ListAspect(ctxt, selector, CreateAdapter(true), logger); err == nil {
				switch outputFormat {
				case "json", "yaml":
					return a.ReplyPrinter(res, outputFormat == "yaml")
				default:
					printCollectionTable(list, false)
				}
				return nil
			} else {
				return err
			}
		},
	}

	createArtifactCollectionCmd = &cobra.Command{
		Use:   "create collectionURN [flags] --dir",
		Short: "Create a new collection",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			id := GetHistory(args[0])
			if !URN_CHECK.Match([]byte(id)) {
				cobra.CheckErr(fmt.Sprintf("'%s' is not a URN", id))
			}
			if collectionDir == "" {
				cobra.CheckErr("Missing '--dir' flag")
				return
			}
			entries, err := os.ReadDir(collectionDir)
			if err != nil {
				cobra.CheckErr(fmt.Sprintf("While reading directory '%s'", collectionDir))
				return
			}
			id2name := make(map[string]string)
			var aids []string
			addAID := func(name string, aid string) {
				if other, ok := id2name[aid]; ok {
					cobra.CheckErr(fmt.Sprintf("'%s' is apparently uploaded with same URN as '%s'", name, other))
				}
				id2name[aid] = name
				aids = append(aids, aid)
			}

			for _, el := range entries {
				name := el.Name()
				if strings.HasPrefix(name, ".") {
					continue
				}
				fn := filepath.Join(collectionDir, name)
				if mfn, exists := getArtifactMetaFileFor(fn); exists {
					aid := getArtifactIdFromMeta(*mfn)
					addAID(name, aid)
					fmt.Printf("... Skipping '%s', already uploaded as '%s'\n", name, aid)
					continue
				}
				addAID(name, uploadArtifact(fn, false, ""))
			}
			content := CollectionContent{
				CollectionID: id,
				Artifacts:    aids,
			}
			var cb []byte
			if cb, err = json.Marshal(content); err != nil {
				cobra.CheckErr(fmt.Sprintf("while marshalling collection list - %v", err))
			}
			ctxt := context.Background()
			_, err = sdk.AddUpdateAspect(ctxt, true, id, CollectionSchema, policy, cb, CreateAdapter(true), logger)
			if err != nil {
				cobra.CheckErr(fmt.Sprintf("while creating/updating collection list - %v", err))
			}
			if !silent {
				if err := getCollection(id); err != nil {
					cobra.CheckErr(fmt.Sprintf("while printing collection details - %v", err))
				}
			}
		},
	}

	// collectionAddCmd = &cobra.Command{
	// 	Use:     "add entityURN [-s schemaName] -f -|collection --format json|yaml [flags]",
	// 	Short:   "Add collection of a specific schema to an entity",
	// 	Aliases: []string{"a", "+"},
	// 	Long:    `.....`,
	// 	Args:    cobra.ExactArgs(1),
	// 	RunE: func(cmd *cobra.Command, args []string) (err error) {
	// 		return addCollectionUpdateCmd(true, cmd, args)
	// 	},
	// }

	// collectionUpdateCmd = &cobra.Command{
	// 	Use:     "update entityURN [-s schemaName] -f -|collection --format json|yaml",
	// 	Short:   "Update an collection record for an entity and a specific schema",
	// 	Aliases: []string{"a", "+"},
	// 	Long:    `This command will only succeed if there is only one active record for the entity/schema pair`,
	// 	Args:    cobra.ExactArgs(1),
	// 	RunE: func(cmd *cobra.Command, args []string) (err error) {
	// 		return addCollectionUpdateCmd(false, cmd, args)
	// 	},
	// }

	collectionGetCmd = &cobra.Command{
		Use:     "get collectionURN",
		Short:   "Get a specific collection record",
		Aliases: []string{"g"},
		// Long:    `.....`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) (err error) {
			return getCollection(GetHistory(args[0]))
		},
	}

// 	collectionRetractCmd = &cobra.Command{
// 		Use:     "retract collectionURN [flags]",
// 		Short:   "Retract a specific collection record",
// 		Aliases: []string{"r"},
// 		// Long:    `.....`,
// 		Args: cobra.ExactArgs(1),
// 		RunE: func(cmd *cobra.Command, args []string) (err error) {
// 			collectionID := GetHistory(args[0])
// 			ctxt := context.Background()
// 			_, err = sdk.RetractCollection(ctxt, collectionID, CreateAdapter(true), logger)
// 			return
// 		},
// 	}

// 	collectionQueryCmd = &cobra.Command{
// 		Use:     "query [-e entity] [-s schemaPrefix] [flags]",
// 		Short:   "Query the collection store for any combination of entity, schema and time.",
// 		Aliases: []string{"q", "search", "s", "list", "l"},
// 		// Long:    `.....`,
// 		RunE: func(cmd *cobra.Command, args []string) (err error) {
// 			if entityURN == "" && schemaPrefix == "" && page == "" {
// 				cobra.CheckErr("Need at least one of '--schema', '--entity' or '--page'")
// 			}
// 			if entityURN != "" {
// 				entityURN = GetHistory(entityURN)
// 			}
// 			selector := sdk.CollectionSelector{
// 				Entity:         entityURN,
// 				SchemaPrefix:   schemaPrefix,
// 				ListRequest:    *createListRequest(),
// 				IncludeContent: collectionIncludeContent,
// 			}

// 			if collectionJsonFilter != "" {
// 				selector.JsonFilter = &collectionJsonFilter
// 			}

//			ctxt := context.Background()
//			if list, res, err := sdk.ListCollection(ctxt, selector, CreateAdapter(true), logger); err == nil {
//				switch outputFormat {
//				case "json":
//					return a.ReplyPrinter(res, false)
//				case "yaml":
//					return a.ReplyPrinter(res, true)
//				default:
//					printCollectionTable(list, false)
//				}
//				return nil
//			} else {
//				return err
//			}
//		},
//	}
)

func getCollection(collectionID string) (err error) {
	selector := sdk.AspectSelector{
		Entity:         collectionID,
		SchemaPrefix:   CollectionSchema,
		IncludeContent: true,
		ListRequest: sdk.ListRequest{
			Limit: 2,
		},
	}
	if atTime != "" {
		t, err := dateparse.ParseLocal(atTime)
		if err != nil {
			cobra.CheckErr(fmt.Sprintf("Can't parse '%s' into a date - %s", atTime, err))
		}
		selector.AtTime = &t
	}

	ctxt := context.Background()
	adapter := CreateAdapter(true)
	var list *api.ListResponseBody
	if list, _, err = sdk.ListAspect(ctxt, selector, adapter, logger); err != nil {
		return
	}
	if len(list.Items) != 1 {
		cobra.CheckErr("API Error: Check deployment - Collection is not well defined")
	}
	aspectID := list.Items[0].ID
	switch outputFormat {
	case "json", "yaml":
		if res, err := sdk.GetAspectRaw(ctxt, *aspectID, adapter, logger); err == nil {
			return a.ReplyPrinter(res, outputFormat == "yaml")
		} else {
			return err
		}
	default:
		if res, err := sdk.GetAspect(ctxt, *aspectID, adapter, logger); err == nil {
			printCollection(res)
			return nil
		} else {
			return err
		}
	}
}

// func addCollectionUpdateCmd(isAdd bool, cmd *cobra.Command, args []string) (err error) {
// 	entity := args[0]
// 	pyld, err := payloadFromFile(collectionFile, inputFormat)
// 	if err != nil {
// 		cobra.CheckErr(fmt.Sprintf("While reading collection file '%s' - %s", collectionFile, err))
// 	}

// 	collection, err := pyld.AsObject()
// 	if err != nil {
// 		cobra.CheckErr(fmt.Sprintf("Cannot parse collection file '%s' - %s", collectionFile, err))
// 	}
// 	var schema string
// 	schema = schemaURN
// 	if schema == "" {
// 		if s, ok := collection["$schema"]; ok {
// 			schema = fmt.Sprintf("%s", s)
// 		} else {
// 			cobra.CheckErr("Missing schema name")
// 		}
// 	}
// 	logger.Debug("add/update collection", log.String("entity", entity), log.String("schema", schema), log.Reflect("pyld", collection))
// 	ctxt := context.Background()
// 	res, err := sdk.AddUpdateCollection(ctxt, isAdd, entity, schema, policy, pyld.AsBytes(), CreateAdapter(true), logger)
// 	if err != nil {
// 		return err
// 	}
// 	if silent {
// 		if m, err := res.AsObject(); err == nil {
// 			fmt.Printf("%s\n", m["record-id"])
// 		} else {
// 			cobra.CheckErr(fmt.Sprintf("Parsing reply: %s", res.AsBytes()))
// 		}
// 	} else {
// 		return a.ReplyPrinter(res, outputFormat == "yaml")
// 	}
// 	return nil
// }

func printCollection(res *api.ReadResponseBody) {
	// ID *string `form:"id,omitempty" json:"id,omitempty" xml:"id,omitempty"`
	// Entity *string `form:"entity,omitempty" json:"entity,omitempty" xml:"entity,omitempty"`
	// Schema *string `form:"schema,omitempty" json:"schema,omitempty" xml:"schema,omitempty"`
	// Content any `form:"content,omitempty" json:"content,omitempty" xml:"content,omitempty"`
	// ContentType *string `json:"content-type,omitempty"`
	// ValidFrom *string `form:"valid-from,omitempty" json:"valid-from,omitempty" xml:"valid-from,omitempty"`
	// ValidTo *string `form:"valid-to,omitempty" json:"valid-to,omitempty" xml:"valid-to,omitempty"`
	// Asserter *string `form:"asserter,omitempty" json:"asserter,omitempty" xml:"asserter,omitempty"`
	// Retracter *string              `form:"retracter,omitempty"

	if res.ContentType == nil || *res.ContentType != "application/json" {
		cobra.CheckErr("Cannot find collection member list in reply - should never happen")
	}
	var cm map[string]any
	var ok bool
	if cm, ok = res.Content.(map[string]any); !ok {
		cobra.CheckErr("Unexpected content type")
	}
	var list []any
	if list, ok = cm["artifacts"].([]any); !ok {
		cobra.CheckErr("Unexpected missing content - 'artifacts'")
	}
	tw2 := table.NewWriter()
	tw2.AppendHeader(table.Row{fmt.Sprintf("Artifacts (%d)", len(list))})
	tw2.SetStyle(table.StyleLight)
	rows := make([]table.Row, len(list))
	for i, el := range list {
		if i >= maxCollectionItems {
			rows[i] = table.Row{fmt.Sprintf("... %d more", len(list)-i)}
			break
		}
		if a, ok := el.(string); ok {
			rows[i] = table.Row{fmt.Sprintf("(%s) %s", MakeHistory(&a), a)}
		}
	}
	tw2.AppendRows(rows)

	p := []table.Row{
		{"Entity", fmt.Sprintf("%s (%s)", *res.Entity, MakeHistory(res.Entity))},
		{"Asserter", safeString(res.Asserter)},
	}
	if res.ValidTo == nil {
		p = append(p, table.Row{"LastUpdated", safeDate(res.ValidFrom, true)})
	} else {
		p = append(p,
			table.Row{"ValidFrom", safeDate(res.ValidFrom, true)},
			table.Row{"Retracter", safeString(res.Retracter)},
			table.Row{"ValidTo", safeDate(res.ValidTo, true)},
		)
	}
	p = append(p, table.Row{"Items", tw2.Render()})

	tw := table.NewWriter()
	tw.SetStyle(table.StyleLight)
	tw.Style().Options.SeparateColumns = false
	tw.Style().Options.SeparateRows = false
	tw.Style().Options.DrawBorder = false
	tw.SetColumnConfigs([]table.ColumnConfig{
		{Number: 1, Align: text.AlignRight},
		// {Number: 2, WidthMax: 80},
	})
	tw.AppendRows(p)
	fmt.Printf("\n%s\n\n", tw.Render())
}

func printCollectionTable(list *api.ListResponseBody, wide bool) {
	tw2 := table.NewWriter()
	tw2.AppendHeader(table.Row{"ID", "Last Updated"})
	tw2.SetStyle(table.StyleLight)
	rows := make([]table.Row, len(list.Items))
	for i, p := range list.Items {
		rows[i] = table.Row{
			fmt.Sprintf("(%s) %s", MakeHistory(p.Entity), *p.Entity),
			safeDate(p.ValidFrom, true),
		}
	}
	tw2.AppendRows(rows)

	tw := table.NewWriter()
	tw.SetStyle(table.StyleLight)
	tw.Style().Options.SeparateColumns = false
	tw.Style().Options.SeparateRows = false
	tw.Style().Options.DrawBorder = false
	tw.SetColumnConfigs([]table.ColumnConfig{
		{Number: 1, Align: text.AlignRight},
		// {Number: 2, WidthMax: 80},
	})

	p := []table.Row{}
	if list.AtTime != nil {
		p = append(p, table.Row{"At Time", safeDate(list.AtTime, false)})
	}
	p = append(p, table.Row{"Collections", tw2.Render()})
	p = addNextPageRow(findNextCollectionPage(list.Links), p)
	tw.AppendRows(p)

	fmt.Printf("\n%s\n\n", tw.Render())
}

func findNextCollectionPage(links []*api.LinkTResponseBody) *string {
	if links == nil {
		return nil
	}
	for _, l := range links {
		if l.Rel != nil && *l.Rel == "next" {
			return l.Href
		}
	}
	return nil
}
