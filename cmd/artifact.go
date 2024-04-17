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
	"bufio"
	"context"
	"fmt"
	"io"
	"path/filepath"

	api "github.com/ivcap-works/ivcap-core-api/http/artifact"

	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"

	sdk "github.com/ivcap-works/ivcap-cli/pkg"
	a "github.com/ivcap-works/ivcap-cli/pkg/adapter"
	asapi "github.com/ivcap-works/ivcap-core-api/http/aspect"

	"github.com/jedib0t/go-pretty/v6/table"
	"github.com/jedib0t/go-pretty/v6/text"
	"github.com/spf13/cobra"
	log "go.uber.org/zap"
)

func init() {
	rootCmd.AddCommand(artifactCmd)

	// LIST
	artifactCmd.AddCommand(listArtifactCmd)
	addListFlags(listArtifactCmd)

	// READ
	artifactCmd.AddCommand(readArtifactCmd)

	// DOWNLOAD
	artifactCmd.AddCommand(downloadArtifactCmd)
	downloadArtifactCmd.Flags().StringVarP(&fileName, "file", "f", "", "File to write content to [stdout]")

	// CREATE
	artifactCmd.AddCommand(createArtifactCmd)
	addFlags(createArtifactCmd, []Flag{Name, Policy})
	createArtifactCmd.Flags().StringVarP(&artifactCollection, "collection", "c", "", "Assigns artifact to a specific collection")
	createArtifactCmd.Flags().StringVarP(&fileName, "file", "f", "", "Path to file containing artifact content")
	createArtifactCmd.Flags().StringVarP(&contentType, "content-type", "t", "", "Content type of artifact")
	createArtifactCmd.Flags().Int64Var(&chunkSize, "chunk-size", DEF_CHUNK_SIZE, "Chunk size for splitting large files")
	createArtifactCmd.Flags().BoolVar(&force, "force", false, "Force creation of new artifact, even if already uploaded")

	// UPLOAD
	artifactCmd.AddCommand(uploadArtifactCmd)
	uploadArtifactCmd.Flags().StringVarP(&fileName, "file", "f", "", "Path to file containing artifact content")
	uploadArtifactCmd.Flags().StringVarP(&contentType, "content-type", "t", "", "Content type of artifact")
	uploadArtifactCmd.Flags().Int64Var(&chunkSize, "chunk-size", DEF_CHUNK_SIZE, "Chunk size for splitting large files")

	// // ADD METADATA
	// artifactCmd.AddCommand(addArtifactMetadataCmd)
	// addArtifactMetadataCmd.Flags().StringVarP(&metaFile, "file", "f", "", "Path to file containing metadata")

	// // REMOVE
	// artifactCmd.AddCommand(removeArtifactMetadataCmd)

	// // COLLECTION
	// artifactCmd.AddCommand(addArtifactToCollectionCmd)
	// // artifactCmd.AddCommand(removeArtifactFromCollectionCmd)
}

const DEF_CHUNK_SIZE = 10000000 // -1 ... no chunking

type ArtifactPostResponse struct {
	// Artifact ID
	ID string `form:"id" json:"id" xml:"id"`
	// Optional name
	Name string `form:"name,omitempty" json:"name,omitempty" xml:"name,omitempty"`
	// Artifact status
	Status string `form:"status" json:"status" xml:"status"`
	// Mime-type of data
	MimeType string `form:"mime-type,omitempty" json:"mime-type,omitempty" xml:"mime-type,omitempty"`
	// Size of data
	Size int64 `form:"size,omitempty" json:"size,omitempty" xml:"size,omitempty"`
}

const ArtifactInCollectionSchema = "urn:common:schema:in_collection.1"

type ArtifactInCollection struct {
	ArtifactID   string `json:"artifact"`
	CollectionID string `json:"collection"`
}

var (
	artifactCollection string
	contentType        string
	chunkSize          int64
	force              bool

	artifactCmd = &cobra.Command{
		Use:     "artifact",
		Short:   "Create and manage artifacts ",
		Aliases: []string{"a", "artifacts"},
		// 	Long: `A longer description that spans multiple lines and likely contains examples
		// and usage of using your command. For example:
	}

	listArtifactCmd = &cobra.Command{
		Use:   "list",
		Short: "List existing artifacts",

		RunE: func(cmd *cobra.Command, args []string) error {
			req := createListRequest()
			if res, err := sdk.ListArtifactsRaw(context.Background(), req, CreateAdapter(true), logger); err == nil {
				switch outputFormat {
				case "json":
					return a.ReplyPrinter(res, false)
				case "yaml":
					return a.ReplyPrinter(res, true)
				default:
					var list api.ListResponseBody
					if err = res.AsType(&list); err != nil {
						return fmt.Errorf("failed to parse response body: %w", err)
					}
					printArtifactTable(&list, false)
				}
				return nil
			} else {
				return err
			}
		},
	}

	readArtifactCmd = &cobra.Command{
		Use:     "get [flags] artifact_id",
		Aliases: []string{"read"},
		Short:   "Fetch details about a single artifact",
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			recordID := GetHistory(args[0])
			req := &sdk.ReadArtifactRequest{Id: recordID}
			adapter := CreateAdapter(true)

			switch outputFormat {
			case "json", "yaml":
				res, err := sdk.ReadArtifactRaw(context.Background(), req, adapter, logger)
				if err != nil {
					return err
				}
				return a.ReplyPrinter(res, outputFormat == "yaml")
			default:
				if artifact, err := sdk.ReadArtifact(context.Background(), req, adapter, logger); err == nil {
					selector := sdk.AspectSelector{Entity: recordID}
					if meta, _, err := sdk.ListAspect(context.Background(), selector, adapter, logger); err == nil {
						printArtifact(artifact, meta, false)
					} else {
						return err
					}
				} else {
					return err
				}
			}
			return nil
		},
	}

	downloadArtifactCmd = &cobra.Command{
		Use:   "download artifact_id [flags] [-f file|-]",
		Short: "Download the content associated with this artifact",
		Args:  cobra.ExactArgs(1),
		RunE:  downloadArtifact,
	}

	createArtifactCmd = &cobra.Command{
		Use:   "create [flags] -n name -f file|-",
		Short: "Create a new artifact",

		Run: func(cmd *cobra.Command, args []string) {
			uploadArtifact(fileName, force, artifactCollection)
		},
	}

	uploadArtifactCmd = &cobra.Command{
		Use:     "upload artifactID -f file|-",
		Short:   "Resume uploading artifact content",
		Aliases: []string{"resume"},
		Args:    cobra.ExactArgs(1),

		Run: func(cmd *cobra.Command, args []string) {
			artifactID := args[0]
			reader, contentType, size := getReader(fileName, contentType)
			logger.Debug("upload artifact", log.String("content-type", contentType), log.String("file", fileName))
			adapter := CreateAdapter(true)
			ctxt := context.Background()

			offset := int64(0)

			rreq := &sdk.ReadArtifactRequest{Id: artifactID}
			readResp, err := sdk.ReadArtifact(ctxt, rreq, adapter, logger)
			if err != nil {
				cobra.CompErrorln(fmt.Sprintf("while getting a status update on '%s' - %v", artifactID, err))
				return
			}
			path, err := (*adapter).GetPath(*readResp.DataHref)
			if err != nil {
				cobra.CompErrorln(fmt.Sprintf("while parsing API reply - %v", err))
				return
			}

			headers := map[string]string{
				"Tus-Resumable": "1.0.0",
			}
			pyld, err := (*adapter).Head(ctxt, path, &headers, logger)
			if err != nil {
				cobra.CompErrorln(fmt.Sprintf("while checking on upload status of artifact '%s' - %v", artifactID, err))
				return
			}
			offset, err = strconv.ParseInt(pyld.Header("Upload-Offset"), 10, 64)
			if err != nil {
				cobra.CompErrorln(fmt.Sprintf("problems parsing 'Upload-Offset' in return header '%s' - %v", pyld.Header("Upload-Offset"), err))
				return
			}

			if size > 0 && offset >= size {
				// already done
				cobra.CompErrorln(fmt.Sprintf("Artifact '%s' already fully uploaded\n", artifactID))
				return
			}

			if err = upload(ctxt, reader, artifactID, path, size, offset, adapter); err != nil {
				cobra.CompErrorln(fmt.Sprintf("while uploading artifact '%s' - %v", artifactID, err))
			}
		},
	}

	// NOT SURE HOW WE BEST HANDLE THAT
	//
	// addArtifactToCollectionCmd = &cobra.Command{
	// 	Use:     "add-to-collection artifactID collectionName",
	// 	Short:   "Add artifact to a collection",
	// 	Aliases: []string{"add-collection"},
	// 	Args:    cobra.ExactArgs(2),

	// 	Run: func(cmd *cobra.Command, args []string) {
	// 		artifactID := args[0]
	// 		collectionID := args[1]
	// 		logger.Debug("add collection", log.String("artifactID", artifactID), log.String("collectionID", collectionID))
	// 		meta := ArtifactInCollection{ArtifactID: artifactID, CollectionID: collectionID}
	// 		data, err := json.Marshal(meta)
	// 		if err != nil {
	// 			cobra.CompErrorln(fmt.Sprintf("while serialise metadata - %v", err))

	// 		}
	// 		adapter := CreateAdapter(true)
	// 		ctxt := context.Background()
	// 		if res, err := sdk.AddUpdateAspect(ctxt, true, collectionID, ArtifactInCollectionSchema, policy, data, adapter, logger); err == nil {
	// 			if silent {
	// 				if m, err := res.AsObject(); err == nil {
	// 					fmt.Printf("%s\n", m["record-id"])
	// 				} else {
	// 					cobra.CheckErr(fmt.Sprintf("Parsing reply: %s", res.AsBytes()))
	// 				}
	// 			} else {
	// 				if err = a.ReplyPrinter(res, outputFormat == "yaml"); err != nil {
	// 					cobra.CheckErr("print reply")
	// 					return
	// 				}
	// 			}
	// 		} else {
	// 			cobra.CompErrorln(fmt.Sprintf("while adding metadata '%s' to artifact '%s' - %v", ArtifactInCollectionSchema, artifactID, err))
	// 			return
	// 		}
	// 	},
	// }

	// removeArtifactFromCollectionCmd = &cobra.Command{
	// 	Use:     "remove-from-collection artifactID collectionName",
	// 	Short:   "Remove artifact from a collection",
	// 	Aliases: []string{"remove-collection", "rm-collection"},
	// 	Args:    cobra.ExactArgs(2),

	// 	Run: func(cmd *cobra.Command, args []string) {
	// 		artifactID := args[0]
	// 		collectionName := args[1]
	// 		logger.Debug("rm collection", log.String("artifactID", artifactID), log.String("collectionName", collectionName))
	// 		adapter := CreateAdapter(true)
	// 		ctxt := context.Background()
	// 		_, err := sdk.RemoveArtifactToCollection(ctxt, artifactID, collectionName, adapter, logger)
	// 		if err != nil {
	// 			cobra.CompErrorln(fmt.Sprintf("while removing artifact '%s' from collection(s) '%s' - %v", artifactID, collectionName, err))
	// 			return
	// 		}
	// 	},
	// }

// 	addArtifactMetadataCmd = &cobra.Command{
// 		Use:     "add-metadata artifactID schemaName -f meta.json",
// 		Short:   "Add artifact to a comma separated list of collections",
// 		Aliases: []string{"add-meta"},
// 		Args:    cobra.ExactArgs(2),

// 		Run: func(cmd *cobra.Command, args []string) {
// 			artifactID := args[0]
// 			schemaName := args[1]
// 			logger.Debug("add meta", log.String("artifactID", artifactID), log.String("schemaName", schemaName),
// 				log.String("metaFile", metaFile))
// 			reader, _, size := getReader(metaFile, "application/json")

// 			adapter := CreateAdapter(true)
// 			ctxt := context.Background()
// 			_, err := sdk.AddArtifactMeta(ctxt, artifactID, schemaName, reader, size, adapter, logger)
// 			if err != nil {
// 				cobra.CompErrorln(fmt.Sprintf("while adding metadata '%s' to artifact '%s' - %v", schemaName, artifactID, err))
// 				return
// 			}
// 		},
// 	}

// 	removeArtifactMetadataCmd = &cobra.Command{
// 		Use:     "remove-metadata artifactID schemaName",
// 		Short:   "Remove artifact from a comma separated list of collections",
// 		Aliases: []string{"remove-collection", "rm-collection"},
// 		Args:    cobra.ExactArgs(2),

//		Run: func(cmd *cobra.Command, args []string) {
//			artifactID := args[0]
//			collectionName := args[1]
//			logger.Debug("rm collection", log.String("artifactID", artifactID), log.String("collectionName", collectionName))
//			adapter := CreateAdapter(true)
//			ctxt := context.Background()
//			_, err := sdk.RemoveArtifactToCollection(ctxt, artifactID, collectionName, adapter, logger)
//			if err != nil {
//				cobra.CompErrorln(fmt.Sprintf("while removing artifact '%s' from collection(s) '%s' - %v", artifactID, collectionName, err))
//				return
//			}
//		},
//	}
)

func uploadArtifact(
	fileName string,
	force bool,
	artifactCollection string,
) (artifactID string) {
	var reader io.Reader
	var size int64
	metaFile, metaExists := getArtifactMetaFileFor(fileName)
	if !force && metaFile != nil && metaExists {
		artifactID = getArtifactIdFromMeta(*metaFile)
		msg := fmt.Sprintf("File '%s' already uploaded as '%s (%s)'. Use '--force' to create a new artifact",
			fileName, artifactID, MakeHistory(&artifactID))
		cobra.CheckErr(msg)
		return
	}
	reader, contentType, size = getReader(fileName, contentType)
	logger.Debug("create artifact", log.String("content-type", contentType), log.String("file", fileName))
	if name == "" && fileName != "-" {
		name = filepath.Base(fileName)
	}
	adapter := CreateAdapterWithTimeout(true, timeout)
	req := &sdk.CreateArtifactRequest{
		Name:       name,
		Size:       size,
		Collection: artifactCollection,
		Policy:     policy,
	}
	ctxt := context.Background()
	resp, err := sdk.CreateArtifact(ctxt, req, contentType, size, nil, adapter, logger)
	if err != nil {
		cobra.CheckErr(fmt.Sprintf("while creating record for '%s'- %v", fileName, err))
		return
	}
	artifactID = *resp.ID
	if !silent {
		fmt.Printf("Created artifact '%s'\n", artifactID)
	}
	path, err := (*adapter).GetPath(*resp.DataHref)
	if err != nil {
		cobra.CheckErr(fmt.Sprintf("while parsing API reply - %v", err))
		return
	}
	if err = upload(ctxt, reader, artifactID, path, size, 0, adapter); err != nil {
		cobra.CheckErr(fmt.Sprintf("while upload - %v", err))
		return
	}
	if silent {
		// print artifact ID anyway
		fmt.Printf("%s\n", artifactID)
	}
	if metaFile != nil {
		err = os.WriteFile(*metaFile, []byte(artifactID), 0644) // #nosec G306 -- only includes the artifact ID
		if err != nil {
			cobra.CheckErr(fmt.Sprintf("saving information to metafile '%s' failed - %v", *metaFile, err))
		}
	}
	return
}

func upload(
	ctxt context.Context,
	reader io.Reader,
	artifactID string,
	path string,
	size int64,
	offset int64,
	adapter *a.Adapter,
) (err error) {
	if err = sdk.UploadArtifact(ctxt, reader, size, offset, chunkSize, path, adapter, silent, logger); err != nil {
		cobra.CompErrorln(fmt.Sprintf("while uploading data file '%s' - %v", fileName, err))
		return
	}
	if silent {
		return
	}
	fmt.Printf("Completed uploading '%s'\n", artifactID)
	readReq := &sdk.ReadArtifactRequest{Id: artifactID}

	switch outputFormat {
	case "json", "yaml":
		res, err := sdk.ReadArtifactRaw(ctxt, readReq, adapter, logger)
		if err != nil {
			return err
		}
		return a.ReplyPrinter(res, outputFormat == "yaml")
	default:
		var readResp *api.ReadResponseBody
		if readResp, err = sdk.ReadArtifact(ctxt, readReq, adapter, logger); err == nil {
			printArtifact(readResp, nil, false)
		} else {
			cobra.CompErrorln(fmt.Sprintf("while getting a status update on '%s' - %v", artifactID, err))
			return
		}
	}
	return
}

func downloadArtifact(cmd *cobra.Command, args []string) error {
	recordID := GetHistory(args[0])
	req := &sdk.ReadArtifactRequest{Id: recordID}
	adapter := CreateAdapter(true)
	ctxt := context.Background()
	artifact, err := sdk.ReadArtifact(ctxt, req, adapter, logger)
	if err != nil {
		return err
	}
	data := artifact.DataHref
	if data == nil { // } || data.Self == nil {
		cobra.CheckErr("No data available")
		return nil // should never get here, but linter complaints otherwise
	}
	url, err := url.ParseRequestURI(*data)
	if err != nil {
		return err
	}

	downloadHandler := func(resp *http.Response, path string, logger *log.Logger) (err error) {
		if resp.StatusCode >= 300 {
			return a.ProcessErrorResponse(resp, path, nil, logger)
		}

		var outFile *os.File
		if fileName == "-" {
			outFile = os.Stdout
		} else {
			outFile, err = os.Create(filepath.Clean(fileName))
			if err != nil {
				return
			}
		}
		var reader io.Reader
		if silent {
			reader = resp.Body
		} else {
			reader = sdk.AddProgressBar("... downloading file", resp.ContentLength, resp.Body)
		}
		_, err = io.Copy(outFile, reader)
		return
	}

	err = (*adapter).GetWithHandler(ctxt, url.Path, nil, downloadHandler, logger)
	if err != nil {
		return err
	}
	fmt.Printf("\n") // To move past progress bar
	return nil
}

func printArtifactTable(list *api.ListResponseBody, wide bool) {
	t := table.NewWriter()
	t.SetOutputMirror(os.Stdout)
	t.AppendHeader(table.Row{"ID", "Name", "Status", "Size", "MimeType"})
	rows := make([]table.Row, len(list.Items))
	for i, o := range list.Items {
		rows[i] = table.Row{MakeHistory(o.ID), safeTruncString(o.Name), safeString(o.Status),
			safeBytes(o.Size), safeString(o.MimeType)}
	}
	rows = addNextPageRow(findNextArtifactPage(list.Links), rows)
	t.AppendRows(rows)
	t.Render()
}

func printArtifact(artifact *api.ReadResponseBody, meta *asapi.ListResponseBody, wide bool) {
	tw3 := table.NewWriter()
	tw3.SetStyle(table.StyleLight)
	if meta != nil {
		rows2 := make([]table.Row, len(meta.Items))
		for i, p := range meta.Items {
			rows2[i] = table.Row{MakeHistory(p.ID), safeString(p.Schema)}
		}
		tw3.AppendRows(rows2)
	}

	tw := table.NewWriter()
	tw.SetStyle(table.StyleLight)
	tw.Style().Options.SeparateColumns = false
	tw.Style().Options.SeparateRows = false
	tw.Style().Options.DrawBorder = false
	tw.SetColumnConfigs([]table.ColumnConfig{
		{Number: 1, Align: text.AlignRight},
		{Number: 2, WidthMax: 80},
	})
	tw.AppendRows([]table.Row{
		{"ID", fmt.Sprintf("%s (%s)", *artifact.ID, MakeHistory(artifact.ID))},
		{"Name", safeString(artifact.Name)},
		{"Status", safeString(artifact.Status)},
		{"Size", safeBytes(artifact.Size)},
		{"Mime-type", safeString(artifact.MimeType)},
		{"Account ID", safeString(artifact.Account)},
		{"Metadata", tw3.Render()},
	})
	fmt.Printf("\n%s\n\n", tw.Render())
}

func getReader(fileName string, proposedFormat string) (reader io.Reader, format string, size int64) {
	if fileName == "" {
		cobra.CheckErr("Missing file name '-f'")
	}
	format = proposedFormat
	var file *os.File
	var err error
	size = -1 // -1 indicates that we can't obtain size
	if fileName == "-" {
		file = os.Stdin
	} else {
		if file, err = os.Open(filepath.Clean(fileName)); err != nil {
			cobra.CheckErr(fmt.Sprintf("while opening data file '%s' - %v", fileName, err))
		}
		if info, err := file.Stat(); err == nil {
			size = info.Size()
		}
		if proposedFormat == "" {
			if format, err = getFileContentType(file); err != nil {
				cobra.CheckErr(fmt.Sprintf("while checking content type of file '%s' - %v", fileName, err))
			}
		}
	}
	if format == "" {
		cobra.CheckErr("Missing content type [-t]")
	}
	reader = bufio.NewReader(file)
	return
}

func getFileContentType(file *os.File) (contentType string, err error) {
	buf := make([]byte, 512)
	_, err = file.Read(buf)
	if err != nil {
		return
	}
	contentType = http.DetectContentType(buf)
	if contentType == "application/octet-stream" {
		// see if we can do better
		n := file.Name()
		if strings.HasSuffix(n, ".nc") {
			contentType = "application/netcdf"
		}
	}
	_, err = file.Seek(0, 0)
	return
}

func findNextArtifactPage(links []*api.LinkTResponseBody) *string {
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

func getArtifactMetaFileFor(fileName string) (fnp *string, fileExists bool) {
	if fileName == "-" {
		// from pipe, so don't know source
		return nil, false
	}
	var afn string
	var err error
	if afn, err = filepath.Abs(fileName); err != nil {
		cobra.CheckErr(fmt.Sprintf("Can't obtain absolute path of '%s' - %v", fileName, err))
	}
	fdir := filepath.Dir(afn)
	fn := filepath.Join(fdir, fmt.Sprintf(".%s", filepath.Base(afn)))
	if _, err = os.Stat(fn); err == nil {
		fileExists = true
	}
	return &fn, fileExists
}

func getArtifactIdFromMeta(fileName string) string {
	b, err := os.ReadFile(filepath.Clean(fileName))
	if err == nil {
		return string(b)
	} else {
		return ""
	}
}
