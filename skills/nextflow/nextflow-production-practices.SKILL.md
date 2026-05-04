---
name: nextflow-production-practices
version: 0.1.0
description: >
  Production-ready Nextflow pipeline best practices including container requirements,
  script invocation, samples schemas, and provenance tracking for IVCAP.
requires:
  bins: ["ivcap"]
---

# Nextflow Production Practices

This skill covers production-ready Nextflow pipeline development with emphasis on reusability, provenance tracking, and comprehensive documentation.

**See also:**
- Pipeline Basics: `skills://nextflow-pipeline-basics/SKILL.md`
- Examples: `skills://nextflow-examples/SKILL.md`
- Debugging: `skills://nextflow-debugging/SKILL.md`

---

## ⚠️ CRITICAL: IVCAP Pipelines MUST Use Containers

**All Nextflow pipelines deployed to IVCAP MUST use container directives, NOT conda.**

The IVCAP execution environment has **no Python, R, bioinformatics tools, or conda installed**.
It provides only Nextflow itself. Every process must specify its dependencies via containers.

### ✅ Required for IVCAP:

```groovy
process ANALYZE {
    container 'quay.io/biocontainers/biopython:1.83'
    // ✅ All dependencies are in the container

    script:
    """
    python analyze.py
    """
}
```

### ❌ Will NOT work on IVCAP:

```groovy
process ANALYZE {
    conda 'conda-forge::biopython=1.83'
    // ❌ conda is not available - this will fail

    script:
    """
    python analyze.py
    """
}
```

**Key points:**
- **Every process** that runs code must have a `container` directive
- Find containers at https://biocontainers.pro/ or Docker Hub
- Use `quay.io/biocontainers/<tool>:<version>` for most bioinformatics tools
- This is **mandatory** - pipelines without containers will fail on IVCAP

---

## 🔍 Verifying Container Images Before Use

**Before specifying any container directive, verify the image exists and is pullable.**

Container images can be removed, have incorrect tags, or be unavailable. Always verify before adding to your pipeline:

### Step 1: Search for Available Tags

Search the web for the exact image tag:
- For BioContainers: `site:quay.io/repository/biocontainers/<tool>`
- For Docker Hub: `site:hub.docker.com <tool>`

Example search: `site:quay.io/repository/biocontainers/python` to find tags like `quay.io/biocontainers/python:3.14`

### Step 2: Verify Image Exists

Before using an image, confirm it's accessible using one of these methods:

**Option A: Dry-run pull (Docker)**
```bash
docker pull <image>:<tag> --dry-run
```

**Option B: Inspect manifest (preferred - no download)**
```bash
docker manifest inspect <image>:<tag>
```

**Example:**
```bash
# Verify BioContainers image exists
docker manifest inspect quay.io/biocontainers/biopython:1.83

# Verify standard Docker Hub image
docker manifest inspect python:3.11-slim
```

### Step 3: Prefer Well-Known Stable Images

**Priority order:**
1. **Official Docker Hub images** - Always current, never deleted
   - ✅ `docker.io/library/python:3.11-slim`
   - ✅ `docker.io/library/ubuntu:22.04`
   - ✅ `docker.io/library/r-base:4.3.0`

2. **BioContainers with verified tags** - Versioned but may be deprecated
   - ⚠️ `quay.io/biocontainers/biopython:1.83` (verify first!)

3. **Custom images** - Only if you maintain them
   - ⚠️ Your own registries

**Avoid guessing tags!** A tag that looks right may not exist.

### ⚠️ CRITICAL: Use Fully Qualified Image Names for Docker Hub

IVCAP's Kubernetes environment **prepends `quay.io/` to unqualified container names**. If you specify a container as just `python:3.11-slim`, it will be transformed to `quay.io/python:3.11-slim`, which doesn't exist and will fail to pull.

**Always use fully qualified names for Docker Hub images:**
- ✅ `docker.io/library/python:3.11-slim` (correct - fully qualified)
- ❌ `python:3.11-slim` (wrong - will become `quay.io/python:3.11-slim`)

This applies to all official Docker Hub images: `ubuntu`, `alpine`, `node`, `postgres`, etc.

**Examples:**
```groovy
// ✅ CORRECT - Fully qualified Docker Hub images
process ANALYZE {
    container 'docker.io/library/python:3.11-slim'
    // ...
}

process BUILD {
    container 'docker.io/library/node:18-alpine'
    // ...
}

// ❌ WRONG - Unqualified names (will fail on IVCAP)
process ANALYZE {
    container 'python:3.11-slim'
    // This becomes quay.io/python:3.11-slim ❌
}

// ✅ CORRECT - Quay.io images can remain as-is
process BIOINFORMATICS {
    container 'quay.io/biocontainers/samtools:1.17'
    // Already fully qualified ✅
}
```

### Step 4: Fallback Pattern for Simple Dependencies

If you only need Python + pip-installable packages (like Biopython + curl), the simplest verified approach is:

```groovy
process ANALYZE {
    container 'docker.io/library/python:3.11-slim'
    // ✅ Official image, always available (fully qualified for IVCAP)

    beforeScript 'apt-get update && apt-get install -y procps && rm -rf /var/lib/apt/lists/*'

    script:
    """
    pip install biopython==1.83
    python analyze.py
    """
}
```

### ⚠️ CRITICAL: Install procps in beforeScript for Nextflow Metrics

When using plain language containers (like `python:3.11-slim`, `ubuntu:22.04`, etc.), **always install the `procps` package in the `beforeScript` directive**. Nextflow checks for the `ps` command **before** running your process script to collect CPU and memory metrics. If procps is installed inside the script block, it's too late—Nextflow has already failed the check.

**Always use beforeScript for procps:**
```groovy
process MY_PROCESS {
    container 'docker.io/library/python:3.11-slim'

    beforeScript 'apt-get update && apt-get install -y procps && rm -rf /var/lib/apt/lists/*'

    script:
    """
    # Your actual processing code here
    python analyze.py
    """
}
```

**Why beforeScript is required:**
- ✅ Nextflow checks for `ps` command availability **before** executing the script block
- ✅ `beforeScript` runs first, ensuring procps is installed when Nextflow checks
- ❌ Installing in `script` block is too late—Nextflow has already checked and failed

This applies to:
- ✅ `docker.io/library/python:*-slim` images
- ✅ `docker.io/library/ubuntu:*` images
- ✅ `docker.io/library/debian:*` images
- ✅ Any minimal base image that doesn't include procps

BioContainers typically already include procps, but plain language images often don't.

**Trade-offs:**
- ✅ Image always exists and is current
- ✅ No verification needed
- ⚠️ Slower startup (apt-get + pip install at runtime)
- ⚠️ Less reproducible (pip may fetch different dependencies)

**When to use this:**
- Quick prototypes and testing
- Simple dependencies available on PyPI
- When BioContainers verification fails

**Production recommendation:** Build and maintain your own verified container with pinned dependencies (and procps pre-installed).

---

## ⚠️ CRITICAL: Invoking Scripts in bin/ Directory

When using custom scripts in the `bin/` directory of your Nextflow pipeline, **file permissions don't carry through into Kubernetes pods**. Even if you mark scripts as executable locally, they won't have execute permissions in the container.

### ❌ Wrong: Relying on Execute Permissions

```groovy
// bin/analyze.py (with chmod +x locally)
process ANALYZE {
    container 'docker.io/library/python:3.11-slim'

    script:
    """
    analyze.py input.txt output.txt
    # ❌ Will fail: "Permission denied" - execute bit not preserved
    """
}
```

### ✅ Correct: Explicit Interpreter Invocation

**Always explicitly invoke scripts with their interpreter:**

```groovy
process ANALYZE {
    container 'docker.io/library/python:3.11-slim'

    script:
    """
    python ${projectDir}/bin/analyze.py input.txt output.txt
    # ✅ Works: Explicitly calls python interpreter
    """
}
```

**For different script types:**

```groovy
// Python scripts
script:
"""
python ${projectDir}/bin/my_script.py args
"""

// R scripts
script:
"""
Rscript ${projectDir}/bin/my_script.R args
"""

// Bash scripts
script:
"""
bash ${projectDir}/bin/my_script.sh args
"""

// Perl scripts
script:
"""
perl ${projectDir}/bin/my_script.pl args
"""
```

**Why this matters:**
- Kubernetes pods don't preserve file permissions from the pipeline package
- The execute bit (`chmod +x`) is lost when files are mounted into the container
- Explicit interpreter invocation works reliably across all environments
- `${projectDir}/bin/` is automatically added to PATH by Nextflow, but permissions still don't work

**Best practice:**
```groovy
process COMPLEX_ANALYSIS {
    container 'docker.io/library/python:3.11-slim'

    beforeScript 'apt-get update && apt-get install -y procps && rm -rf /var/lib/apt/lists/*'

    input:
    path input_file

    output:
    path 'results.txt'

    script:
    """
    # ✅ Explicit invocation for all custom scripts
    python ${projectDir}/bin/preprocess.py ${input_file} temp.txt
    python ${projectDir}/bin/analyze.py temp.txt results.txt
    """
}
```

---

## Why Samples Schemas Matter

### The Problem: Ad-Hoc Input Handling

Many pipelines are built for immediate use with specific datasets:

```groovy
// ❌ Bad: Hard-coded paths, no schema
params.input1 = "/data/sample1_R1.fastq"
params.input2 = "/data/sample1_R2.fastq"
params.reference = "/data/hg38.fa"
```

**Issues:**
- Not reusable with different data
- No provenance tracking (IVCAP can't track what data was used)
- No validation (wrong data types silently fail)
- Unclear what inputs are expected

### The Solution: Well-Defined Samples Schema

```yaml
# ✅ Good: Structured samples with schema
samples:
  - name: sample_id
    description: Unique identifier for this sample
    type: string

  - name: read1_fastq
    description: Forward read FASTQ file (IVCAP artifact URN or external URL)
    type: string
    format: uri

  - name: read2_fastq
    description: Reverse read FASTQ file (IVCAP artifact URN or external URL)
    type: string
    format: uri
```

**Benefits:**
- ✅ **Reusable:** Works with any dataset matching the schema
- ✅ **Provenance:** IVCAP automatically tracks all input artifacts
- ✅ **Validated:** Type checking prevents runtime errors
- ✅ **Documented:** Self-describing interface

---

## Build Checklist

When building any Nextflow pipeline for IVCAP, ensure:

### 1. ✅ Samples Schema is Always Defined

**Rule:** Even if your pipeline processes a single file or fixed dataset, define
the samples schema in `ivcap.yaml`.

**Why:**
- Future reusability (you or others may want to run it on different data)
- IVCAP can track data lineage automatically
- Enables batch processing if needed later
- Makes the pipeline self-documenting

**Example:** Single-file analysis pipeline

```yaml
# Even for a "one-off" analysis, define the schema
samples:
  - name: input_file
    description: Input data file to analyze (CSV, TSV, or Excel format)
    type: string
    format: uri

  - name: file_format
    description: Format of the input file (csv, tsv, xlsx)
    type: string
```

### 2. ✅ Descriptions are Detailed and Comprehensive

**Rule:** The pipeline `description` in `ivcap.yaml` must be multi-line and
answer key questions about the pipeline.

**Template for Pipeline Description:**

```yaml
description: |
  [One-line summary of what this pipeline does]

  ## Purpose
  [Explain the biological/scientific question this pipeline addresses]

  ## Inputs
  [Describe all inputs with formats and requirements]

  ## Processing Steps
  [Outline the major processing stages]
  1. [Step 1]
  2. [Step 2]
  3. [Step 3]

  ## Outputs
  [Describe outputs with formats and locations]

  ## Dependencies
  [List key tools/algorithms with versions]

  ## References
  [Citations or links to papers/documentation]

  ## Notes
  [Any important caveats, limitations, or recommendations]
```

### 3. ✅ Parameter Descriptions Include Units and Ranges

**Rule:** Every parameter must have a description that specifies:
- What it controls
- Valid ranges or formats
- Units (bp, seconds, percentage, etc.)
- Default behavior if optional

**Example:**

```yaml
parameters:
  - name: min_read_length
    description: |
      Minimum read length to retain after quality filtering.
      Reads shorter than this threshold will be discarded.
      Recommended range: 50-150 bp for Illumina short reads.
      Units: base pairs (bp)
    type: integer
    optional: false

  - name: quality_threshold
    description: |
      Phred quality score threshold for trimming.
      Bases with quality below this value will be trimmed from read ends.
      Range: 0-40 (typical: 20 for Q20 or 30 for Q30)
      Default: 20 if not specified
    type: integer
    optional: true
```

### 4. ✅ Sample Field Descriptions Explain Format and Purpose

**Rule:** Each sample field must document:
- What data it represents
- Expected format (URN, string ID, number, etc.)
- Whether it's an IVCAP artifact reference
- Any validation requirements

**Example:**

```yaml
samples:
  - name: sample_id
    description: |
      Unique identifier for this sample.
      Must be alphanumeric with no spaces (underscores allowed).
      Example: SAMPLE_001, patient_42, control_A
    type: string

  - name: read1_fastq
    description: |
      Forward read FASTQ file from paired-end sequencing.
      Must be gzip-compressed (.fastq.gz or .fq.gz).
      Provide as IVCAP artifact URN (urn:ivcap:artifact:...) or
      external URL (https://...).
    type: string
    format: uri
```

---

## Building with Provenance in Mind

### What is Provenance Tracking?

IVCAP automatically records:
- Which artifacts were used as inputs
- What parameters were specified
- When the pipeline was executed
- Who executed it
- What outputs were produced

**This only works if inputs are structured as samples with URN references.**

### Provenance-Friendly Pattern

```yaml
# ivcap.yaml
samples:
  - name: input_data
    description: Input dataset (IVCAP artifact URN)
    type: string
    format: uri

  - name: reference_database
    description: Reference database (IVCAP artifact URN)
    type: string
    format: uri
```

```groovy
// main.nf - Extract samples from IVCAP request
def jobParams = null
if (params.containsKey('request_file')) {
    def jsonFile = file(params.request_file)
    def jsonSlurper = new groovy.json.JsonSlurper()
    jobParams = jsonSlurper.parse(jsonFile)
}

def samplesData = jobParams?.samples ?: []
def samplesChannel = Channel.fromList(samplesData)
    .map { sample ->
        tuple(
            sample.sample_id,
            file(sample.input_data),        // IVCAP resolves URN → file
            file(sample.reference_database)
        )
    }
```

**When this pipeline runs:**
- IVCAP records: "Job X used artifact:abc123 and artifact:def456"
- Lineage graph shows: Input artifacts → Pipeline → Output artifacts
- Reproducibility: Anyone can see exactly what data was used

### Anti-Pattern: Hard-Coded Paths

```groovy
// ❌ No provenance tracking possible
params.input = "/data/myfile.csv"
input_ch = Channel.fromPath(params.input)
```

**IVCAP cannot track this because:**
- No artifact URN reference
- No structured samples
- Provenance is lost

---

## Validation Checklist

Before deploying your pipeline, verify:

### Documentation Quality

- [ ] Pipeline description is multi-line and comprehensive
- [ ] Description includes: Purpose, Inputs, Steps, Outputs, Dependencies, References
- [ ] Every parameter has a detailed description
- [ ] Parameter descriptions include units, ranges, and defaults
- [ ] Every sample field has a clear description
- [ ] Sample field descriptions explain format and validation requirements

### Schema Completeness

- [ ] Samples schema is defined (even for single-file pipelines)
- [ ] All input data files are referenced through sample fields
- [ ] Sample fields use `format: uri` for artifact references
- [ ] Parameter types are appropriate (`string`, `integer`, `number`, `boolean`)
- [ ] Optional parameters are marked with `optional: true`

### Provenance Readiness

- [ ] All data inputs come through samples (not hard-coded paths)
- [ ] Artifact references use URN format or URLs
- [ ] No direct file paths in `params` defaults
- [ ] Pipeline can process different datasets without code changes

### Example Request

- [ ] Example request is provided in `ivcap.yaml`
- [ ] Example demonstrates typical use case
- [ ] Example shows both parameters and samples
- [ ] Example uses realistic URNs or URLs

### Testing

- [ ] Test with actual IVCAP artifact URNs
- [ ] Verify provenance is recorded correctly
- [ ] Test with different sample counts (1, 2, many)
- [ ] Validate error messages for invalid inputs

---

## Common Mistakes to Avoid

### ❌ Mistake 1: No Samples Schema

```yaml
# Wrong: Only parameters, no samples
parameters:
  - name: input_file
    description: Input file path
    type: string
```

**Why it's wrong:**
- No provenance tracking
- Not reusable
- No type safety

**Fix:**
```yaml
# Correct: Use samples for data inputs
parameters:
  - name: analysis_mode
    description: Analysis mode (fast or thorough)
    type: string

samples:
  - name: input_file
    description: Input data file (IVCAP artifact URN)
    type: string
    format: uri
```

### ❌ Mistake 2: Vague Descriptions

```yaml
# Wrong: Vague description
description: This pipeline processes data.
```

**Fix:**
```yaml
# Correct: Detailed description
description: |
  RNA-seq differential expression analysis using DESeq2.

  ## Purpose
  Identify differentially expressed genes between experimental conditions...
  [continue with full details]
```

### ❌ Mistake 3: Missing Units/Ranges

```yaml
# Wrong: No units or ranges
parameters:
  - name: threshold
    description: Quality threshold
    type: number
```

**Fix:**
```yaml
# Correct: Include units and ranges
parameters:
  - name: quality_threshold
    description: |
      Phred quality score threshold (0-40).
      Recommended: 20 (Q20) or 30 (Q30).
      Default: 20
    type: integer
    optional: true
```

### ❌ Mistake 4: Hard-Coded Data

```groovy
// Wrong: Hard-coded paths in main.nf
params.reference = "/data/hg38.fa"
params.samples = "/data/samples.csv"
```

**Fix:**
```yaml
# ivcap.yaml - Accept data as samples
samples:
  - name: reference_genome
    description: Reference genome FASTA (IVCAP artifact URN)
    type: string
    format: uri
```

```groovy
// main.nf - Extract from IVCAP request
def refGenome = jobParams?.samples[0]?.reference_genome
def refFile = file(refGenome)
```

---

## Next Steps

- **Examples**: `skills://nextflow-examples/SKILL.md`
- **Basics**: `skills://nextflow-pipeline-basics/SKILL.md`
- **Debugging**: `skills://nextflow-debugging/SKILL.md`
