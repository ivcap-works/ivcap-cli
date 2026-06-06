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
	"net/url"
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

// CollectionSchema is the aspect schema used to store a collection definition
// (name + optional description).
const CollectionSchema = "urn:ivcap:schema:collection.1"

// CollectionItemSchema is the aspect schema used to record that an item
// (artifact or other entity URN) is a member of a collection.
const CollectionItemSchema = "urn:ivcap:schema:collection-item.1"

const DEF_MAX_COLLECTION_ITEMS = 10

var (
	maxCollectionItems    int
	collectionDir         string
	collectionDescription string
	collectionNameFilter  string
)

func init() {
	rootCmd.AddCommand(collectionCmd)

	// LIST
	collectionCmd.AddCommand(listCollectionCmd)
	addListFlags(listCollectionCmd)
	listCollectionCmd.Flags().StringVar(&collectionNameFilter, "name", "",
		`Filter by name using a JSONPath comparison expression applied to $.name,
e.g. '== "My Collection"', 'starts with "test"', 'like_regex ".*research.*" flag "i"'`)

	// CREATE – only stores the collection definition (name, description)
	collectionCmd.AddCommand(createArtifactCollectionCmd)
	addFlags(createArtifactCollectionCmd, []Flag{Name, Policy})
	createArtifactCollectionCmd.Flags().StringVar(&collectionDescription, "description", "", "Optional description of the collection")

	// ADD – adds item URNs (or uploads files) to an existing collection
	collectionCmd.AddCommand(collectionAddCmd)
	collectionAddCmd.Flags().StringVar(&collectionDir, "dir", "", "Directory or glob pattern of files to upload and add")

	// REMOVE – retracts collection-item aspects
	collectionCmd.AddCommand(collectionRemoveCmd)

	// RETRACT – retracts all collection-item aspects then the collection itself
	collectionCmd.AddCommand(collectionRetractCmd)

	// GET
	collectionCmd.AddCommand(collectionGetCmd)
	addFlags(collectionGetCmd, []Flag{AtTime})
	collectionGetCmd.Flags().IntVarP(&maxCollectionItems, "max-items", "l", DEF_MAX_COLLECTION_ITEMS, "max number of items shown")
}

// CollectionDef is the content body for the collection definition aspect.
type CollectionDef struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
}

// CollectionItem is the content body for each collection-item aspect.
type CollectionItem struct {
	Collection string `json:"collection"`
	Item       string `json:"item"`
}

var (
	collectionCmd = &cobra.Command{
		Use:     "collection",
		Aliases: []string{"c", "collections"},
		Short:   "Create and manage collections",
	}

	listCollectionCmd = &cobra.Command{
		Use:     "list",
		Aliases: []string{"l"},
		Short:   "List defined collections",
		Long: `List defined collections.

Use --name to filter by the collection name stored in the aspect content.
The value is a JSONPath comparison expression applied to the '$.name' field,
for example:

  ivcap collection list --name '== "My Collection"'
  ivcap collection list --name 'starts with "test"'
  ivcap collection list --name 'like_regex ".*ocean.*" flag "i"'`,

		RunE: func(cmd *cobra.Command, args []string) error {
			selector := sdk.AspectSelector{
				SchemaPrefix:   CollectionSchema,
				ListRequest:    *createListRequest(),
				IncludeContent: true, // always fetch content so we can show the name
			}
			if collectionNameFilter != "" {
				jf := fmt.Sprintf(`$.name ? (@ %s)`, collectionNameFilter)
				selector.JsonFilter = &jf
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

	// createArtifactCollectionCmd creates a collection definition aspect.
	// The collectionURN arg is the entity that identifies this collection.
	// Use "collection add" afterwards to associate items with it.
	createArtifactCollectionCmd = &cobra.Command{
		Use:   "create collectionURN [flags]",
		Short: "Create a new collection definition",
		Long: `Create a new collection definition stored as a DataFabric aspect.

The collectionURN must be a well-formed URN that will serve as the entity
identifier for all collection-item records. After creating the collection,
use 'collection add' to add artifact (or other entity) URNs to it.`,
		Args: cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			id := GetHistory(args[0])
			if !URN_CHECK.Match([]byte(id)) {
				cobra.CheckErr(fmt.Sprintf("'%s' is not a URN", id))
				return
			}
			if name == "" {
				cobra.CheckErr("Missing '--name' flag")
				return
			}
			def := CollectionDef{
				Name:        name,
				Description: collectionDescription,
			}
			cb, err := json.Marshal(def)
			if err != nil {
				cobra.CheckErr(fmt.Sprintf("while marshalling collection definition - %v", err))
				return
			}
			ctxt := context.Background()
			_, err = sdk.AddUpdateAspect(ctxt, false, id, CollectionSchema, policy, cb, CreateAdapter(true), logger)
			if err != nil {
				cobra.CheckErr(fmt.Sprintf("while creating collection - %v", err))
				return
			}
			if !silent {
				if err := getCollection(id); err != nil {
					cobra.CheckErr(fmt.Sprintf("while printing collection details - %v", err))
				}
			}
		},
	}

	// collectionAddCmd adds items to an existing collection.
	//
	// Item URNs can be supplied directly as extra positional arguments.
	// When --dir is given (directory path or glob), matching files are first
	// uploaded as artifacts and the resulting artifact URNs are used.
	// In all cases a collection-item aspect (CollectionItemSchema) is created
	// for every new item, skipping any that already belong to the collection.
	collectionAddCmd = &cobra.Command{
		Use:   "add collectionURN [urn...] [--dir <dir-or-glob>]",
		Short: "Add item(s) to an existing collection",
		Long: `Add one or more items to an existing collection.

Items can be specified in two ways (both may be combined):

  1. As positional URN arguments after the collection URN.
  2. Via --dir: a directory path or glob pattern. Each matching file is
     uploaded as an artifact (skipped if already uploaded) and the resulting
     artifact URN is used as the item.

A 'collection-item' aspect (` + CollectionItemSchema + `) is created for
each new item. Duplicates (same collection + item) are detected and skipped.`,
		Args: cobra.MinimumNArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			collectionID := GetHistory(args[0])
			if !URN_CHECK.Match([]byte(collectionID)) {
				cobra.CheckErr(fmt.Sprintf("'%s' is not a URN", collectionID))
				return
			}

			var itemURNs []string

			// 1. Direct URN arguments
			for _, arg := range args[1:] {
				urn := GetHistory(arg)
				if !URN_CHECK.Match([]byte(urn)) {
					cobra.CheckErr(fmt.Sprintf("'%s' is not a URN", urn))
					return
				}
				itemURNs = append(itemURNs, urn)
			}

			// 2. Files from --dir (directory or glob)
			if collectionDir != "" {
				files, err := resolveCollectionFiles(collectionDir)
				if err != nil {
					cobra.CheckErr(fmt.Sprintf("resolving '%s' - %v", collectionDir, err))
					return
				}
				if len(files) == 0 {
					cobra.CheckErr(fmt.Sprintf("no files matched '%s'", collectionDir))
					return
				}
				for _, fn := range files {
					// Upload (or detect existing) artifact; no X-Collection header
					aid := uploadArtifact(fn, false, "")
					if aid != "" {
						itemURNs = append(itemURNs, aid)
					}
				}
			}

			if len(itemURNs) == 0 {
				cobra.CheckErr("No items to add: provide URN arguments or use '--dir'")
				return
			}

			adapter := CreateAdapter(true)
			ctxt := context.Background()

			added := 0
			skipped := 0
			for _, itemURN := range itemURNs {
				exists, err := isAlreadyCollectionItem(ctxt, collectionID, itemURN, adapter)
				if err != nil {
					cobra.CheckErr(fmt.Sprintf("while checking membership of '%s' - %v", itemURN, err))
					return
				}
				if exists {
					if !silent {
						fmt.Printf("Skipping '%s': already a member of collection\n", itemURN)
					}
					skipped++
					continue
				}
				ci := CollectionItem{Collection: collectionID, Item: itemURN}
				cb, err := json.Marshal(ci)
				if err != nil {
					cobra.CheckErr(fmt.Sprintf("while marshalling collection-item - %v", err))
					return
				}
				_, err = sdk.AddUpdateAspect(ctxt, true, collectionID, CollectionItemSchema, policy, cb, adapter, logger)
				if err != nil {
					cobra.CheckErr(fmt.Sprintf("while adding '%s' to collection - %v", itemURN, err))
					return
				}
				if !silent {
					fmt.Printf("Added '%s' to collection\n", itemURN)
				}
				added++
			}
			if !silent {
				fmt.Printf("Done: %d added, %d skipped (already member)\n", added, skipped)
			}
		},
	}

	collectionGetCmd = &cobra.Command{
		Use:     "get collectionURN",
		Short:   "Get a specific collection record",
		Aliases: []string{"g"},
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) (err error) {
			return getCollection(GetHistory(args[0]))
		},
	}

	// collectionRetractCmd fully retracts a collection: first all of its
	// collection-item aspects (paginated), then the collection definition aspect.
	collectionRetractCmd = &cobra.Command{
		Use:     "retract collectionURN",
		Aliases: []string{"x"},
		Short:   "Fully retract a collection and all its item memberships",
		Long: `Fully retract a collection from the DataFabric.

All collection-item aspects (` + CollectionItemSchema + `) for the
collection are retracted first, then the collection definition aspect
(` + CollectionSchema + `) itself is retracted.

This operation cannot be undone.`,
		Args: cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			collectionID := GetHistory(args[0])
			if !URN_CHECK.Match([]byte(collectionID)) {
				cobra.CheckErr(fmt.Sprintf("'%s' is not a URN", collectionID))
				return
			}

			adapter := CreateAdapter(true)
			ctxt := context.Background()

			// 1. Retract all collection-item aspects (paginated).
			retracted, err := retractAllCollectionItems(ctxt, collectionID, adapter)
			if err != nil {
				cobra.CheckErr(fmt.Sprintf("while retracting collection items - %v", err))
				return
			}
			if !silent {
				fmt.Printf("Retracted %d collection-item record(s)\n", retracted)
			}

			// 2. Find and retract the collection definition aspect.
			selector := sdk.AspectSelector{
				Entity:       collectionID,
				SchemaPrefix: CollectionSchema,
				ListRequest:  sdk.ListRequest{Limit: 2},
			}
			list, _, err := sdk.ListAspect(ctxt, selector, adapter, logger)
			if err != nil {
				cobra.CheckErr(fmt.Sprintf("while looking up collection definition - %v", err))
				return
			}
			if len(list.Items) == 0 {
				if !silent {
					fmt.Println("No collection definition aspect found; nothing more to retract")
				}
				return
			}
			collAspectID := *list.Items[0].ID
			if _, err = sdk.RetractAspect(ctxt, collAspectID, adapter, logger); err != nil {
				cobra.CheckErr(fmt.Sprintf("while retracting collection definition - %v", err))
				return
			}
			if !silent {
				fmt.Printf("Retracted collection definition '%s'\n", collectionID)
			}
		},
	}

	// collectionRemoveCmd removes items from a collection by retracting their
	// collection-item aspects.
	collectionRemoveCmd = &cobra.Command{
		Use:     "remove collectionURN urn [urn...]",
		Aliases: []string{"rm", "delete"},
		Short:   "Remove item(s) from a collection",
		Long: `Remove one or more items from an existing collection.

For each item URN provided, the corresponding collection-item aspect
(` + CollectionItemSchema + `) is retracted from the DataFabric.

Items that are not currently members of the collection are silently skipped.`,
		Args: cobra.MinimumNArgs(2),
		Run: func(cmd *cobra.Command, args []string) {
			collectionID := GetHistory(args[0])
			if !URN_CHECK.Match([]byte(collectionID)) {
				cobra.CheckErr(fmt.Sprintf("'%s' is not a URN", collectionID))
				return
			}

			adapter := CreateAdapter(true)
			ctxt := context.Background()

			removed := 0
			skipped := 0
			for _, arg := range args[1:] {
				itemURN := GetHistory(arg)
				if !URN_CHECK.Match([]byte(itemURN)) {
					cobra.CheckErr(fmt.Sprintf("'%s' is not a URN", itemURN))
					return
				}
				// Find the collection-item aspect for this item
				jf := fmt.Sprintf(`$.item ? (@ == "%s")`, itemURN)
				selector := sdk.AspectSelector{
					Entity:       collectionID,
					SchemaPrefix: CollectionItemSchema,
					JsonFilter:   &jf,
					ListRequest:  sdk.ListRequest{Limit: 1},
				}
				list, _, err := sdk.ListAspect(ctxt, selector, adapter, logger)
				if err != nil {
					cobra.CheckErr(fmt.Sprintf("while searching for '%s' in collection - %v", itemURN, err))
					return
				}
				if len(list.Items) == 0 {
					if !silent {
						fmt.Printf("Skipping '%s': not a member of collection\n", itemURN)
					}
					skipped++
					continue
				}
				aspectID := *list.Items[0].ID
				_, err = sdk.RetractAspect(ctxt, aspectID, adapter, logger)
				if err != nil {
					cobra.CheckErr(fmt.Sprintf("while removing '%s' from collection - %v", itemURN, err))
					return
				}
				if !silent {
					fmt.Printf("Removed '%s' from collection\n", itemURN)
				}
				removed++
			}
			if !silent {
				fmt.Printf("Done: %d removed, %d skipped (not a member)\n", removed, skipped)
			}
		},
	}
)

// addItemToCollection creates a collection-item aspect linking itemURN to
// collectionURN, unless the membership already exists (idempotent).
// It is called both by the 'collection add' command and by 'artifact create
// --collection' so that both paths produce identical DataFabric records.
func addItemToCollection(ctxt context.Context, collectionURN, itemURN string, adpt *a.Adapter) error {
	exists, err := isAlreadyCollectionItem(ctxt, collectionURN, itemURN, adpt)
	if err != nil {
		return fmt.Errorf("checking collection membership: %w", err)
	}
	if exists {
		return nil // already a member, nothing to do
	}
	ci := CollectionItem{Collection: collectionURN, Item: itemURN}
	cb, err := json.Marshal(ci)
	if err != nil {
		return fmt.Errorf("marshalling collection-item: %w", err)
	}
	_, err = sdk.AddUpdateAspect(ctxt, true, collectionURN, CollectionItemSchema, policy, cb, adpt, logger)
	return err
}

// isAlreadyCollectionItem returns true when a collection-item aspect already
// exists for the given collection / item pair.  It uses a server-side
// JSONPath filter to minimise data transfer.
func isAlreadyCollectionItem(
	ctxt context.Context,
	collectionURN string,
	itemURN string,
	adpt *a.Adapter,
) (bool, error) {
	jf := fmt.Sprintf(`$.item ? (@ == "%s")`, itemURN)
	selector := sdk.AspectSelector{
		Entity:       collectionURN,
		SchemaPrefix: CollectionItemSchema,
		JsonFilter:   &jf,
		ListRequest:  sdk.ListRequest{Limit: 1},
	}
	list, _, err := sdk.ListAspect(ctxt, selector, adpt, logger)
	if err != nil {
		return false, err
	}
	return len(list.Items) > 0, nil
}

// resolveCollectionFiles resolves a --dir argument into a list of file paths.
// If the argument is a directory, all non-hidden, non-directory entries are
// returned.  Otherwise it is treated as a glob pattern.
func resolveCollectionFiles(dirOrGlob string) ([]string, error) {
	info, err := os.Stat(dirOrGlob)
	if err == nil && info.IsDir() {
		entries, err := os.ReadDir(dirOrGlob)
		if err != nil {
			return nil, err
		}
		var files []string
		for _, e := range entries {
			if strings.HasPrefix(e.Name(), ".") || e.IsDir() {
				continue
			}
			files = append(files, filepath.Join(dirOrGlob, e.Name()))
		}
		return files, nil
	}
	// Fall through to glob
	matches, err := filepath.Glob(dirOrGlob)
	if err != nil {
		return nil, fmt.Errorf("invalid glob pattern: %w", err)
	}
	var files []string
	for _, m := range matches {
		fi, err := os.Stat(m)
		if err != nil || fi.IsDir() || strings.HasPrefix(filepath.Base(m), ".") {
			continue
		}
		files = append(files, m)
	}
	return files, nil
}

// collectionItemDisplay holds display info for a single collection member.
type collectionItemDisplay struct {
	URN  string // item URN
	Name string // non-empty only when the artifact has a user-visible name
}

// fetchCollectionItems queries the collection-item aspects for collectionID,
// then — for artifact items — resolves a human-readable name from the artifact
// record (skipped when the name was left as the artifact URN default).
func fetchCollectionItems(ctxt context.Context, collectionID string, adpt *a.Adapter) ([]collectionItemDisplay, bool, error) {
	selector := sdk.AspectSelector{
		Entity:         collectionID,
		SchemaPrefix:   CollectionItemSchema,
		IncludeContent: true,
		ListRequest:    sdk.ListRequest{Limit: maxCollectionItems},
	}
	itemList, _, err := sdk.ListAspect(ctxt, selector, adpt, logger)
	if err != nil {
		return nil, false, err
	}

	var items []collectionItemDisplay
	for _, aspect := range itemList.Items {
		cm, ok := aspect.Content.(map[string]any)
		if !ok {
			continue
		}
		itemURN, _ := cm["item"].(string)
		if itemURN == "" {
			continue
		}
		disp := collectionItemDisplay{URN: itemURN}
		// For artifact URNs, try to fetch a meaningful name.
		if strings.HasPrefix(itemURN, "urn:ivcap:artifact:") {
			req := &sdk.ReadArtifactRequest{Id: itemURN}
			if art, aerr := sdk.ReadArtifact(ctxt, req, adpt, logger); aerr == nil {
				if art.Name != nil && *art.Name != "" && *art.Name != itemURN {
					disp.Name = *art.Name
				}
			}
		}
		items = append(items, disp)
	}

	// Whether there are more items than maxCollectionItems
	hasMore := len(itemList.Items) == maxCollectionItems && itemList.Links != nil
	for _, l := range itemList.Links {
		if l.Rel != nil && *l.Rel == "next" {
			hasMore = true
			break
		}
	}
	return items, hasMore, nil
}

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
		return
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
			items, hasMore, _ := fetchCollectionItems(ctxt, collectionID, adapter)
			printCollection(res, items, hasMore)
			return nil
		} else {
			return err
		}
	}
}

func printCollection(res *api.ReadResponseBody, items []collectionItemDisplay, hasMore bool) {
	if res.ContentType == nil || *res.ContentType != "application/json" {
		cobra.CheckErr("Cannot find collection definition in reply")
		return
	}
	var cm map[string]any
	var ok bool
	if cm, ok = res.Content.(map[string]any); !ok {
		cobra.CheckErr("Unexpected content type")
		return
	}

	collName, _ := cm["name"].(string)
	collDesc, _ := cm["description"].(string)

	p := []table.Row{
		{"Entity", fmt.Sprintf("%s (%s)", *res.Entity, MakeHistory(res.Entity))},
		{"Name", collName},
	}
	if collDesc != "" {
		p = append(p, table.Row{"Description", collDesc})
	}
	p = append(p, table.Row{"Asserter", safeString(res.Asserter)})
	if res.ValidTo == nil {
		p = append(p, table.Row{"LastUpdated", safeDate(res.ValidFrom, true)})
	} else {
		p = append(p,
			table.Row{"ValidFrom", safeDate(res.ValidFrom, true)},
			table.Row{"Retracter", safeString(res.Retracter)},
			table.Row{"ValidTo", safeDate(res.ValidTo, true)},
		)
	}

	// Items sub-table
	if len(items) > 0 {
		twi := table.NewWriter()
		twi.SetStyle(table.StyleLight)
		twi.AppendHeader(table.Row{"Item", "Name"})
		for _, item := range items {
			twi.AppendRow(table.Row{
				fmt.Sprintf("(%s) %s", MakeHistory(&item.URN), item.URN),
				item.Name,
			})
		}
		label := fmt.Sprintf("Items (%d)", len(items))
		if hasMore {
			label = fmt.Sprintf("Items (%d+, use --max-items to see more)", len(items))
		}
		p = append(p, table.Row{label, twi.Render()})
	} else {
		p = append(p, table.Row{"Items", "(none)"})
	}

	tw := table.NewWriter()
	tw.SetStyle(table.StyleLight)
	tw.Style().Options.SeparateColumns = false
	tw.Style().Options.SeparateRows = false
	tw.Style().Options.DrawBorder = false
	tw.SetColumnConfigs([]table.ColumnConfig{
		{Number: 1, Align: text.AlignRight},
	})
	tw.AppendRows(p)
	fmt.Printf("\n%s\n\n", tw.Render())
}

func printCollectionTable(list *api.ListResponseBody, wide bool) {
	tw2 := table.NewWriter()
	tw2.AppendHeader(table.Row{"ID", "Name", "Last Updated"})
	tw2.SetStyle(table.StyleLight)
	rows := make([]table.Row, len(list.Items))
	for i, p := range list.Items {
		collName := ""
		if cm, ok := p.Content.(map[string]any); ok {
			collName, _ = cm["name"].(string)
		}
		rows[i] = table.Row{
			fmt.Sprintf("(%s) %s", MakeHistory(p.Entity), *p.Entity),
			collName,
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

// retractAllCollectionItems pages through every collection-item aspect for
// collectionID and retracts each one. Returns the total number retracted.
func retractAllCollectionItems(ctxt context.Context, collectionID string, adpt *a.Adapter) (int, error) {
	const pageSize = 100
	var page *string
	total := 0
	for {
		selector := sdk.AspectSelector{
			Entity:       collectionID,
			SchemaPrefix: CollectionItemSchema,
			ListRequest:  sdk.ListRequest{Limit: pageSize, Page: page},
		}
		list, _, err := sdk.ListAspect(ctxt, selector, adpt, logger)
		if err != nil {
			return total, err
		}
		for _, item := range list.Items {
			if item.ID == nil {
				continue
			}
			if _, err := sdk.RetractAspect(ctxt, *item.ID, adpt, logger); err != nil {
				return total, fmt.Errorf("retracting aspect %s: %w", *item.ID, err)
			}
			total++
		}
		// Advance to the next page, or stop if there are no more.
		page = pageTokenFromHref(findNextCollectionPage(list.Links))
		if page == nil {
			break
		}
	}
	return total, nil
}

// pageTokenFromHref extracts the 'page' query parameter from a full href URL
// returned by the DataFabric API in link headers.
func pageTokenFromHref(href *string) *string {
	if href == nil {
		return nil
	}
	u, err := url.Parse(*href)
	if err != nil {
		return nil
	}
	tok := u.Query().Get("page")
	if tok == "" {
		return nil
	}
	return &tok
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
