package cmd

import (
	"context"
	"fmt"
	"time"

	"github.com/araddon/dateparse"
	sdk "github.com/reinventingscience/ivcap-client/pkg"
	a "github.com/reinventingscience/ivcap-client/pkg/adapter"

	// "github.com/jedib0t/go-pretty/v6/table"
	// "github.com/jedib0t/go-pretty/v6/text"
	"github.com/spf13/cobra"
	log "go.uber.org/zap"
)

var schemaName string
var time_at string

var (
	metaCmd = &cobra.Command{
		Use:     "metadata",
		Aliases: []string{"m", "meta"},
		Short:   "Add/get/revoke metadata",
	}

	metaAddCmd = &cobra.Command{
		Use:     "add [flags] entity [-s schemaName] -f -|meta --format json|yaml",
		Short:   "Add metadata of a specific schema to an entiry",
		Aliases: []string{"a", "+"},
		Long:    `.....`,
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) (err error) {
			entity := args[0]
			pyld, err := payloadFromFile(metaFile, inputFormat)
			if err != nil {
				cobra.CheckErr(fmt.Sprintf("While reading metadata file '%s' - %s", metaFile, err))
			}

			meta, err := pyld.AsObject()
			if err != nil {
				cobra.CheckErr(fmt.Sprintf("Cannot parse meta file '%s' - %s", metaFile, err))
			}
			var schema string
			schema = schemaName
			if schema == "" {
				if s, ok := meta["$schema"]; ok {
					schema = fmt.Sprintf("%s", s)
				} else {
					cobra.CheckErr("Missing schema name")
				}
			}
			logger.Debug("add meta", log.String("entity", entity), log.String("schema", schema), log.Reflect("pyld", meta))
			ctxt := context.Background()
			if res, err := sdk.AddMetadata(ctxt, entity, schema, pyld.AsBytes(), CreateAdapter(true), logger); err == nil {
				a.ReplyPrinter(res, outputFormat == "yaml")
			} else {
				return err
			}
			return nil
		},
	}

	metaGetCmd = &cobra.Command{
		Use:     "get [flags] entity [-s schemaName1,schemaName2]",
		Short:   "Get the metadata attached to an entity, optionally restricted to a list of schemas",
		Aliases: []string{"a", "+"},
		Long:    `.....`,
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) (err error) {
			entity := args[0]
			var ts *time.Time
			if time_at != "" {
				t, err := dateparse.ParseLocal(time_at)
				if err != nil {
					cobra.CheckErr(fmt.Sprintf("Can't parse '%s' into a date - %s", time_at, err))
				}
				ts = &t
			}
			ctxt := context.Background()
			if res, err := sdk.GetMetadata(ctxt, entity, schemaName, ts, CreateAdapter(true), logger); err == nil {
				a.ReplyPrinter(res, outputFormat == "yaml")
			} else {
				return err
			}
			return nil
		},
	}

	metaRevokeCmd = &cobra.Command{
		Use:     "revoke [flags] record-id",
		Short:   "Revoke a specific metadata record",
		Aliases: []string{"r"},
		Long:    `.....`,
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) (err error) {
			recordID := args[0]
			ctxt := context.Background()
			_, err = sdk.RevokeMetadata(ctxt, recordID, CreateAdapter(true), logger)
			return
		},
	}
)

func init() {
	rootCmd.AddCommand(metaCmd)

	metaCmd.AddCommand(metaAddCmd)
	metaAddCmd.Flags().StringVarP(&schemaName, "schema", "s", "", "URN/UUID of schema")
	metaAddCmd.Flags().StringVarP(&metaFile, "file", "f", "", "Path to file containing metdata")
	metaAddCmd.Flags().StringVarP(&inputFormat, "format", "", "json", "Format of service description file [json, yaml]")

	metaCmd.AddCommand(metaGetCmd)
	metaGetCmd.Flags().StringVarP(&schemaName, "schema", "s", "", "URN/UUID of schema")
	metaGetCmd.Flags().StringVarP(&time_at, "time-at", "t", "", "Timestamp for which to request information [now]")

	metaCmd.AddCommand(metaRevokeCmd)
}
