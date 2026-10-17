---
name: ivcap-ncbi-genome-download
version: 0.1.0
description: Look up an NCBI genome assembly and download the data package of one genome to a safe local path.
requires:
  bins: ["ivcap"]
---

# Skill: Look up and download a genome from NCBI (agent-safe)

These commands talk to NCBI Datasets directly. They need no IVCAP context and no login.

## Best practices
- Always use `--output json` when you will parse the result.
- A download covers **exactly one genome**. Pass one **assembly accession** (`GCF_...` or `GCA_...`).
  A BioProject accession (`PRJNA...`) is rejected for downloads, because a project can name many genomes.
- Not sure which assembly you want? Look it up first with `ncbi genome reports` (it accepts BioProjects).
- A download **writes a local file**. Echo the command you plan to run and confirm with the user first.
- Prefer the default destination (`./<ACCESSION>.zip` in the working directory) or an explicit relative path.
  Do **not** use `../` or absolute paths unless explicitly requested.
- The command refuses to replace an existing file. Use `--force` only when the user asked to overwrite.
- NCBI rate-limits requests. If the user has an NCBI API key, pass it through `NCBI_API_KEY`
  (or `--ncbi-api-key`).

## Steps

### 1) Find the assembly

```bash
ivcap --output json ncbi genome reports GCF_000001735.4
ivcap --output json ncbi genome reports PRJNA10719 --reference
```

Each entry is a genome assembly report. Pick the one you want and note its `accession`
(for example `GCF_000001735.4`).

### 2) Download its package

```bash
ivcap --output json ncbi genome download GCF_000001735.4 --include genome,gff3,seq-report -f ./GCF_000001735.4.zip
```

`--include` takes a comma-separated list of: `genome`, `rna`, `protein`, `cds`, `gff3`, `gtf`, `gbff`,
`seq-report`, `all`, `none`. The default is `genome`. Annotation files only exist for annotated assemblies.

### 3) Read the result

The JSON result has `accession`, `file`, `bytes`, an optional `warning`, and a `manifest` listing every
file in the package as `{name, size, compressed_size, crc32}`. Use the manifest to check that the files
you asked for are present.

## Errors
- `a download needs exactly one genome assembly` - the request named a BioProject, several assemblies, or
  matched several versions. Pass one assembly accession (with its version) chosen via `ncbi genome reports`.
- `no genome assemblies match the request` - check the accession and version, then look it up again.
- `... already exists; use --force to overwrite it` - choose another path, or ask the user before using `--force`.

## Related
To store the genome in IVCAP instead of a local file, use `skills://ivcap-ncbi-genome-import/SKILL.md`.
