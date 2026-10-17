---
name: ivcap-genome-import
version: 0.1.0
description: Import one NCBI genome package into IVCAP as an artifact with an assembly report aspect and a manifest aspect.
requires:
  bins: ["ivcap"]
---

# Skill: Import a genome from NCBI into IVCAP (agent-safe)

This command **creates data on the active IVCAP deployment**: one artifact and two aspects. Unlike
`ivcap genome reports` and `ivcap genome download` it needs a logged-in IVCAP context.

## Best practices
- Confirm with the user before running it. State the accession, the `--include` list, the deployment and
  project it will write to, and any `--name` or `--policy`.
- Pass one **assembly accession** (`GCF_...` or `GCA_...`). BioProject accessions (`PRJNA...`) are rejected.
  If unsure which assembly is wanted, look it up first with `ivcap genome reports`.
- Use `--output json` when you will parse the result.
- Use a `--policy` only if the user gave one; do not invent a policy URN.
- Do not run it twice for the same genome: every run creates a new artifact.
- A package can be large (genome, gff3 and seq-report for TAIR10.1 is about 60 MB compressed), so the
  download and upload can take a while. The package is stored in a temporary file that is removed afterwards.
- The manifest records a license pointer. By default it links to NCBI's data usage policy: NCBI states no
  license, it places no restrictions itself, but submitters may claim rights. Never describe the data as public
  domain or under a particular license. Pass `--license` (an SPDX identifier, a license URL, or `undefined`) only
  if the user states one.
- If the command reports missing credentials or context, ask the user to log in. Do not try to authenticate.

## Steps

### 1) Import

```bash
ivcap --output json genome import GCF_000001735.4 --include genome,gff3,seq-report
```

`--include` takes the same values as `ivcap genome download`. `--name` sets the artifact name
(default `<ACCESSION>.zip`).

### 2) Read the result

The JSON result has `artifact` (the artifact URN), `accession`, `bytes`, `report_aspect` and
`manifest_aspect` (the IDs of the two aspects added to the artifact).

### 3) Check (optional)

```bash
ivcap artifact get urn:ivcap:artifact:...
```

The metadata list of the artifact shows the schemas of its aspects: the assembly report
(`urn:ivcap:schema:ncbi.assembly-data-report.1`) and the file manifest
(`urn:ivcap:schema:artifact.archive-manifest.1`).

## Errors
- `a download needs exactly one genome assembly` - pass a single assembly accession, not a BioProject. The
  check happens before anything is downloaded, and the message names the assemblies that matched.
- `invalid --license` - use an SPDX identifier such as `CC-BY-4.0`, an http(s) license URL, or `undefined`.
- `the assembly report is for ... but the downloaded package is for ...` - NCBI changed between its two
  requests. Nothing was imported; run the import again once.
- `artifact ... was uploaded, but its ... aspect could not be added` - the artifact exists but an aspect is
  missing. Tell the user the artifact URN and which aspect failed. Do not re-run the import, because that
  creates a second artifact.
