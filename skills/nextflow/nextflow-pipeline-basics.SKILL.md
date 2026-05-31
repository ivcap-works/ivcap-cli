---
name: nextflow-pipeline-basics
version: 0.1.0
description: >
  Core Nextflow DSL2 pipeline assembly fundamentals: project structure,
  ivcap.yaml, main.nf, config, scripts, and container dependencies.
requires:
  bins: ["ivcap"]
---

# Nextflow Pipeline Basics

This skill covers the fundamental building blocks of Nextflow DSL2 pipelines for IVCAP deployment.

**For complete pipeline workflow, see also:**
- Packaging & Deployment: `skills://nextflow-pipeline-deployment/SKILL.md`
- MCP Tools Usage: `skills://nextflow-mcp-tools/SKILL.md`
- Production Best Practices: `skills://nextflow-production-practices/SKILL.md`

---

## Agent Mindset

You are a bioinformatics pipeline architect and instructor. The researcher describes
the biology; you translate it into Nextflow. Walk through the pipeline stage
by stage — explain the *why* before the *what*. Never dump all files at once.

Always build in `/home/claude/pipeline/` and deliver only one download: the
final tar.gz. Keep explanations to one concept at a time.

---

## ⚠️ CRITICAL: Container Requirements for IVCAP

**IVCAP Nextflow pipelines MUST use containers, NOT conda environments.**

The IVCAP execution environment has **NO special Python libraries, bioinformatics tools, or conda installed**. It is a minimal runtime that only has Nextflow itself.

### ❌ DO NOT USE (will fail on IVCAP):

```groovy
process ANALYZE {
    conda 'conda-forge::biopython=1.83'
    // ❌ This will FAIL - conda is not available
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
    // ✅ Container has all dependencies pre-installed
    script:
    """
    python analyze.py
    """
}
```

**For detailed container guidance, see:** `skills://nextflow-production-practices/SKILL.md`

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

## Phase 2A — Write ivcap.yaml

**This file is required when using `ivcap nextflow create`.**

The `ivcap.yaml` file defines the pipeline's **parameter model** and (optionally) its **sample table model**.

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
parameters:
  - name: min_read_length
    description: Minimum read length to retain after quality filtering (bp)
    type: integer
    optional: false

  - name: quality_threshold
    description: Phred quality score threshold for trimming
    type: integer
    optional: true

# --- Samples Section ---
samples:
  - name: sample_id
    description: Unique identifier for this sample
    type: string

  - name: read1_urn
    description: Forward read FASTQ file (IVCAP artifact URN or external URL)
    type: string
    format: uri

# --- Example Request ---
example:
  $schema: urn:ivcap:schema:my-pipeline.request.1
  parameters:
    min_read_length: 100
    quality_threshold: 20
  samples:
    - sample_id: sample1
      read1_urn: https://example.com/sample1_R1.fastq.gz
```

**For detailed ivcap.yaml documentation, see:** `skills://nextflow-production-practices/SKILL.md`

---

## Phase 2B — Write main.nf

### DSL2 skeleton

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
    container 'quay.io/biocontainers/tool:version'

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
    container 'quay.io/biocontainers/tool:version'

    input:
    tuple val(sample_id), path(sample_file)

    output:
    tuple val(sample_id), path("${sample_id}.result"), emit: results

    script:
    """
    process_sample.py ${sample_id} ${sample_file}
    """
}
```

**Aggregation after parallel steps:**
```groovy
// Collect all parallel outputs then aggregate
AGGREGATE(PER_SAMPLE.out.results.collect())
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

**Important:** For IVCAP deployment, scripts must be invoked explicitly with their interpreter (e.g., `python script.py`) not just `script.py`. See `skills://nextflow-production-practices/SKILL.md` for details.

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
```

**Where to find containers:**
- BioContainers: https://biocontainers.pro/
- Docker Hub: https://hub.docker.com/
- Search: `quay.io/biocontainers/<tool>:<version>`

**For container verification and best practices, see:** `skills://nextflow-production-practices/SKILL.md`

---

## Next Steps

After completing these basics:
- **Package and Deploy**: `skills://nextflow-pipeline-deployment/SKILL.md`
- **Use MCP Tools**: `skills://nextflow-mcp-tools/SKILL.md`
- **Production Practices**: `skills://nextflow-production-practices/SKILL.md`
- **Debugging**: `skills://nextflow-debugging/SKILL.md`
