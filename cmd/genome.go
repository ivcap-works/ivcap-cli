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

// The NCBI access lives in the ivcap-ncbi-genome-download library; this file
// maps flags onto its requests and prints the results the way the other commands
// do. Download and reports need no IVCAP login; import also talks to IVCAP.

var (
	genomeAPIKey string

	// genome download
	genomeFile            string
	genomeForce           bool
	genomeIncludes        []string
	genomeChromosomes     []string
	genomeAssemblyVersion string
	genomeFastZip         bool

	// genome import
	genomeImportIncludes       []string
	genomeImportChromosomes    []string
	genomeImportVersion        string
	genomeImportFastZip        bool
	genomeImportName           string
	genomeImportPolicy         string
	genomeImportChunkSize      int64
	genomeImportReportSchema   string
	genomeImportManifestSchema string
	genomeImportLicense        string

	// genome reports
	genomeRepVersion  string
	genomeRepRef      bool
	genomeRepAnnot    bool
	genomeRepSource   string
	genomeRepLevels   []string
	genomeRepMAG      string
	genomeRepAtypical bool
	genomeRepMulti    bool
	genomeRepFromType bool
)

func init() {
	rootCmd.AddCommand(genomeCmd)
	genomeCmd.AddCommand(genomeDownloadCmd)
	genomeCmd.AddCommand(genomeImportCmd)
	genomeCmd.AddCommand(genomeReportsCmd)

	// 'genome list' only talks to IVCAP, so the NCBI API key is not offered there.
	for _, c := range []*cobra.Command{genomeDownloadCmd, genomeImportCmd, genomeReportsCmd} {
		c.Flags().StringVar(&genomeAPIKey, "ncbi-api-key", "",
			"NCBI API key, which raises NCBI's rate limit [NCBI_API_KEY]")
	}

	d := genomeDownloadCmd.Flags()
	d.StringVarP(&genomeFile, "file", "f", "", "File to write the package to [<ACCESSION>.zip], or - for stdout")
	d.BoolVar(&genomeForce, "force", false, "Overwrite the file if it already exists")
	d.StringSliceVar(&genomeIncludes, "include", []string{"genome"},
		genomeIncludeHelp)
	d.StringSliceVar(&genomeChromosomes, "chromosomes", nil,
		genomeChromosomesHelp)
	d.StringVar(&genomeAssemblyVersion, "assembly-version", "",
		genomeOneVersionHelp)
	d.BoolVar(&genomeFastZip, "fast-zip-validation", false,
		"Only check the zip's directory after the download, not every file in it")

	i := genomeImportCmd.Flags()
	i.StringSliceVar(&genomeImportIncludes, "include", []string{"genome"},
		genomeIncludeHelp)
	i.StringSliceVar(&genomeImportChromosomes, "chromosomes", nil,
		genomeChromosomesHelp)
	i.StringVar(&genomeImportVersion, "assembly-version", "",
		genomeOneVersionHelp)
	i.BoolVar(&genomeImportFastZip, "fast-zip-validation", false,
		"Only check the zip's directory after the download, not every file in it")
	i.StringVarP(&genomeImportName, "name", "n", "", "Artifact name [zip file name]")
	i.StringVarP(&genomeImportPolicy, "policy", "p", "", "Policy controlling access to the artifact and aspects")
	i.Int64Var(&genomeImportChunkSize, "chunk-size", DEF_CHUNK_SIZE, fmt.Sprintf("Chunk size in bytes for splitting large files: at least %d, or -1 to upload without chunking", MIN_CHUNK_SIZE))
	i.StringVar(&genomeImportReportSchema, "report-schema", ncbiAssemblyReportSchema,
		"Schema URN for the assembly report aspect")
	i.StringVar(&genomeImportManifestSchema, "manifest-schema", artifactArchiveManifestSchema,
		"Schema URN for the RO-Crate manifest aspect")
	i.StringVar(&genomeImportLicense, "license", "",
		"License of the imported data, recorded in the manifest: an SPDX identifier such as CC0-1.0, a license URL, "+
			"or 'undefined' [a link to NCBI's data usage policy, which states no license]")

	r := genomeReportsCmd.Flags()
	r.StringVar(&genomeRepVersion, "assembly-version", "",
		genomeReportsVersionHelp)
	r.BoolVar(&genomeRepRef, "reference", false, "Limit to reference genomes")
	r.BoolVar(&genomeRepAnnot, "annotated", false, "Limit to annotated genomes")
	r.StringVar(&genomeRepSource, "assembly-source", "all", "Limit to 'refseq' (GCF_) or 'genbank' (GCA_) assemblies")
	r.StringSliceVar(&genomeRepLevels, "assembly-level", nil,
		"Limit to assembly levels (comma-separated): chromosome, complete, contig, scaffold")
	r.StringVar(&genomeRepMAG, "mag", "all", "Limit to metagenome assembled genomes ('only') or remove them ('exclude')")
	r.BoolVar(&genomeRepAtypical, "exclude-atypical", false, "Exclude atypical assemblies")
	r.BoolVar(&genomeRepMulti, "exclude-multi-isolate", false, "Exclude assemblies from multi-isolate projects")
	r.BoolVar(&genomeRepFromType, "from-type", false, "Only return records with type material")
}

var (
	genomeCmd = &cobra.Command{
		Use:   "genome",
		Short: "Look up, download and import genomes from NCBI",
		Long: `Look up genome assemblies at NCBI, download their data packages and import them into IVCAP.

The download and reports commands only talk to NCBI Datasets
(https://www.ncbi.nlm.nih.gov/datasets/), so they need no IVCAP context or login. Import uploads
an existing package to the active IVCAP deployment and attaches report and manifest aspects.
List shows what has been imported and needs the same context and login as import.`,
	}

	genomeDownloadCmd = &cobra.Command{
		Use:   "download ACCESSION [flags] [-f file|-]",
		Short: "Download the data package of one genome assembly as a zip file",
		Long: `Download the NCBI Datasets data package of exactly one genome assembly.

ACCESSION must be an assembly accession (GCF_... or GCA_...), with or without a version. A
BioProject accession is rejected, because a project can name many genomes: use
'ivcap genome reports' to see which assemblies it has, then pass one of them.

The package contains the genomic FASTA by default; add annotation files and reports with
--include. By default it is written to ./<ACCESSION>.zip and the command refuses to replace an
existing file unless --force is given. Use '-f -' to write the zip to stdout.

With --output json|yaml the result (accession, file, size and the list of files in the package)
is printed in machine-readable form.

--timeout does not apply to the transfer itself; stop it with Ctrl-C.`,
		Example: `  ivcap genome download GCF_000001735.4
  ivcap genome download GCF_000001735.4 --include genome,gff3,seq-report -f TAIR10.1.zip
  ivcap --output json genome download GCF_000001735.4 --include all`,
		Args: exactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctxt, stop := signal.NotifyContext(context.Background(), os.Interrupt)
			defer stop()
			o := genomeDownloadOpts{
				Accession:       args[0],
				File:            genomeFile,
				Force:           genomeForce,
				Includes:        genomeIncludes,
				Chromosomes:     genomeChromosomes,
				AssemblyVersion: genomeAssemblyVersion,
				FastZip:         genomeFastZip,
				Format:          outputFormat,
				Silent:          silent,
			}
			client := ncbiNewClient(genomeAPIKey, silent, os.Stderr)
			return runGenomeDownload(ctxt, client, o, os.Stdout, os.Stderr)
		},
	}

	genomeImportCmd = &cobra.Command{
		Use:   "import ACCESSION [flags]",
		Short: "Import an NCBI genome package into IVCAP",
		Long: `Import an NCBI genome data package as an IVCAP artifact.

The package is downloaded to a temporary file which is removed after upload; no local package is
left behind. The same --include, --chromosomes, --assembly-version and --fast-zip-validation options
as 'genome download' control the package. The imported artifact receives an assembly report
aspect and an RO-Crate 1.2 archive manifest aspect.

The manifest records a license for the data. NCBI states none: it places no restrictions itself,
but submitters may claim rights that it cannot assess. By default the manifest therefore links to
NCBI's data usage policy, which is a pointer and not a license grant. Use --license to state another.

If an aspect request fails after upload, the error names the uploaded artifact so it can be repaired.`,
		Example: `  ivcap genome import GCF_000001735.4
  ivcap genome import GCF_000001735.4 --include genome,gff3,seq-report
  ivcap --output json genome import GCF_000001735.4 --policy urn:ivcap:policy:...`,
		Args: exactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctxt, stop := signal.NotifyContext(context.Background(), os.Interrupt)
			defer stop()
			o := genomeImportOpts{
				Accession:       args[0],
				Includes:        genomeImportIncludes,
				Chromosomes:     genomeImportChromosomes,
				AssemblyVersion: genomeImportVersion,
				FastZip:         genomeImportFastZip,
				Name:            genomeImportName,
				Policy:          genomeImportPolicy,
				ChunkSize:       genomeImportChunkSize,
				ReportSchema:    genomeImportReportSchema,
				ManifestSchema:  genomeImportManifestSchema,
				License:         genomeImportLicense,
				Format:          outputFormat,
				Silent:          silent,
			}
			client := ncbiNewClient(genomeAPIKey, silent, os.Stderr)
			return runGenomeImport(ctxt, client, o, CreateAdapterWithTimeout(true, timeout), os.Stdout, os.Stderr)
		},
	}

	genomeReportsCmd = &cobra.Command{
		Use:   "reports ACCESSION... [flags]",
		Short: "Show the NCBI genome assembly reports for assembly or BioProject accessions",
		Long: `Look up the NCBI genome assembly report of each assembly named by the accessions.

Accessions may be assembly accessions (GCF_... or GCA_...) or BioProject accessions (PRJNA...);
a BioProject expands to its assemblies. Use this to find the assembly you want to download.

With --output json|yaml the full reports are printed. Each report follows the JSON Schema at
https://github.com/ivcap-works/ivcap-ncbi-genome-download/tree/v0.2.0/schema`,
		Example: `  ivcap genome reports GCF_000001735.4
  ivcap --output json genome reports GCF_000001735.4
  ivcap genome reports PRJNA10719 --reference`,
		Args: minimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctxt, stop := signal.NotifyContext(context.Background(), os.Interrupt)
			defer stop()
			if timeout > 0 {
				var cancel context.CancelFunc
				ctxt, cancel = context.WithTimeout(ctxt, time.Duration(timeout)*time.Second)
				defer cancel()
			}
			o := genomeReportsOpts{
				Accessions:          args,
				AssemblyVersion:     genomeRepVersion,
				Reference:           genomeRepRef,
				Annotated:           genomeRepAnnot,
				Source:              genomeRepSource,
				Levels:              genomeRepLevels,
				MAG:                 genomeRepMAG,
				ExcludeAtypical:     genomeRepAtypical,
				ExcludeMultiIsolate: genomeRepMulti,
				FromType:            genomeRepFromType,
				Format:              outputFormat,
			}
			client := ncbiNewClient(genomeAPIKey, silent, os.Stderr)
			return runGenomeReports(ctxt, client, o, os.Stdout)
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

type genomeDownloadOpts struct {
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

// genomeDownloadRequest maps the flags onto a library request.
func genomeDownloadRequest(o genomeDownloadOpts) (ncbigenome.Request, error) {
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

var genomeFileSafeName = regexp.MustCompile(`^[A-Za-z0-9_.]+$`)

// genomeDestination returns where to write the package. Without --file it is a
// file in the working directory named after the accession, and the accession is
// checked first so it can never steer the path out of the working directory.
func genomeDestination(accession, file string) (string, error) {
	if file != "" {
		return filepath.Clean(file), nil
	}
	if !genomeFileSafeName.MatchString(accession) {
		return "", fmt.Errorf("%q is not a valid assembly accession", accession)
	}
	return strings.ToUpper(accession) + ".zip", nil
}

// genomeCheckOverwrite refuses to replace an existing file unless force is set.
func genomeCheckOverwrite(path string, force bool) error {
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

// genomeNewBar makes the progress bar of a download. It is a variable so that tests can use a fake.
var genomeNewBar = sdk.GetProgressBar

// genomeProgress returns a progress callback for the library and a function that
// ends the progress line. Both do nothing when progress output is not wanted.
func genomeProgress(quiet bool, format string, stderr io.Writer) (func(int64), func()) {
	if quiet || format != "" {
		return nil, func() {}
	}
	var bar io.Writer
	var last int64
	onProgress := func(written int64) {
		if bar == nil {
			bar = genomeNewBar("... downloading package", -1)
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
		if bar == nil {
			return
		}
		// The size is unknown, so this bar is a spinner that redraws itself from a background
		// goroutine until it is finished. Finish it, or it keeps spinning over everything that
		// follows, such as the upload of an import.
		if f, ok := bar.(interface{ Finish() error }); ok {
			_ = f.Finish()
		}
		fmt.Fprintln(stderr) // move past the progress bar
	}
	return onProgress, done
}

type genomeDownloadOutput struct {
	Accession string               `json:"accession"`
	File      string               `json:"file"`
	Bytes     int64                `json:"bytes"`
	Warning   string               `json:"warning,omitempty"`
	Manifest  *ncbigenome.Manifest `json:"manifest,omitempty"`
}

func runGenomeDownload(ctxt context.Context, client *ncbigenome.Client, o genomeDownloadOpts, stdout, stderr io.Writer) error {
	req, err := genomeDownloadRequest(o)
	if err != nil {
		return err
	}
	path, err := genomeDestination(o.Accession, o.File)
	if err != nil {
		return err
	}
	toStdout := path == "-"
	if toStdout && o.Format != "" {
		return errors.New("'-f -' writes the package to stdout, so it cannot be combined with --output")
	}
	if err := genomeCheckOverwrite(path, o.Force); err != nil {
		return err
	}

	onProgress, done := genomeProgress(o.Silent, o.Format, stderr)
	req.OnProgress = onProgress

	var res *ncbigenome.Result
	if toStdout {
		res, err = client.Download(ctxt, req, stdout)
	} else {
		res, err = client.DownloadToFile(ctxt, req, path)
	}
	done()
	if err != nil {
		return genomeCLIErr(err)
	}

	if res.Warning != "" && !o.Silent {
		fmt.Fprintln(stderr, "Warning:", res.Warning)
	}
	if toStdout {
		if !o.Silent {
			fmt.Fprintf(stderr, "Wrote %s of %s to stdout\n", genomeBytes(res.Bytes), res.Accession)
		}
		return nil
	}
	return printGenomeDownload(stdout, res, path, o.Format)
}

func printGenomeDownload(w io.Writer, res *ncbigenome.Result, path, format string) error {
	switch format {
	case "json", "yaml":
		return printGenomeStructured(w, genomeDownloadOutput{
			Accession: res.Accession,
			File:      path,
			Bytes:     res.Bytes,
			Warning:   res.Warning,
			Manifest:  res.Manifest,
		}, format)
	}
	fmt.Fprintf(w, "Downloaded %s to %s (%s)\n", res.Accession, path, genomeBytes(res.Bytes))
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

// ---- import -----------------------------------------------------------------

const (
	ncbiAssemblyReportSchema = "urn:ivcap:schema:ncbi.assembly-data-report.1"
)

// Help texts of the options that several 'genome' commands share. The meanings are the ones the
// library documents (Include, AssemblyVersion, Request.Chromosomes).
const (
	genomeIncludeHelp = "Files to include, comma-separated: genome (genomic FASTA), rna (transcripts), protein (amino acid " +
		"sequences), cds (nucleotide coding sequences), gff3 or gtf (annotation), gbff (GenBank flat file), seq-report " +
		"(per-sequence report), all (every type, use alone) or none (metadata only, use alone)"
	genomeChromosomesHelp = "Limit the genome FASTA to these chromosomes (comma-separated names, or 'all'); " +
		"the names are passed to NCBI unchanged"
	genomeOneVersionHelp = "Version policy [a versioned ACCESSION is used as given, an unversioned one means its current " +
		"version]: 'current' (same as 'latest') ignores the version in ACCESSION and takes the current one; 'all' selects " +
		"every version and is rejected here unless there is exactly one"
	genomeReportsVersionHelp = "Version policy [a versioned ACCESSION is used as given, an unversioned one means its " +
		"current version]: 'current' (same as 'latest') ignores any version and returns the current one; 'all' returns " +
		"every version of each assembly"
)

// ncbiDataLicense is what an import records as the license when --license is not given. NCBI
// states no license: it places no restrictions itself, but submitters may claim rights that it
// cannot assess, so this is a pointer to its policy and not a license grant.
// Source: https://www.ncbi.nlm.nih.gov/home/about/policies/ (section on molecular data usage).
var ncbiDataLicense = roCrateLicense{
	ID:   "https://www.ncbi.nlm.nih.gov/home/about/policies/",
	Name: "NCBI Website and Data Usage Policies and Disclaimers",
	Description: "NCBI itself places no restrictions on the use or distribution of this data, and does not accept " +
		"data whose submitter requested restrictions. Individual submitters may still claim patent, copyright or " +
		"other rights, which NCBI cannot assess; it gives no unrestricted permission on their behalf. " +
		"This is a pointer to that policy, not a license grant.",
}

// BioProject accessions (PRJNA..., PRJEB..., PRJDB...) can name many genomes.
var ncbiBioProject = regexp.MustCompile(`(?i)^PRJ[A-Z]{2}[0-9]+$`)

func resolveGenomeLicense(value string) (roCrateLicense, error) {
	if strings.TrimSpace(value) == "" {
		return ncbiDataLicense, nil
	}
	return resolveLicense(value)
}

// genomeTooManyAssemblies explains that an accession matched several assemblies.
func genomeTooManyAssemblies(accession string, reports []ncbigenome.AssemblyDataReport) error {
	shown := make([]string, 0, 5)
	for i, r := range reports {
		if i == 5 {
			shown = append(shown, "...")
			break
		}
		shown = append(shown, r.Accession)
	}
	return fmt.Errorf("%w: %s matches %d assemblies (%s); pass one assembly accession, and use "+
		"'ivcap genome reports %s' to see them", ncbigenome.ErrNotSingleGenome, accession, len(reports),
		strings.Join(shown, ", "), accession)
}

type genomeImportOpts struct {
	Accession       string
	Includes        []string
	Chromosomes     []string
	AssemblyVersion string
	FastZip         bool
	Name            string
	Policy          string
	ChunkSize       int64
	ReportSchema    string
	ManifestSchema  string
	License         string
	Format          string // "", "json" or "yaml"
	Silent          bool
}

type genomeImportOutput struct {
	Artifact       string `json:"artifact"`
	Accession      string `json:"accession"`
	Bytes          int64  `json:"bytes"`
	ReportAspect   string `json:"report_aspect,omitempty"`
	ManifestAspect string `json:"manifest_aspect,omitempty"`
}

func runGenomeImport(
	ctxt context.Context,
	client *ncbigenome.Client,
	o genomeImportOpts,
	adapter *adpt.Adapter,
	stdout, stderr io.Writer,
) error {
	if o.ReportSchema == "" {
		return errors.New("report schema must not be empty")
	}
	if o.ManifestSchema == "" {
		return errors.New("manifest schema must not be empty")
	}
	// Fail before downloading anything. The package is a file, so its size is always known.
	if err := validateChunkSize(o.ChunkSize, 0); err != nil {
		return err
	}
	lic, err := resolveGenomeLicense(o.License)
	if err != nil {
		return err
	}
	if ncbiBioProject.MatchString(o.Accession) {
		return fmt.Errorf("%w: %s is a BioProject accession and can name many genomes; use "+
			"'ivcap genome reports' to see which assemblies it has, then pass one assembly accession",
			ncbigenome.ErrNotSingleGenome, o.Accession)
	}

	req, err := genomeDownloadRequest(genomeDownloadOpts{
		Accession:       o.Accession,
		Includes:        o.Includes,
		Chromosomes:     o.Chromosomes,
		AssemblyVersion: o.AssemblyVersion,
		FastZip:         o.FastZip,
	})
	if err != nil {
		return err
	}

	// Fetch the report explicitly: it is stored as its own aspect, independently
	// of any report file NCBI may include inside the downloaded package.
	reports, err := client.AssemblyReports(ctxt, req)
	if err != nil {
		return genomeCLIErr(err)
	}
	switch {
	case len(reports) == 0:
		return genomeCLIErr(ncbigenome.ErrNoAssemblies)
	case len(reports) > 1:
		return genomeTooManyAssemblies(o.Accession, reports)
	}
	reportBytes, err := genomeAspectJSON(reports[0], o.ReportSchema)
	if err != nil {
		return fmt.Errorf("cannot encode assembly report: %w", err)
	}

	tmpDir, err := os.MkdirTemp("", "ivcap-ncbi-import-*")
	if err != nil {
		return fmt.Errorf("cannot create temporary download directory: %w", err)
	}
	defer os.RemoveAll(tmpDir)
	path := filepath.Join(tmpDir, "package.zip")
	onProgress, done := genomeProgress(o.Silent, o.Format, stderr)
	req.OnProgress = onProgress
	res, err := client.DownloadToFile(ctxt, req, path)
	done()
	if err != nil {
		return genomeCLIErr(err)
	}
	if res.Accession != reports[0].Accession {
		return fmt.Errorf("the assembly report is for %s but the downloaded package is for %s (NCBI changed "+
			"between the two requests); nothing was imported, run the import again", reports[0].Accession, res.Accession)
	}
	if res.Warning != "" && !o.Silent {
		fmt.Fprintln(stderr, "Warning:", res.Warning)
	}
	info, err := os.Stat(filepath.Clean(path))
	if err != nil {
		return fmt.Errorf("cannot read temporary NCBI package: %w", err)
	}
	manifest := res.Manifest
	if manifest == nil {
		manifest, err = ncbigenome.ReadManifest(path)
		if err != nil {
			return fmt.Errorf("cannot read temporary package manifest: %w", err)
		}
	}
	published := info.ModTime().UTC().Format(time.DateOnly)
	if reports[0].AssemblyInfo != nil && reports[0].AssemblyInfo.ReleaseDate != "" {
		published = reports[0].AssemblyInfo.ReleaseDate
	}
	entries := make([]archiveManifestEntry, 0, len(manifest.Files))
	for _, file := range manifest.Files {
		entries = append(entries, archiveManifestEntry{Name: file.Name, Size: file.Size})
	}
	manifestBytes, err := json.Marshal(archiveManifestAspect{
		Schema:  o.ManifestSchema,
		Profile: roCrateProfile,
		ROCrate: buildROCrate(archiveManifestMetadata{
			Identifier:  reports[0].Accession,
			Name:        fmt.Sprintf("NCBI genome data package %s", reports[0].Accession),
			Description: "NCBI Datasets genome data package imported into IVCAP.",
			Published:   published,
			License:     lic,
		}, entries),
	})
	if err != nil {
		return fmt.Errorf("cannot encode package manifest: %w", err)
	}

	f, err := os.Open(filepath.Clean(path))
	if err != nil {
		return fmt.Errorf("cannot open NCBI package %q: %w", path, err)
	}
	defer f.Close()
	artifactName := o.Name
	if artifactName == "" {
		artifactName = res.Accession + ".zip"
	}
	uploadSilent := o.Silent || o.Format != ""
	if !uploadSilent {
		fmt.Fprintf(stderr, "Uploading %s (%s) to IVCAP\n", artifactName, genomeBytes(info.Size()))
	}
	resp, err := sdk.CreateArtifact(ctxt, &sdk.CreateArtifactRequest{
		Name: artifactName, Size: info.Size(), Policy: o.Policy,
	}, "application/zip", info.Size(), nil, adapter, logger)
	if err != nil {
		return fmt.Errorf("cannot create artifact for NCBI package %s: %w", res.Accession, err)
	}
	if resp.ID == nil || resp.DataHref == nil {
		return errors.New("artifact create response is missing its id or upload URL")
	}
	artifactID := *resp.ID
	uploadPath, err := (*adapter).GetPath(*resp.DataHref)
	if err != nil {
		return fmt.Errorf("cannot parse upload URL for artifact %s: %w", artifactID, err)
	}
	if err := sdk.UploadArtifact(ctxt, f, info.Size(), 0, o.ChunkSize, uploadPath, adapter, uploadSilent, logger); err != nil {
		return fmt.Errorf("cannot upload artifact %s: %w", artifactID, err)
	}

	reportAspect, err := addAspectContent(ctxt, artifactID, o.ReportSchema, o.Policy, reportBytes, adapter)
	if err != nil {
		return fmt.Errorf("artifact %s was uploaded, but its report aspect could not be added: %w", artifactID, err)
	}
	manifestAspect, err := addAspectContent(ctxt, artifactID, o.ManifestSchema, o.Policy, manifestBytes, adapter)
	if err != nil {
		return fmt.Errorf("artifact %s and report aspect %s were created, but the manifest aspect could not be added: %w",
			artifactID, reportAspect, err)
	}

	result := genomeImportOutput{
		Artifact: artifactID, Accession: reports[0].Accession, Bytes: info.Size(),
		ReportAspect: reportAspect, ManifestAspect: manifestAspect,
	}
	if o.Format == "json" || o.Format == "yaml" {
		return printGenomeStructured(stdout, result, o.Format)
	}
	if o.Silent {
		_, err = fmt.Fprintln(stdout, artifactID)
		return err
	}
	_, err = fmt.Fprintf(stdout, "Downloaded and imported %s as %s (%s)\nReport aspect: %s\nManifest aspect: %s\n",
		result.Accession, artifactID, genomeBytes(info.Size()), reportAspect, manifestAspect)
	return err
}

func genomeAspectJSON(value any, schema string) ([]byte, error) {
	b, err := json.Marshal(value)
	if err != nil {
		return nil, err
	}
	var aspect map[string]any
	if err := json.Unmarshal(b, &aspect); err != nil {
		return nil, err
	}
	aspect["$schema"] = schema
	aspect["json_schema"] = ncbigenome.AssemblyDataReportSchemaURL
	return json.Marshal(aspect)
}

// ---- reports ----------------------------------------------------------------

type genomeReportsOpts struct {
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

// genomeReportsRequest maps the flags onto a library request.
func genomeReportsRequest(o genomeReportsOpts) (ncbigenome.Request, error) {
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

func runGenomeReports(ctxt context.Context, client *ncbigenome.Client, o genomeReportsOpts, stdout io.Writer) error {
	req, err := genomeReportsRequest(o)
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
		return printGenomeStructured(stdout, reports, o.Format)
	}
	return printGenomeReportsTable(stdout, reports)
}

func printGenomeReportsTable(w io.Writer, reports []ncbigenome.AssemblyDataReport) error {
	if len(reports) == 0 {
		_, err := fmt.Fprintln(w, "No matching assemblies.")
		return err
	}
	t := table.NewWriter()
	t.SetOutputMirror(w)
	t.AppendHeader(table.Row{"Accession", "Organism", "Assembly", "Level", "Status", "Source", "Released"})
	for _, r := range reports {
		t.AppendRow(genomeReportRow(r))
	}
	t.Render()
	return nil
}

func genomeReportRow(r ncbigenome.AssemblyDataReport) table.Row {
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

// printGenomeStructured prints v as indented JSON or YAML. The value is converted
// through JSON first, as adapter.ReplyPrinter does, so both formats use the JSON
// property names; YAML marshalling alone would ignore the json tags and rename
// the keys.
func printGenomeStructured(w io.Writer, v any, format string) error {
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

func genomeBytes(n int64) string {
	if n < 0 {
		n = 0
	}
	return humanize.Bytes(uint64(n))
}

// genomeCLIError carries a command-line wording of a library error while keeping
// the original reachable through errors.Is and errors.As.
type genomeCLIError struct {
	msg string
	err error
}

func (e *genomeCLIError) Error() string { return e.msg }
func (e *genomeCLIError) Unwrap() error { return e.err }

// genomeCLIErr words library errors that name the Go API for command-line users:
// the library tells callers to use AssemblyReports, the CLI has a command for that.
func genomeCLIErr(err error) error {
	if err == nil {
		return nil
	}
	msg := err.Error()
	fixed := strings.ReplaceAll(msg, "use AssemblyReports to see which assemblies it has",
		"use 'ivcap genome reports' to see which assemblies it has")
	if fixed == msg {
		return err
	}
	return &genomeCLIError{msg: fixed, err: err}
}
