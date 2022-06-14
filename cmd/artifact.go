package cmd

import (
	"bufio"
	api "cayp/api_gateway/gen/http/artifact/client"
	"context"
	"fmt"
	"io"
	"io/fs"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"

	sdk "github.com/reinventingscience/ivcap-client/pkg"
	a "github.com/reinventingscience/ivcap-client/pkg/adapter"

	"github.com/jedib0t/go-pretty/v6/table"
	"github.com/jedib0t/go-pretty/v6/text"
	"github.com/spf13/cobra"
	log "go.uber.org/zap"
)

var (
	artifactName       string
	artifactCollection string
	outputFile         string
	inputFile          string
	contentType        string

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
			req := &sdk.ListArtifactRequest{Offset: 0, Limit: 50}
			if offset > 0 {
				req.Offset = offset
			}
			if limit > 0 {
				req.Limit = limit
			}
			if res, err := sdk.ListArtifactsRaw(context.Background(), req, CreateAdapter(true), logger); err == nil {
				switch format {
				case "json":
					a.ReplyPrinter(res, false)
				case "yaml":
					a.ReplyPrinter(res, true)
				default:
					var list api.ListResponseBody
					res.AsType(&list)
					printArtifactTable(&list, false)
				}
				return nil
			} else {
				return err
			}
		},
	}

	readArtifactCmd = &cobra.Command{
		Use:     "read [flags] artifact_id",
		Aliases: []string{"get"},
		Short:   "Fetch details about a single artifact",
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			recordID := args[0]
			req := &sdk.ReadArtifactRequest{Id: recordID}

			switch format {
			case "json", "yaml":
				if res, err := sdk.ReadArtifactRaw(context.Background(), req, CreateAdapter(true), logger); err == nil {
					a.ReplyPrinter(res, format == "yaml")
				} else {
					return err
				}
			default:
				if artifact, err := sdk.ReadArtifact(context.Background(), req, CreateAdapter(true), logger); err == nil {
					printArtifact(artifact, false)
				} else {
					return err
				}
			}
			return nil
		},
	}

	downloadArtifactCmd = &cobra.Command{
		Use:   "download [flags] artifact_id [-o file|-]",
		Short: "Download the content associated with this artifact",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			recordID := args[0]
			req := &sdk.ReadArtifactRequest{Id: recordID}
			adapter := CreateAdapter(true)
			artifact, err := sdk.ReadArtifact(context.Background(), req, adapter, logger)
			if err != nil {
				return err
			}
			data := artifact.Data
			if data == nil || data.Self == nil {
				cobra.CheckErr("No data available")
			}
			url, err := url.ParseRequestURI(*data.Self)
			if err != nil {
				return err
			}
			pyld, err := (*adapter).Get(context.Background(), url.Path, logger)
			if err != nil {
				return err
			}
			content := pyld.AsBytes()
			if outputFile == "" || outputFile == "-" {
				os.Stdout.Write(content)
			} else {
				if err := ioutil.WriteFile(outputFile, content, fs.FileMode(0644)); err != nil {
					cobra.CheckErr(fmt.Sprintf("while writing data to file '%s' - %v", outputFile, err))
				}
				fmt.Printf("Successfully wrote %d bytes to %s\n", len(content), outputFile)
			}
			return nil
		},
	}

	createArtifactCmd = &cobra.Command{
		Use:   "create [key=value key=value] -f file|-",
		Short: "Create a new artifact",

		Run: func(cmd *cobra.Command, args []string) {
			var reader io.Reader
			reader, contentType = getReader(inputFile, contentType)
			logger.Debug("create artifact", log.String("content-type", contentType), log.String("inputFile", inputFile))
			adapter := CreateAdapter(true)
			req := &sdk.CreateArtifactRequest{
				Name:       artifactName,
				Collection: artifactCollection,
			}
			if resp, err := sdk.CreateArtifact(context.Background(), req, contentType, reader, adapter, logger); err == nil {
				printUploadArtifactResponse(resp, false)
			} else {
				cobra.CompErrorln(fmt.Sprintf("while uploading data file '%s' - %v", inputFile, err))
			}
		},
	}
)

func init() {
	rootCmd.AddCommand(artifactCmd)

	artifactCmd.AddCommand(listArtifactCmd)
	listArtifactCmd.Flags().IntVar(&offset, "offset", -1, "record offset into returned list")
	listArtifactCmd.Flags().IntVar(&limit, "limit", -1, "max number of records to be returned")
	listArtifactCmd.Flags().StringVarP(&format, "output", "o", "short", "format to use for list (short, yaml, json)")

	artifactCmd.AddCommand(readArtifactCmd)
	//readArtifactCmd.Flags().StringVarP(&recordID, "artifact-id", "i", "", "ID of artifact to retrieve")
	readArtifactCmd.Flags().StringVarP(&format, "output", "o", "short", "format to use for list (short, yaml, json)")

	artifactCmd.AddCommand(downloadArtifactCmd)
	downloadArtifactCmd.Flags().StringVarP(&outputFile, "output", "o", "", "File to write content to [stdout]")

	artifactCmd.AddCommand(createArtifactCmd)
	createArtifactCmd.Flags().StringVarP(&artifactName, "name", "n", "", "Human friendly name")
	createArtifactCmd.Flags().StringVarP(&artifactCollection, "collection", "c", "", "Assigns artifact to a specific collection")
	createArtifactCmd.Flags().StringVarP(&inputFile, "file", "f", "", "Path to file containing artifact content")
	createArtifactCmd.Flags().StringVarP(&contentType, "content-type", "t", "", "Content type of artifact")
}

func printArtifactTable(list *api.ListResponseBody, wide bool) {
	t := table.NewWriter()
	t.SetOutputMirror(os.Stdout)
	t.AppendHeader(table.Row{"ID", "Name", "Status"})
	rows := make([]table.Row, len(list.Artifacts))
	for i, o := range list.Artifacts {
		rows[i] = table.Row{*o.ID, safeTruncString(o.Name), safeString(o.Status)}
	}
	t.AppendRows(rows)
	t.Render()
}

func printArtifact(artifact *api.ReadResponseBody, wide bool) {
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
		{"ID", *artifact.ID},
		{"Name", safeString(artifact.Name)},
		{"Status", safeString(artifact.Status)},
		{"Size", safeNumber(artifact.Size)},
		{"Mime-type", safeString(artifact.MimeType)},
		{"Account ID", safeString(artifact.Account.ID)},
	})
	//fmt.Printf("META: %v\n", artifact.Metadata)
	fmt.Printf("\n%s\n\n", tw.Render())
}

func printUploadArtifactResponse(artifact *api.UploadResponseBody, wide bool) {
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
		{"ID", *artifact.ID},
		{"Name", safeString(artifact.Name)},
		{"Status", safeString(artifact.Status)},
		{"Size", safeNumber(artifact.Size)},
		{"Account ID", safeString(artifact.Account.ID)},
	})
	fmt.Printf("\n%s\n\n", tw.Render())
}

func getReader(fileName string, proposedFormat string) (reader io.Reader, format string) {
	if fileName == "" {
		cobra.CheckErr("Missing file name '-f'")
	}
	format = proposedFormat
	var file *os.File
	var err error
	if fileName == "-" {
		file = os.Stdin
	} else {
		if file, err = os.Open(fileName); err != nil {
			cobra.CheckErr(fmt.Sprintf("while opening data file '%s' - %v", fileName, err))
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
	_, err = file.Seek(0, 0)
	return
}
