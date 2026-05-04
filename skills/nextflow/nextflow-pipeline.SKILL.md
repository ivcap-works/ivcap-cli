---
name: nextflow-pipeline
version: 0.1.0
description: >
  Guide a researcher through assembling any Nextflow DSL2 pipeline
  entirely inside the agent environment, producing a pipeline definition file
  ready for upload and execution on an IVCAP platform.
requires:
  bins: ["ivcap"]
---

# Nextflow Pipeline Assembly Skill

This skill guides an agent to **interactively assemble** any Nextflow DSL2
pipeline with a researcher, build it entirely inside the agent's Linux environment,
and deliver a single downloadable `.tar.gz` file. The researcher uploads that to
their execution environment and runs it with one command.

**The researcher never writes code. The agent writes everything.**

---

## Two Deployment Approaches

### Approach 1: IVCAP-Native Workflow (Recommended)

Use `ivcap nextflow create` and `ivcap nextflow run` commands to deploy and execute
pipelines directly on an IVCAP platform. This approach:
- Automatically uploads the pipeline archive as an IVCAP artifact
- Creates/updates a service definition in the IVCAP Data Fabric
- Enables direct job submission via `ivcap nextflow run`
- **Requires** an `ivcap.yaml` manifest file in the archive

**How the internal commands behave (important):**
- `ivcap nextflow create/update` require an `ivcap.yaml` file in the tarball
- The file can be located anywhere in the tarball, but it must be **unique by basename** (do not include multiple `ivcap.yaml` files)
- `ivcap nextflow run` is an alias for `ivcap job create`, i.e. it submits a job to a service using either:
  - `-f job-params.json|yaml`, or
  - `-a aspect-urn` (an aspect containing the request payload)

**Key files required:**
- `main.nf` — Nextflow workflow
- `nextflow.config` — Configuration
- `ivcap.yaml` — **Required** service metadata, parameters, and sample table (see Phase 2A)
- `bin/` — Helper scripts

### Approach 2: Manual Deployment

Build a standalone `.tar.gz` that can be run anywhere with Nextflow installed.
This approach is covered in the original workflow (Phase 0–8 below).

**For IVCAP deployments, always prefer Approach 1.**

---

## Agent Mindset

You are a bioinformatics pipeline architect and instructor. The researcher describes
the biology; you translate it into Nextflow. Walk through the pipeline stage
by stage — explain the *why* before the *what*. Never dump all files at once.

Always build in `/home/claude/pipeline/` and deliver only one download: the
final tar.gz. Keep explanations to one concept at a time.

---

## ⚠️ CRITICAL: Container Requirements for IVCAP Execution

**IVCAP Nextflow pipelines MUST use containers, NOT conda environments.**

The IVCAP execution environment that launches Nextflow pipelines has **NO special Python libraries, bioinformatics tools, or conda installed**. It is a minimal runtime that only has Nextflow itself.

### ❌ DO NOT USE (will fail on IVCAP):

```groovy
process ANALYZE {
    conda 'conda-forge::biopython=1.83'
    // ❌ This will FAIL - conda is not available in IVCAP execution environment

    script:
    """
    python analyze.py
    """
}
```

### ✅ MUST USE (required for IVCAP):

```groovy
process ANALYZE {
    container 'quay.io/biocontainers/biopython:1.83'
    // ✅ This works - container has all dependencies pre-installed

    script:
    """
    python analyze.py
    """
}
```

### Why This Matters

When you run a pipeline on IVCAP:
1. IVCAP launches a minimal Nextflow executor environment
2. This environment has **only Nextflow** - no Python, R, conda, or bioinformatics tools
3. Each process runs in its own container with all required dependencies
4. Containers are pulled from registries (Docker Hub, Quay.io, etc.)

### Finding Container Images

Most bioinformatics tools have pre-built containers:
- **BioContainers**: `quay.io/biocontainers/<tool>:<version>`
  - Example: `quay.io/biocontainers/biopython:1.83`
  - Search at: https://biocontainers.pro/
- **Docker Hub**: `<organization>/<tool>:<version>`
  - Example: `broadinstitute/gatk:4.3.0.0`
- **Custom images**: Build and push your own if needed

### Local Development vs IVCAP Deployment

**For local development/testing** (not IVCAP), you can use:
- Conda environments (`conda` directive)
- Virtual environments (venv with `PATH` manipulation)
- Locally installed tools

**For IVCAP deployment** (production), you must use:
- Container images (`container` directive)
- All dependencies must be in the container

### Migration Pattern: Conda → Container

If you have a working local pipeline with conda:

```groovy
// Local development version (conda)
process MY_PROCESS {
    conda 'conda-forge::biopython=1.83 bioconda::samtools=1.15'
    // ...
}
```

Convert to container for IVCAP:

```groovy
// IVCAP version (container)
process MY_PROCESS {
    container 'quay.io/biocontainers/mulled-v2-<hash>'  // Contains both tools
    // Or use separate containers per process
    // container 'quay.io/biocontainers/biopython:1.83'
    // ...
}
```

**Note:** For multiple tools, use BioContainers' "mulled" images that combine multiple packages, or separate processes with individual containers.

### Best Practice for IVCAP Pipelines

**Every process that executes code must specify a container:**

```groovy
process QUALITY_CHECK {
    container 'quay.io/biocontainers/fastqc:0.11.9--0'

    input:
    path reads

    script:
    """
    fastqc ${reads}
    """
}

process TRIM_READS {
    container 'quay.io/biocontainers/trimmomatic:0.39--1'

    input:
    path reads

    script:
    """
    trimmomatic SE ${reads} trimmed.fastq LEADING:3 TRAILING:3
    """
}
```

**This is mandatory for IVCAP execution - there are no exceptions.**

---

## Phase 0 — Elicit Goals

Before writing any code, establish:

1. **What is the biological question?** — What goes in, what comes out?
2. **What are the processing steps?** — Each distinct step becomes a Process
3. **What tools are needed?** — Python, R, command-line tools (blast, samtools etc.)
4. **What is the execution target?** — Local venv, Conda, Docker, or SLURM?
5. **Will data be fetched automatically or provided by the user?**

Capture answers, state your plan, then proceed.

---

## Phase 1 — Standard Project Layout

All Nextflow pipelines follow this layout. Create it first:

```bash
mkdir -p /home/claude/pipeline/{bin,data,results}
```

```
pipeline/
├── main.nf            ← workflow definition (DSL2)
├── nextflow.config    ← resources, profiles, env
├── ivcap.yaml         ← IVCAP service metadata (for ivcap nextflow create)
├── environment.yml    ← conda environment (optional)
├── bin/               ← helper scripts; auto-added to PATH by Nextflow
├── data/              ← input data (may be empty if fetched at runtime)
└── results/           ← output directory (created at runtime)
```

**Key Nextflow conventions:**
- Scripts in `bin/` are automatically on PATH in every process — no absolute paths needed
- `publishDir` in a process copies outputs to `results/` for the user
- `params` at the top of `main.nf` are all overridable from the command line
- `work/` (created at runtime) holds all intermediate files — never committed to git

---

## Phase 2A — Write ivcap.yaml (IVCAP Deployments)

**This file is required when using `ivcap nextflow create`.**

The `ivcap.yaml` file provides service metadata and defines the pipeline’s **parameter model**
and (optionally) its **sample table model**.

The CLI will **auto-generate** the full service request schema from your `ivcap.yaml` definition.

### How `ivcap nextflow create` uses ivcap.yaml

The generated request payload has this overall shape:

```yaml
parameters: { ... }   # object with named parameters from `properties`
samples:              # optional; only if you define `samples` in ivcap.yaml
  - { ... }           # array of objects with named properties from `samples`
```

So **your `ivcap.yaml` must be explicit and complete**:
- The top-level `description` should be a detailed, multi-line description of the pipeline.
- Every entry in `properties` needs a clear description (units, defaults, valid ranges).
- If you use a sample table, every entry in `samples` must document that column.

### Complete ivcap.yaml Template

```yaml
$schema: urn:ivcap:schema.nextflow.pipeline.1
id: urn:sd-core:nextflow:my-pipeline
name: my-pipeline
service-id: urn:ivcap:service:a98b81a8-9279-509f-9c0e-40d39e83058a
description: |
  Detailed multi-line description of what this pipeline does.

  Explain:
  - The biological/scientific purpose
  - Input requirements
  - Expected outputs
  - Any important processing steps
  - Citations or references if applicable

contact:
  name: Your Name
  email: your.email@example.com

# --- Parameters Section ---
# Define all pipeline-level parameters here. These will be available in the
# 'parameters' object when submitting a job.
parameters:
  - name: min_read_length
    description: Minimum read length to retain after quality filtering (bp)
    type: integer
    optional: false

  - name: quality_threshold
    description: Phred quality score threshold for trimming
    type: integer
    optional: true

  - name: reference_genome
    description: Reference genome IVCAP artifact URN or external URL
    type: string
    format: uri
    optional: false

  - name: output_format
    description: Output file format
    type: string
    optional: true

# --- Samples Section ---
# Define the structure of each sample. Samples are submitted as an array of objects,
# where each object represents one sample with named properties (not positional arrays).
# This is typical for Nextflow pipelines where samples often come from CSV files.
samples:
  - name: sample_id
    description: Unique identifier for this sample
    type: string

  - name: read1_urn
    description: Forward read FASTQ file (IVCAP artifact URN or external URL)
    type: string
    format: uri

  - name: read2_urn
    description: Reverse read FASTQ file (IVCAP artifact URN or external URL)
    type: string
    format: uri

# --- Example Request ---
# Show a complete example job request. This helps users understand the expected format.
example:
  $schema: urn:ivcap:schema:my-pipeline.request.1
  parameters:
    min_read_length: 100
    quality_threshold: 20
    reference_genome: https://example.com/reference.fa
    output_format: bam
  samples:
    - sample_id: sample1
      read1_urn: https://example.com/sample1_R1.fastq.gz
      read2_urn: https://example.com/sample1_R2.fastq.gz
    - sample_id: sample2
      read1_urn: https://example.com/sample2_R1.fastq.gz
      read2_urn: https://example.com/sample2_R2.fastq.gz
```

### ivcap.yaml Field Reference

| Field | Required | Description |
|-------|----------|-------------|
| `$schema` | **Yes** | Schema identifier (use `urn:ivcap:schema.nextflow.pipeline.1`) |
| `id` | No | Unique identifier for this pipeline (e.g., `urn:sd-core:nextflow:my-pipeline`) |
| `name` | **Yes** | Short name (used in service description) |
| `service-id` | **Yes** | Service URN in format `urn:ivcap:service:<uuid>` |
| `description` | **Yes** | Detailed multi-line description of the pipeline |
| `contact` | No | Contact information (name, email) |
| `parameters` | **Yes** | Array of pipeline-level parameter definitions |
| `samples` | No | Array of sample field definitions (defines structure of sample objects) |
| `example` | No | Example request showing expected format |

### Parameter Definition Fields

Each entry in `properties`:

| Field | Required | Description |
|-------|----------|-------------|
| `name` | **Yes** | Parameter name (must be valid identifier) |
| `description` | **Yes** | Clear explanation of what this parameter controls |
| `type` | No | JSON Schema type: `string`, `integer`, `number`, `boolean`, `array`, `object` (default: `string`) |
| `format` | No | Format hint: `urn`, `uri`, `date-time`, etc. |
| `optional` | No | If `true`, parameter is optional (default: `false`) |

### Sample Field Definition

Each entry in `samples` defines one column in the sample table:

| Field | Required | Description |
|-------|----------|-------------|
| `name` | **Yes** | Field name (e.g., `sample_id`, `read1_urn`) |
| `description` | **Yes** | Explanation of this field |
| `type` | No | JSON Schema type (default: `string`) |
| `format` | No | Format hint: `urn` for IVCAP artifact references |

**Important:** Samples are submitted as an array of objects, where each object
has named properties matching the fields defined in the `samples` section.
This mirrors how Nextflow typically processes sample data from CSV files.

If you do **not** define `samples` in `ivcap.yaml`, then `samples` should be omitted
from the job request.

### Writing Effective Descriptions

**Pipeline description** should answer:
- What biological/scientific question does this address?
- What are the inputs? (formats, requirements)
- What are the outputs? (formats, locations)
- What tools/algorithms are used?
- Any citations or references?

**Parameter descriptions** should specify:
- What the parameter controls
- Valid value ranges or formats
- Default behavior if not provided (for optional params)
- Units where applicable (bp, seconds, percentage, etc.)

**Sample field descriptions** should explain:
- What data this field represents
- Expected format (URN, string ID, etc.)
- Whether it references an IVCAP artifact

### Example: Variant Calling Pipeline

```yaml
$schema: urn:ivcap:schema.nextflow.pipeline.1
id: urn:sd-core:nextflow:variant-calling-pipeline
name: variant-calling-pipeline
service-id: urn:ivcap:service:b1c2d3e4-5678-90ab-cdef-1234567890ab
description: |
  Germline variant calling pipeline using GATK HaplotypeCaller.

  This pipeline performs:
  1. Read alignment to reference genome (BWA-MEM)
  2. Duplicate marking (Picard)
  3. Base quality score recalibration (GATK BQSR)
  4. Variant calling (GATK HaplotypeCaller)
  5. Variant filtering (GATK VariantFiltration)

  Inputs:
  - Paired-end FASTQ files (Illumina)
  - Reference genome (FASTA with index)
  - Known variants VCF for BQSR

  Outputs:
  - Filtered VCF file (SNPs and indels)
  - BAM alignment files
  - QC metrics

  Reference: GATK Best Practices (Van der Auwera et al., 2013)

contact:
  name: Bioinformatics Core
  email: bioinfo@example.org

parameters:
  - name: reference_genome
    description: Reference genome FASTA file (IVCAP artifact URN or external URL)
    type: string
    format: uri
    optional: false

  - name: known_sites_vcf
    description: Known variants VCF for base recalibration (dbSNP, 1000G)
    type: string
    format: uri
    optional: false

  - name: min_mapping_quality
    description: Minimum mapping quality for reads to include (default 20)
    type: integer
    optional: true

  - name: min_base_quality
    description: Minimum base quality for variant calling (default 10)
    type: integer
    optional: true

  - name: ploidy
    description: Sample ploidy (default 2 for diploid)
    type: integer
    optional: true

samples:
  - name: sample_id
    description: Sample identifier (alphanumeric, no spaces)
    type: string

  - name: read1_fastq
    description: Forward read FASTQ (IVCAP artifact URN or external URL)
    type: string
    format: uri

  - name: read2_fastq
    description: Reverse read FASTQ (IVCAP artifact URN or external URL)
    type: string
    format: uri

  - name: sample_group
    description: Sample group/batch identifier (for joint calling)
    type: string

example:
  $schema: urn:ivcap:schema:variant-calling-pipeline.request.1
  parameters:
    reference_genome: urn:ivcap:artifact:hg38-ref-...
    known_sites_vcf: urn:ivcap:artifact:dbsnp-...
    min_mapping_quality: 20
    min_base_quality: 10
    ploidy: 2
  samples:
    - sample_id: SAMPLE001
      read1_fastq: urn:ivcap:artifact:s1r1...
      read2_fastq: urn:ivcap:artifact:s1r2...
      sample_group: batch1
    - sample_id: SAMPLE002
      read1_fastq: urn:ivcap:artifact:s2r1...
      read2_fastq: urn:ivcap:artifact:s2r2...
      sample_group: batch1
    - sample_id: SAMPLE003
      read1_fastq: urn:ivcap:artifact:s3r1...
      read2_fastq: urn:ivcap:artifact:s3r2...
      sample_group: batch2
```

---

## Phase 2B — Write main.nf

### DSL2 skeleton

Every `main.nf` follows this structure:

```groovy
#!/usr/bin/env nextflow
nextflow.enable.dsl = 2

// ── Parameters (all overridable via --param_name value) ──────
params.input   = "$projectDir/data/input.txt"
params.outdir  = "$projectDir/results"

// ── Workflow (orchestration) ──────────────────────────────────
workflow {
    log.info "Starting pipeline..."

    input_ch = Channel.fromPath(params.input)

    PROCESS_ONE(input_ch)
    PROCESS_TWO(PROCESS_ONE.out)
}

// ── Processes (one per biological step) ──────────────────────
process PROCESS_ONE {
    tag "$sample_id"
    publishDir "${params.outdir}", mode: 'copy'

    input:
    path input_file

    output:
    path "output.txt", emit: result

    script:
    """
    my_script.py --input ${input_file} --out output.txt
    """
}

workflow.onComplete {
    log.info "Done! Results in: ${params.outdir}"
}
```

### Channel patterns — choose the right one

| Situation | Pattern |
|---|---|
| One file input | `Channel.fromPath(params.input)` |
| CSV of samples | `Channel.fromPath(params.csv).splitCsv(header:true)` |
| Per-row tuple | `.map { row -> tuple(row.id, row.file) }` |
| Broadcast single file to all | `channel.first()` or use `val` input |
| Gather parallel results | `.collect()` before the next process |
| One value (not file) | `Channel.value(params.something)` |

### Common process patterns

**Parallel per-item processing:**
```groovy
process PER_SAMPLE {
    tag "$sample_id"   // labels each job in the log

    input:
    tuple val(sample_id), path(sample_file)

    output:
    tuple val(sample_id), path("${sample_id}.result"), emit: results
    ...
}
```

**Aggregation after parallel steps:**
```groovy
// Collect all parallel outputs then aggregate
AGGREGATE(PER_SAMPLE.out.results.collect())
```

**Conditional workflow branching:**
```groovy
if (params.mode == 'fetch') {
    FETCH_DATA(params.source)
    input_ch = FETCH_DATA.out.data
} else {
    input_ch = Channel.fromPath(params.input)
}
```

---

## Phase 3 — Write nextflow.config

See `skills://file/nextflow/references/config.md` for the full annotated template.

**Critical rules — these cause real errors if wrong:**

1. **PATH for venv** — use `$PWD`, not `$projectDir`, in `env` blocks:
   ```groovy
   env { PATH = "$PWD/.venv/bin:$PATH" }   // ✓
   ```

2. **report/timeline overwrite** — always set, or re-runs fail:
   ```groovy
   report   { enabled = true; file = "results/pipeline_report.html";   overwrite = true }
   timeline { enabled = true; file = "results/pipeline_timeline.html"; overwrite = true }
   ```

3. **`workflow.onComplete` belongs in `main.nf`**, not in `nextflow.config`

4. **`params` not accessible in config blocks** — use literal strings:
   ```groovy
   report { file = "results/report.html" }         // ✓
   // NOT: file = "${params.outdir}/report.html"   // ✗ — causes parse error
   ```

5. **Throttle parallel jobs** on laptops:
   ```groovy
   process { maxForks = 10 }
   ```

---

## Phase 4 — Write bin/ Scripts

Scripts go in `bin/`. They are plain Python (or R, bash, etc.) with a shebang.

**Python script template:**
```python
#!/usr/bin/env python3
"""
script_name.py — one line description.
Usage: script_name.py --input FILE --output FILE
"""
import argparse

def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("--input",  required=True)
    parser.add_argument("--output", required=True)
    args = parser.parse_args()
    # ... logic ...

if __name__ == "__main__":
    main()
```

After creating all scripts:
```bash
chmod +x /home/claude/pipeline/bin/*.py
```

**If scripts use f-strings containing JavaScript/shell `${}`**:
All `${}` must be escaped as `${{}}` inside Python f-strings, otherwise Python
raises `NameError`. This is a common trap when generating HTML with JS templates.

---

## Phase 5 — Specify Dependencies

### For IVCAP Deployment: Use Containers (Required)

**Every process must specify a `container` directive:**

```groovy
process ANALYZE_READS {
    container 'quay.io/biocontainers/biopython:1.83'

    input:
    path reads

    output:
    path "analysis.txt"

    script:
    """
    python analyze_reads.py ${reads}
    """
}

process QUALITY_CHECK {
    container 'quay.io/biocontainers/fastqc:0.11.9--0'

    input:
    path reads

    output:
    path "qc_report.html"

    script:
    """
    fastqc ${reads}
    """
}
```

**Where to find containers:**
- BioContainers: https://biocontainers.pro/
- Docker Hub: https://hub.docker.com/
- Search: `quay.io/biocontainers/<tool>:<version>`

### For Local Development Only: environment.yml (Optional)

**⚠️ This is ONLY for local testing, NOT for IVCAP deployment:**

```yaml
name: pipeline-env
channels:
  - conda-forge
  - bioconda
dependencies:
  - python=3.11
  - biopython=1.83    # adjust to your tools
  - pip
```

**Important:** If you use conda for local development, you must convert to containers before deploying to IVCAP. See the "Container Requirements" section above for migration patterns.

---

## Phase 6 — Validate Before Packaging

Run these checks inside the agent environment:

```bash
# All scripts present and executable
ls -la /home/claude/pipeline/bin/

# Python syntax check
for f in /home/claude/pipeline/bin/*.py; do
    python3 -m py_compile "$f" && echo "OK: $f" || echo "FAIL: $f"
done

# Shebang lines present
head -1 /home/claude/pipeline/bin/*.py

# main.nf has DSL2 header
head -3 /home/claude/pipeline/main.nf

# Config has no params.* references in report/timeline blocks
grep -n 'params\.' /home/claude/pipeline/nextflow.config && echo "WARNING: params in config blocks"
```

Fix all issues before packaging.

---

## Phase 7 — Package into tar.gz

```bash
PIPELINE_NAME="my-pipeline"   # set from context
TAR_PATH="/mnt/user-data/outputs/${PIPELINE_NAME}.tar.gz"

tar -czf "$TAR_PATH" \
    --transform "s|^pipeline|${PIPELINE_NAME}|" \
    -C /home/claude pipeline/

echo "Contents:"
tar -tzf "$TAR_PATH"
```

The `--transform` renames the root dir inside the tar so it unpacks as
`my-pipeline/` not `pipeline/`. Then present the file with `present_files`.

---

## Phase 8A — Deploy to IVCAP (IVCAP-Native Workflow)

If the researcher has an IVCAP deployment, use `ivcap nextflow create` to deploy
the pipeline as a service.

### Important: Service ID Requirement

**You must generate a service ID** in the format `urn:ivcap:service:<uuid>` where `<uuid>` is a valid UUIDv5 that you create. The caller is responsible for generating this service ID before calling `ivcap nextflow create`.

**Generate a UUIDv5 service ID** using a namespace and the pipeline name:
```python
import uuid

# Generate UUIDv5 from pipeline name
namespace = uuid.NAMESPACE_DNS  # or uuid.NAMESPACE_URL
service_uuid = uuid.uuid5(namespace, "my-pipeline-name")
service_id = f"urn:ivcap:service:{service_uuid}"
print(service_id)
# Example output: urn:ivcap:service:a98b81a8-9279-509f-9c0e-40d39e83058a
```

**Note:** This requirement for manual service ID generation may change in future versions to support auto-generation of service IDs.

### Step 1: Create/Update Service

```bash
# IMPORTANT: Generate service ID first (UUIDv5)
# Example: urn:ivcap:service:a98b81a8-9279-509f-9c0e-40d39e83058a

# Create new service (first time)
# The service-id MUST be in format 'urn:ivcap:service:<uuid>' where <uuid> is a valid UUID
ivcap nextflow create \
  --service-id "urn:ivcap:service:a98b81a8-9279-509f-9c0e-40d39e83058a" \
  -f /path/to/my-pipeline.tar.gz \
  --format json

# Update existing service (subsequent deployments)
ivcap nextflow update \
  "urn:ivcap:service:a98b81a8-9279-509f-9c0e-40d39e83058a" \
  -f /path/to/my-pipeline.tar.gz \
  --format json
```

**What this does:**
1. Uploads the pipeline archive as an IVCAP artifact
2. Extracts and parses `ivcap.yaml` from the archive
3. Generates the request schema (`fn-schema`) from `properties` + `samples` definitions
4. Creates/updates a Data Fabric aspect for the service
5. Returns the service URN, artifact URN, and aspect record ID

### Step 2: Submit Jobs

```bash
# Via inline JSON/YAML parameters file
ivcap nextflow run \
  "urn:ivcap:service:my-pipeline.1" \
  -f job-params.json \
  --watch \
  --stream

# Via aspect URN (pre-saved parameters)
ivcap nextflow run \
  "urn:ivcap:service:my-pipeline.1" \
  -a "urn:ivcap:aspect:job-params-..." \
  --watch \
  --stream
```

**Job parameters file format** (`job-params.json`):

```json
{
  "parameters": {
    "min_read_length": 100,
    "quality_threshold": 20,
    "reference_genome": "urn:ivcap:artifact:ref-..."
  },
  "samples": [
    {
      "sample_id": "sample1",
      "read1_urn": "urn:ivcap:artifact:read1-...",
      "read2_urn": "urn:ivcap:artifact:read2-..."
    },
    {
      "sample_id": "sample2",
      "read1_urn": "urn:ivcap:artifact:read1-...",
      "read2_urn": "urn:ivcap:artifact:read2-..."
    }
  ]
}
```

Or YAML format (`job-params.yaml`):

```yaml
parameters:
  min_read_length: 100
  quality_threshold: 20
  reference_genome: "urn:ivcap:artifact:ref-..."
samples:
  - sample_id: sample1
    read1_urn: "urn:ivcap:artifact:read1-..."
    read2_urn: "urn:ivcap:artifact:read2-..."
  - sample_id: sample2
    read1_urn: "urn:ivcap:artifact:read1-..."
    read2_urn: "urn:ivcap:artifact:read2-..."
```

### Step 3: Accessing Parameters in main.nf

The IVCAP platform injects parameters and samples into the Nextflow runtime as
JSON files. Reference them in `main.nf`:

```groovy
#!/usr/bin/env nextflow
nextflow.enable.dsl = 2

// IVCAP provides parameters via JSON input
def jobParams = null
if (params.containsKey('request_file')) {
    def jsonFile = file(params.request_file)
    def jsonSlurper = new groovy.json.JsonSlurper()
    jobParams = jsonSlurper.parse(jsonFile)
}

// Extract parameters
def minReadLength = jobParams?.parameters?.min_read_length ?: 100
def refGenome = jobParams?.parameters?.reference_genome

// Extract samples as channel (samples are objects with named properties)
def samplesData = jobParams?.samples ?: []
def samplesChannel = Channel.fromList(samplesData)
    .map { sample -> tuple(sample.sample_id, file(sample.read1_urn), file(sample.read2_urn)) }

workflow {
    log.info "Pipeline: my-pipeline"
    log.info "Min read length: ${minReadLength}"
    log.info "Reference: ${refGenome}"

    PROCESS_SAMPLES(samplesChannel, refGenome, minReadLength)
}

process PROCESS_SAMPLES {
    tag "$sample_id"
    publishDir "${params.outdir}", mode: 'copy'

    input:
    tuple val(sample_id), path(read1), path(read2)
    val reference
    val min_length

    output:
    path "${sample_id}.result"

    script:
    """
    echo "Processing ${sample_id}"
    echo "Read1: ${read1}"
    echo "Read2: ${read2}"
    echo "Reference: ${reference}"
    echo "Min length: ${min_length}"
    # ... actual processing ...
    """
}
```

### Flags for ivcap nextflow run

| Flag | Description |
|------|-------------|
| `-f FILE` | Job parameters file (JSON or YAML) |
| `-a URN` | Aspect URN containing job parameters |
| `--watch` | Wait for job completion and display final status |
| `--stream` | Stream job events (logs, progress) to stdout |
| `--format` | **Input file** format when using `-f` (`json` or `yaml`) |

**Pro tip:** Use `--watch --stream` together for real-time monitoring during development.

### Accessing Pipeline Results (CSV, TSV, JSON Files)

After a pipeline run completes successfully, you can fetch result files from the results artifact using the `artifact_get` MCP tool. Many Nextflow pipelines output tabular data as CSV or TSV files.

**Important:** Always include the appropriate `"accept"` parameter to get text files as readable content instead of base64-encoded data.

**Example: Fetching a CSV results file**
```json
{
  "tool": "artifact_get",
  "arguments": {
    "id": "urn:ivcap:artifact:results-...",
    "path": "results/differential_expression.csv",
    "accept": ["text/csv"]
  }
}
```

**Example: Fetching a TSV file**
```json
{
  "tool": "artifact_get",
  "arguments": {
    "id": "urn:ivcap:artifact:results-...",
    "path": "analysis/gene_counts.tsv",
    "accept": ["text/tsv"]
  }
}
```

**Supported accept formats:**
- `"accept": ["text/plain"]` - For log files, plain text files (also accepts CSV and TSV as plain text)
- `"accept": ["text/csv"]` - For CSV files specifically
- `"accept": ["text/tsv"]` - For TSV files specifically
- `"accept": ["text/*"]` - For any text-based format (universal fallback)
- `"accept": ["application/json"]` - For JSON files

**Important for MCP clients that cannot process base64:** If your client cannot handle base64-encoded blobs, **you must include an `accept` parameter** to get text files as readable content. Without the accept parameter, all files are returned as base64-encoded blobs regardless of their type.

**Note:** If your client can only accept `text/plain`, CSV and TSV files will also be returned as plain text since they are text-based formats.

**Without the accept parameter:** If your MCP client cannot process text content, the file will be returned as base64-encoded data, making it difficult to read and analyze directly.

---

## Phase 8B — Deliver Usage Instructions (Manual Deployment)

After presenting the download, give the researcher:

```bash
# 1. Unpack
tar -xzf my-pipeline.tar.gz
cd my-pipeline/

# 2. Python environment
python -m venv .venv
source .venv/bin/activate
pip install biopython   # or whatever the pipeline needs
chmod +x bin/*.py

# 3. Install Nextflow (if needed)
curl -s https://get.nextflow.io | bash

# 4. Run
nextflow run main.nf

# 5. Resume after failure
nextflow run main.nf -resume
```

**Show the key parameters** the researcher can tune with `--param_name value`.

---

## Phase 9 — Using MCP Tools for Pipeline Deployment

When working within an agent/automation context, use MCP tools (`nextflow_create`, `nextflow_run`) instead of CLI commands. This section provides comprehensive guidance on MCP tool usage.

### When to Use MCP Tools vs CLI

#### Use MCP Tools When:
- Deploying from within an agent/automation context
- Building pipelines programmatically
- Integrating with IVCAP Data Fabric aspects
- Need structured job tracking and artifact management
- Agent has access to MCP server

#### Use CLI When:
- Interactive terminal sessions
- Manual deployment and testing
- Shell scripts and CI/CD pipelines
- Direct human control needed

### MCP Tool Gotchas

⚠️ **Critical differences from CLI workflow:**

1. **Service ID generation is agent responsibility**
   - Generate UUIDv5: `uuid.uuid5(uuid.NAMESPACE_DNS, "pipeline-name")`
   - Format: `urn:ivcap:service:<uuid>`
   - Do NOT use random UUIDs - use namespace-based for reproducibility

2. **Two-step process is mandatory**
   - Step 1: `nextflow_create()` deploys service
   - Step 2: `nextflow_run()` submits jobs
   - Cannot run before service exists

3. **Sources must be fully formed**
   - Each source needs: `{path, type, [text|base64|url|artifact_id]}`
   - Do NOT omit the `type` field
   - `path` must include directory (e.g., `bin/script.py` not just `script.py`)
   - Use `type: "text"` for inline code, not `type: "url"` with local paths

### Sources Parameter Format

The `sources` array defines the pipeline package contents. Each source becomes a file in the tar.gz archive.

#### Source Types

**Type: text (for inline code)**
```json
{
  "path": "main.nf",
  "type": "text",
  "text": "#!/usr/bin/env nextflow\nnextflow.enable.dsl = 2\n..."
}
```
- Use for: Nextflow code, config files, Python scripts
- ✅ Preferred for MCP tool usage
- ❌ Do NOT use `type: "url"` with local file paths

**Type: base64 (for binary files)**
```json
{
  "path": "data/reference.fasta.gz",
  "type": "base64",
  "base64": "H4sIAAAAA...",
  "media_type": "application/gzip"
}
```

**Type: url (for external sources)**
```json
{
  "path": "bin/download_data.sh",
  "type": "url",
  "url": "https://example.com/scripts/download.sh"
}
```

**Type: artifact (for IVCAP artifacts)**
```json
{
  "path": "ivcap.yaml",
  "type": "artifact",
  "artifact_id": "urn:ivcap:artifact:...",
  "artifact_path": "pipeline/ivcap.yaml"
}
```

#### Required Source Fields per Type

| Type | path | type | Content Field | Optional |
|------|------|------|---|---|
| text | ✓ | ✓ | text | - |
| base64 | ✓ | ✓ | base64 | media_type |
| url | ✓ | ✓ | url | - |
| artifact | ✓ | ✓ | artifact_id, artifact_path | - |

#### Common Mistakes

❌ **Wrong:** Omitting `type` field
```json
{"path": "main.nf", "text": "..."}  // Missing type!
```

✅ **Correct:**
```json
{"path": "main.nf", "type": "text", "text": "..."}
```

---

❌ **Wrong:** Using `type: "url"` with local file paths
```json
{"path": "bin/script.py", "type": "url", "url": "file:///home/claude/..."}
```

✅ **Correct:** Inline content directly with `type: "text"`
```json
{"path": "bin/script.py", "type": "text", "text": "#!/usr/bin/env python3\n..."}
```

---

❌ **Wrong:** Omitting directory in path
```json
{"path": "analyze.py", "type": "text", "text": "..."}  // Should be bin/analyze.py
```

✅ **Correct:** Full path with directory structure
```json
{"path": "bin/analyze.py", "type": "text", "text": "..."}
```

### MCP Tool Execution Flow

```
Agent/User
    ↓
Generate service ID (UUIDv5)
    ↓
Prepare sources array
  - main.nf (type: text)
  - nextflow.config (type: text)
  - ivcap.yaml (type: text)
  - bin/*.py (type: text)
    ↓
Call nextflow_create()
  ✓ Pipeline artifact created
  ✓ Service registered in Data Fabric
  ✓ Returns: service_id, pipeline_artifact_urn
    ↓
Call nextflow_run()
  ✓ Job submitted to service
  ✓ Returns: job_id (if immediate)
  ✓ Returns: job_id + polling instructions (if slow)
    ↓
Call job_status() in loop
  ✓ Check execution progress
  ✓ Wait for completion
    ↓
Call artifact_get()
  ✓ Retrieve results from results_artifact_urn
  ✓ Use accept: ["text/csv"] for text content
    ↓
Complete
```

### Handling Long-Running Jobs

Most Nextflow pipelines complete within 30 seconds. If longer:

#### Pattern 1: Immediate Result (Fast Path)
```json
Response from nextflow_run():
{
  "job_id": "urn:ivcap:job:...",
  "status": "succeeded",
  "result": {
    "status": "succeeded",
    "results_artifact_urn": "urn:ivcap:artifact:..."
  }
}
```
✓ Job completed immediately
✓ Results are in `result.results_artifact_urn`

#### Pattern 2: Polling Required (Slow Path)
```json
Response from nextflow_run():
{
  "job_id": "urn:ivcap:job:...",
  "status": "executing",
  "poll_after_seconds": 30,
  "message": "Job still executing..."
}
```
⏳ Job is running
→ Call `job_status(job_id=...)` after 30 seconds
→ Repeat until status is "succeeded" or "failed"

#### Polling Code Pattern

```python
import time

job_id = response["job_id"]
poll_interval = response.get("poll_after_seconds", 30)

while True:
    status_response = job_status(job_id=job_id)
    if status_response["status"] in ["succeeded", "failed"]:
        break
    print(f"Still running... checking again in {poll_interval}s")
    time.sleep(poll_interval)
    poll_interval = status_response.get("poll_after_seconds", 30)

# Now get results
results_urn = status_response["result"]["results_artifact_urn"]
```

#### Important Notes
- ✅ Set appropriate `poll_after_seconds` (usually 30s)
- ✅ Always check both `.status` and `.result.status`
- ❌ Don't poll in tight loops (respect poll_after_seconds)
- ❌ Don't assume immediate completion

### Accessing Pipeline Results via MCP

Results are stored in IVCAP artifacts. Use `artifact_get()` to retrieve them.

#### Pattern: Fetch CSV Results
```python
# Get CSV with proper text formatting
csv_content = artifact_get(
    id="urn:ivcap:artifact:...",
    path="/results/results.csv",
    accept=["text/csv"]
)

# Parse
import csv
reader = csv.DictReader(csv_content.splitlines())
for row in reader:
    print(row)
```

#### Pattern: Fetch Text Report
```python
report = artifact_get(
    id="urn:ivcap:artifact:...",
    path="/results/report.txt",
    accept=["text/plain"]
)
print(report)
```

#### Pattern: List Artifact Contents
```python
# Without path, returns full tar.gz (may be large)
# Better: know expected paths from pipeline outputs
```

#### Critical: Use `accept` Parameter
- ✅ `accept: ["text/csv"]` → Returns readable CSV
- ✅ `accept: ["text/plain"]` → Returns readable text
- ✅ `accept: ["text/*"]` → Returns any text format
- ❌ Omit `accept` → Returns base64-encoded data (harder to parse)

#### Results Artifact Structure
Nextflow results artifact contains:
```
<job_uuid>/
  results/
    pipeline_report.html
    pipeline_timeline.html
    [your publishDir outputs]
  .nextflow.log
  work/
    [Nextflow work directory]
```

### Debugging Failed Jobs

#### Check Job Status
```python
response = job_status(job_id="urn:ivcap:job:...")
print(response["result"]["status"])  # "succeeded" or "failed"
results_urn = response["result"]["results_artifact_urn"]
```

#### Fetch Nextflow Log from Failed Job
```python
job_uuid = job_id.split(":")[-1]

log_content = artifact_get(
    id=results_urn,
    path=f"{job_uuid}/.nextflow.log",
    accept=["text/plain"]
)

# Look for errors
if "ERROR" in log_content:
    for line in log_content.split('\n'):
        if "ERROR" in line:
            print(line)
```

#### Common Failure Causes

| Issue | Solution |
|-------|----------|
| Container image not found | Verify image exists: `docker manifest inspect <image>` |
| Script file not found | Use `bin/script.py` path, not bare `script.py` |
| Missing `ivcap.yaml` | Ensure ivcap.yaml is in sources with `path: "ivcap.yaml"` |
| Malformed ivcap.yaml | Run YAML validator: `yaml.safe_load(ivcap_content)` |
| Parameter type mismatch | Check parameter types in ivcap.yaml match input |

### MCP Tools Quick Reference

#### nextflow_create()
| Parameter | Required | Type | Example |
|-----------|----------|------|---------|
| service_id | YES | string | `"urn:ivcap:service:fc51f603-1514-5dd0-a259-e7cb08970874"` |
| sources | YES | array | `[{path: "main.nf", type: "text", text: "..."}]` |
| name | NO | string | `"my-pipeline"` |
| collection | NO | string | `"urn:ivcap:collection:..."` |

#### nextflow_run()
| Parameter | Required | Type | Example |
|-----------|----------|------|---------|
| service_id | YES | string | `"urn:ivcap:service:fc51f603..."` |
| input | NO* | object | `{parameters: {top_variants: 30}}` |
| aspect_urn | NO* | string | `"urn:ivcap:aspect:..."` |
| watch | NO | boolean | `true` (wait for completion) |

*Either input or aspect_urn required

#### job_status()
| Parameter | Required | Type | Example |
|-----------|----------|------|---------|
| job_id | YES | string | `"urn:ivcap:job:13a363f5..."` |

#### artifact_get()
| Parameter | Required | Type | Example |
|-----------|----------|------|---------|
| id | YES | string | `"urn:ivcap:artifact:99ce0551..."` |
| path | NO | string | `"13a363f5.../results/output.csv"` |
| accept | NO | array | `["text/csv"]` |

---

## Common Errors Quick Reference

See `skills://file/nextflow/references/troubleshooting.md` for full details.

| Error | Fix |
|---|---|
| `command not found` (exit 127) | Use `$PWD/.venv/bin` in env PATH, not `$projectDir` |
| `report file already exists` | Add `overwrite = true` to report/timeline blocks |
| `Unknown config attribute report.params.*` | Use literal string in report.file, not `${params.*}` |
| `onComplete() not applicable` | Move `workflow.onComplete` to `main.nf`, not config |
| `first is useless on value channel` | Remove `.first()` when channel already emits one item |
| `params.fasta = "path" triggers wrong branch | Set optional params to `null`, not a path string |

---

## Reference Files

- `skills://file/nextflow/references/config.md` — Full annotated `nextflow.config` template
- `skills://file/nextflow/references/troubleshooting.md` — Detailed error catalogue with root causes
