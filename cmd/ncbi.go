// Copyright 2026 Commonwealth Scientific and Industrial Research Organisation (CSIRO) ABN 41 687 119 230
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
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	humanize "github.com/dustin/go-humanize"
	sdk "github.com/ivcap-works/ivcap-cli/pkg"
	adpt "github.com/ivcap-works/ivcap-cli/pkg/adapter"
	ncbigenome "github.com/ivcap-works/ivcap-ncbi-genome-download"
	"github.com/jedib0t/go-pretty/v6/table"
	"github.com/spf13/cobra"
)

// The `ncbi` commands talk to NCBI Datasets, not to an IVCAP deployment, so they
// need no context and no login. The NCBI access itself lives in the
// ivcap-ncbi-genome-download library; this file only maps flags onto its
// requests and prints the results the way the other commands do.

var (
	ncbiAPIKey string

	// ncbi genome download
	ncbiFile            string
	ncbiForce           bool
	ncbiIncludes        []string
	ncbiChromosomes     []string
	ncbiAssemblyVersion string
	ncbiFastZip         bool

	// ncbi genome reports
	ncbiRepVersion  string
	ncbiRepRef      bool
	ncbiRepAnnot    bool
	ncbiRepSource   string
	ncbiRepLevels   []string
	ncbiRepMAG      string
	ncbiRepAtypical bool
	ncbiRepMulti    bool
	ncbiRepFromType bool
)

func init() {
	rootCmd.AddCommand(ncbiCmd)
	ncbiCmd.AddCommand(ncbiGenomeCmd)
	ncbiGenomeCmd.AddCommand(ncbiDownloadCmd)
	ncbiGenomeCmd.AddCommand(ncbiReportsCmd)

	ncbiCmd.PersistentFlags().StringVar(&ncbiAPIKey, "ncbi-api-key", "",
		"NCBI API key, which raises NCBI's rate limit [NCBI_API_KEY]")

	d := ncbiDownloadCmd.Flags()
	d.StringVarP(&ncbiFile, "file", "f", "", "File to write the package to [<ACCESSION>.zip], or - for stdout")
	d.BoolVar(&ncbiForce, "force", false, "Overwrite the file if it already exists")
	d.StringSliceVar(&ncbiIncludes, "include", []string{"genome"},
		"Files to include (comma-separated): genome, rna, protein, cds, gff3, gtf, gbff, seq-report, all, none")
	d.StringSliceVar(&ncbiChromosomes, "chromosomes", nil,
		"Limit the genome FASTA to these chromosomes (comma-separated, or 'all')")
	d.StringVar(&ncbiAssemblyVersion, "assembly-version", "",
		"Version policy: 'current', 'latest' or 'all' [use the version given in ACCESSION]")
	d.BoolVar(&ncbiFastZip, "fast-zip-validation", false,
		"Only check the zip's directory after the download, not every file in it")

	r := ncbiReportsCmd.Flags()
	r.StringVar(&ncbiRepVersion, "assembly-version", "",
		"Version policy: 'current', 'latest' or 'all' [use the versions given]")
	r.BoolVar(&ncbiRepRef, "reference", false, "Limit to reference genomes")
	r.BoolVar(&ncbiRepAnnot, "annotated", false, "Limit to annotated genomes")
	r.StringVar(&ncbiRepSource, "assembly-source", "all", "Limit to 'refseq' (GCF_) or 'genbank' (GCA_) assemblies")
	r.StringSliceVar(&ncbiRepLevels, "assembly-level", nil,
		"Limit to assembly levels (comma-separated): chromosome, complete, contig, scaffold")
	r.StringVar(&ncbiRepMAG, "mag", "all", "Limit to metagenome assembled genomes ('only') or remove them ('exclude')")
	r.BoolVar(&ncbiRepAtypical, "exclude-atypical", false, "Exclude atypical assemblies")
	r.BoolVar(&ncbiRepMulti, "exclude-multi-isolate", false, "Exclude assemblies from multi-isolate projects")
	r.BoolVar(&ncbiRepFromType, "from-type", false, "Only return records with type material")
}

var (
	ncbiCmd = &cobra.Command{
		Use:   "ncbi",
		Short: "Look up and download genomes from NCBI",
		Long: `Look up genome assemblies at NCBI and download their data packages.

These commands talk to NCBI Datasets (https://www.ncbi.nlm.nih.gov/datasets/), not to an
IVCAP deployment, so they need no context and no login.`,
	}

	ncbiGenomeCmd = &cobra.Command{
		Use:   "genome",
		Short: "Genome assemblies at NCBI",
	}

	ncbiDownloadCmd = &cobra.Command{
		Use:   "download ACCESSION [flags] [-f file|-]",
		Short: "Download the data package of one genome assembly as a zip file",
		Long: `Download the NCBI Datasets data package of exactly one genome assembly.

ACCESSION must be an assembly accession (GCF_... or GCA_...), with or without a version. A
BioProject accession is rejected, because a project can name many genomes: use
'ivcap ncbi genome reports' to see which assemblies it has, then pass one of them.

The package contains the genomic FASTA by default; add annotation files and reports with
--include. By default it is written to ./<ACCESSION>.zip and the command refuses to replace an
existing file unless --force is given. Use '-f -' to write the zip to stdout.

With --output json|yaml the result (accession, file, size and the list of files in the package)
is printed in machine-readable form.

--timeout does not apply to the transfer itself; stop it with Ctrl-C.`,
		Example: `  ivcap ncbi genome download GCF_000001735.4
  ivcap ncbi genome download GCF_000001735.4 --include genome,gff3,seq-report -f TAIR10.1.zip
  ivcap --output json ncbi genome download GCF_000001735.4 --include all`,
		Args: exactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctxt, stop := signal.NotifyContext(context.Background(), os.Interrupt)
			defer stop()
			o := ncbiDownloadOpts{
				Accession:       args[0],
				File:            ncbiFile,
				Force:           ncbiForce,
				Includes:        ncbiIncludes,
				Chromosomes:     ncbiChromosomes,
				AssemblyVersion: ncbiAssemblyVersion,
				FastZip:         ncbiFastZip,
				Format:          outputFormat,
				Silent:          silent,
			}
			client := ncbiNewClient(ncbiAPIKey, silent, os.Stderr)
			return runNcbiDownload(ctxt, client, o, os.Stdout, os.Stderr)
		},
	}

	ncbiReportsCmd = &cobra.Command{
		Use:   "reports ACCESSION... [flags]",
		Short: "Show the NCBI genome assembly reports for assembly or BioProject accessions",
		Long: `Look up the NCBI genome assembly report of each assembly named by the accessions.

Accessions may be assembly accessions (GCF_... or GCA_...) or BioProject accessions (PRJNA...);
a BioProject expands to its assemblies. Use this to find the assembly you want to download.

With --output json|yaml the full reports are printed. Each report follows the JSON Schema at
https://github.com/ivcap-works/ivcap-ncbi-genome-download/tree/main/schema`,
		Example: `  ivcap ncbi genome reports GCF_000001735.4
  ivcap --output json ncbi genome reports GCF_000001735.4
  ivcap ncbi genome reports PRJNA10719 --reference`,
		Args: minimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctxt, stop := signal.NotifyContext(context.Background(), os.Interrupt)
			defer stop()
			if timeout > 0 {
				var cancel context.CancelFunc
				ctxt, cancel = context.WithTimeout(ctxt, time.Duration(timeout)*time.Second)
				defer cancel()
			}
			o := ncbiReportsOpts{
				Accessions:          args,
				AssemblyVersion:     ncbiRepVersion,
				Reference:           ncbiRepRef,
				Annotated:           ncbiRepAnnot,
				Source:              ncbiRepSource,
				Levels:              ncbiRepLevels,
				MAG:                 ncbiRepMAG,
				ExcludeAtypical:     ncbiRepAtypical,
				ExcludeMultiIsolate: ncbiRepMulti,
				FromType:            ncbiRepFromType,
				Format:              outputFormat,
			}
			client := ncbiNewClient(ncbiAPIKey, silent, os.Stderr)
			return runNcbiReports(ctxt, client, o, os.Stdout)
		},
	}
)

// ncbiNewClient creates the NCBI client. The key falls back to $NCBI_API_KEY.
func ncbiNewClient(apiKey string, quiet bool, stderr io.Writer) *ncbigenome.Client {
	if apiKey == "" {
		apiKey = os.Getenv("NCBI_API_KEY")
	}
	userAgent := "ivcap-cli"
	if v := rootCmd.Version; v != "" {
		userAgent += "/" + v
	}
	return ncbigenome.New(ncbigenome.Options{
		APIKey:    apiKey,
		UserAgent: userAgent,
		OnMessage: func(msg string) {
			if !quiet {
				fmt.Fprintln(stderr, "NCBI:", msg)
			}
		},
	})
}

// ---- download ---------------------------------------------------------------

type ncbiDownloadOpts struct {
	Accession       string
	File            string // path, "-" for stdout, "" for ./<ACCESSION>.zip
	Force           bool
	Includes        []string
	Chromosomes     []string
	AssemblyVersion string
	FastZip         bool
	Format          string // "", "json" or "yaml"
	Silent          bool
}

// ncbiDownloadRequest maps the flags onto a library request.
func ncbiDownloadRequest(o ncbiDownloadOpts) (ncbigenome.Request, error) {
	includes, err := ncbigenome.ParseIncludes(strings.Join(o.Includes, ","))
	if err != nil {
		return ncbigenome.Request{}, err
	}
	version, err := ncbigenome.ParseAssemblyVersion(o.AssemblyVersion)
	if err != nil {
		return ncbigenome.Request{}, err
	}
	return ncbigenome.Request{
		Accessions:          []string{o.Accession},
		AssemblyVersion:     version,
		Include:             includes,
		Chromosomes:         o.Chromosomes,
		SkipZipContentCheck: o.FastZip,
	}, nil
}

var ncbiFileSafeName = regexp.MustCompile(`^[A-Za-z0-9_.]+$`)

// ncbiDestination returns where to write the package. Without --file it is a
// file in the working directory named after the accession, and the accession is
// checked first so it can never steer the path out of the working directory.
func ncbiDestination(accession, file string) (string, error) {
	if file != "" {
		return filepath.Clean(file), nil
	}
	if !ncbiFileSafeName.MatchString(accession) {
		return "", fmt.Errorf("%q is not a valid assembly accession", accession)
	}
	return strings.ToUpper(accession) + ".zip", nil
}

// ncbiCheckOverwrite refuses to replace an existing file unless force is set.
func ncbiCheckOverwrite(path string, force bool) error {
	if path == "-" || force {
		return nil
	}
	_, err := os.Stat(path)
	if err == nil {
		return fmt.Errorf("%s already exists; use --force to overwrite it", path)
	}
	if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return nil
}

// ncbiProgress returns a progress callback for the library and a function that
// ends the progress line. Both do nothing when progress output is not wanted.
func ncbiProgress(quiet bool, format string, stderr io.Writer) (func(int64), func()) {
	if quiet || format != "" {
		return nil, func() {}
	}
	var bar io.Writer
	var last int64
	onProgress := func(written int64) {
		if bar == nil {
			bar = sdk.GetProgressBar("... downloading package", -1)
		}
		delta := written - last
		last = written
		if delta <= 0 {
			return
		}
		if a, ok := bar.(interface{ Add64(int64) error }); ok {
			_ = a.Add64(delta)
			return
		}
		_, _ = bar.Write(make([]byte, delta))
	}
	done := func() {
		if bar != nil {
			fmt.Fprintln(stderr) // move past the progress bar
		}
	}
	return onProgress, done
}

type ncbiDownloadOutput struct {
	Accession string               `json:"accession"`
	File      string               `json:"file"`
	Bytes     int64                `json:"bytes"`
	Warning   string               `json:"warning,omitempty"`
	Manifest  *ncbigenome.Manifest `json:"manifest,omitempty"`
}

func runNcbiDownload(ctxt context.Context, client *ncbigenome.Client, o ncbiDownloadOpts, stdout, stderr io.Writer) error {
	req, err := ncbiDownloadRequest(o)
	if err != nil {
		return err
	}
	path, err := ncbiDestination(o.Accession, o.File)
	if err != nil {
		return err
	}
	toStdout := path == "-"
	if toStdout && o.Format != "" {
		return errors.New("'-f -' writes the package to stdout, so it cannot be combined with --output")
	}
	if err := ncbiCheckOverwrite(path, o.Force); err != nil {
		return err
	}

	onProgress, done := ncbiProgress(o.Silent, o.Format, stderr)
	req.OnProgress = onProgress

	var res *ncbigenome.Result
	if toStdout {
		res, err = client.Download(ctxt, req, stdout)
	} else {
		res, err = client.DownloadToFile(ctxt, req, path)
	}
	done()
	if err != nil {
		return ncbiCLIErr(err)
	}

	if res.Warning != "" && !o.Silent {
		fmt.Fprintln(stderr, "Warning:", res.Warning)
	}
	if toStdout {
		if !o.Silent {
			fmt.Fprintf(stderr, "Wrote %s of %s to stdout\n", ncbiBytes(res.Bytes), res.Accession)
		}
		return nil
	}
	return printNcbiDownload(stdout, res, path, o.Format)
}

func printNcbiDownload(w io.Writer, res *ncbigenome.Result, path, format string) error {
	switch format {
	case "json", "yaml":
		return printNcbiStructured(w, ncbiDownloadOutput{
			Accession: res.Accession,
			File:      path,
			Bytes:     res.Bytes,
			Warning:   res.Warning,
			Manifest:  res.Manifest,
		}, format)
	}
	fmt.Fprintf(w, "Downloaded %s to %s (%s)\n", res.Accession, path, ncbiBytes(res.Bytes))
	if res.Manifest != nil && len(res.Manifest.Files) > 0 {
		t := table.NewWriter()
		t.SetOutputMirror(w)
		t.AppendHeader(table.Row{"File", "Size", "Compressed"})
		for _, f := range res.Manifest.Files {
			t.AppendRow(table.Row{f.Name, humanize.Bytes(f.Size), humanize.Bytes(f.CompressedSize)})
		}
		t.AppendFooter(table.Row{"Total", humanize.Bytes(res.Manifest.TotalSize()),
			humanize.Bytes(res.Manifest.TotalCompressedSize())})
		t.Render()
	}
	return nil
}

// ---- reports ----------------------------------------------------------------

type ncbiReportsOpts struct {
	Accessions          []string
	AssemblyVersion     string
	Reference           bool
	Annotated           bool
	Source              string
	Levels              []string
	MAG                 string
	ExcludeAtypical     bool
	ExcludeMultiIsolate bool
	FromType            bool
	Format              string // "", "json" or "yaml"
}

// ncbiReportsRequest maps the flags onto a library request.
func ncbiReportsRequest(o ncbiReportsOpts) (ncbigenome.Request, error) {
	version, err := ncbigenome.ParseAssemblyVersion(o.AssemblyVersion)
	if err != nil {
		return ncbigenome.Request{}, err
	}
	source, err := ncbigenome.ParseAssemblySource(o.Source)
	if err != nil {
		return ncbigenome.Request{}, err
	}
	levels, err := ncbigenome.ParseAssemblyLevels(strings.Join(o.Levels, ","))
	if err != nil {
		return ncbigenome.Request{}, err
	}
	mag, err := ncbigenome.ParseMAGFilter(o.MAG)
	if err != nil {
		return ncbigenome.Request{}, err
	}
	return ncbigenome.Request{
		Accessions:      o.Accessions,
		AssemblyVersion: version,
		Filters: ncbigenome.Filters{
			ReferenceOnly:       o.Reference,
			AnnotatedOnly:       o.Annotated,
			AssemblySource:      source,
			AssemblyLevels:      levels,
			ExcludeAtypical:     o.ExcludeAtypical,
			ExcludeMultiIsolate: o.ExcludeMultiIsolate,
			TypeMaterialOnly:    o.FromType,
			MAG:                 mag,
		},
	}, nil
}

func runNcbiReports(ctxt context.Context, client *ncbigenome.Client, o ncbiReportsOpts, stdout io.Writer) error {
	req, err := ncbiReportsRequest(o)
	if err != nil {
		return err
	}
	reports, err := client.AssemblyReports(ctxt, req)
	if err != nil {
		return err
	}
	switch o.Format {
	case "json", "yaml":
		if reports == nil {
			reports = []ncbigenome.AssemblyDataReport{} // print [] rather than null
		}
		return printNcbiStructured(stdout, reports, o.Format)
	}
	return printNcbiReportsTable(stdout, reports)
}

func printNcbiReportsTable(w io.Writer, reports []ncbigenome.AssemblyDataReport) error {
	if len(reports) == 0 {
		_, err := fmt.Fprintln(w, "No matching assemblies.")
		return err
	}
	t := table.NewWriter()
	t.SetOutputMirror(w)
	t.AppendHeader(table.Row{"Accession", "Organism", "Assembly", "Level", "Status", "Source", "Released"})
	for _, r := range reports {
		t.AppendRow(ncbiReportRow(r))
	}
	t.Render()
	return nil
}

func ncbiReportRow(r ncbigenome.AssemblyDataReport) table.Row {
	var organism, name, level, status, released string
	if r.Organism != nil {
		organism = r.Organism.OrganismName
	}
	if r.AssemblyInfo != nil {
		name = r.AssemblyInfo.AssemblyName
		level = r.AssemblyInfo.AssemblyLevel
		status = string(r.AssemblyInfo.AssemblyStatus)
		released = r.AssemblyInfo.ReleaseDate
	}
	source := strings.TrimPrefix(string(r.SourceDatabase), "SOURCE_DATABASE_")
	return table.Row{r.Accession, organism, name, level, status, source, released}
}

// ---- shared helpers -----------------------------------------------------------

// printNcbiStructured prints v as indented JSON or YAML. The value is converted
// through JSON first, as adapter.ReplyPrinter does, so both formats use the JSON
// property names; YAML marshalling alone would ignore the json tags and rename
// the keys.
func printNcbiStructured(w io.Writer, v any, format string) error {
	b, err := json.Marshal(v)
	if err != nil {
		return err
	}
	var generic any
	if err := json.Unmarshal(b, &generic); err != nil {
		return err
	}
	s, err := adpt.ToString(generic, format == "yaml")
	if err != nil {
		return err
	}
	_, err = fmt.Fprintln(w, s)
	return err
}

func ncbiBytes(n int64) string {
	if n < 0 {
		n = 0
	}
	return humanize.Bytes(uint64(n))
}

// ncbiCLIError carries a command-line wording of a library error while keeping
// the original reachable through errors.Is and errors.As.
type ncbiCLIError struct {
	msg string
	err error
}

func (e *ncbiCLIError) Error() string { return e.msg }
func (e *ncbiCLIError) Unwrap() error { return e.err }

// ncbiCLIErr words library errors that name the Go API for command-line users:
// the library tells callers to use AssemblyReports, the CLI has a command for that.
func ncbiCLIErr(err error) error {
	if err == nil {
		return nil
	}
	msg := err.Error()
	fixed := strings.ReplaceAll(msg, "use AssemblyReports to see which assemblies it has",
		"use 'ivcap ncbi genome reports' to see which assemblies it has")
	if fixed == msg {
		return err
	}
	return &ncbiCLIError{msg: fixed, err: err}
}
