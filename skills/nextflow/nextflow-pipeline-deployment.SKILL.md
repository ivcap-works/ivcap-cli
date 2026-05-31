---
name: nextflow-pipeline-deployment
version: 0.1.0
description: >
  Validation, packaging, and deployment of Nextflow pipelines to IVCAP,
  including CLI and manual deployment workflows.
requires:
  bins: ["ivcap"]
---

# Nextflow Pipeline Deployment

This skill covers validation, packaging into tar.gz, and deployment of Nextflow pipelines to IVCAP.

**See also:**
- Pipeline Basics: `skills://nextflow-pipeline-basics/SKILL.md`
- MCP Tools Usage: `skills://nextflow-mcp-tools/SKILL.md`
- Debugging: `skills://nextflow-debugging/SKILL.md`

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

### Deployment Method 1: CLI (Recommended for Manual Deployment)

**Step 1: Create/Update Service**

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

### Deployment Method 2: MCP Tools (Recommended for Agents/Automation)

**IMPORTANT:** The MCP `nextflow_create` tool **NO LONGER accepts inline sources**. You must first build and upload an artifact using the `artifact_build` tool.

#### ⚠️ Critical: artifact_create vs artifact_build

If you are an agent assembling files in your sandbox (e.g., `/home/claude/`, `/tmp/`):
- ❌ **DO NOT** use `artifact_create` with file URLs like `file:///home/claude/pipeline.tar.gz`
- ✅ **DO** use `artifact_build` to upload content from your sandbox
- **Why:** The MCP server cannot access files in your agent's sandbox

#### ⚠️ CRITICAL: Do NOT Pre-Tar Your Files

The `artifact_build` tool **automatically tars your files for you**:

- ❌ **DON'T** Package files → create tar.gz → upload tar
- ✅ **DO** Call init → add individual files → call submit

The tool will handle all tarring and compression internally. Provide individual files (main.nf, nextflow.config, ivcap.yaml, etc.) and `artifact_build` will package them into a tar.gz archive before uploading.

**Step 1: Build and Upload Pipeline Artifact**

```python
import uuid
import base64

# Generate service ID (UUIDv5 from pipeline name)
namespace = uuid.NAMESPACE_DNS
service_uuid = uuid.uuid5(namespace, "my-pipeline")
service_id = f"urn:ivcap:service:{service_uuid}"

# Initialize build session
init_response = artifact_build(stage="init")
session_id = init_response["id"]

# Add pipeline files
pipeline_files = {
    "main.nf": "#!/usr/bin/env nextflow\nnextflow.enable.dsl = 2\n...",
    "nextflow.config": "params.outdir = ...",
    "ivcap.yaml": "name: my-pipeline\n..."
}

for path, content in pipeline_files.items():
    content_b64 = base64.b64encode(content.encode()).decode()
    artifact_build(
        stage="add",
        id=session_id,
        files=[{
            "path_name": path,
            "content": content_b64,
            "mime_type": "text/plain"
        }]
    )

# Submit to create artifact
submit_result = artifact_build(
    stage="submit",
    id=session_id,
    name="my-pipeline-v1.0"
)
artifact_id = submit_result["id"]
```

**Step 2: Deploy Pipeline Service**

```python
# Deploy the pipeline using the artifact
deploy_result = nextflow_create(
    service_id=service_id,
    artifact_id=artifact_id
)

print(f"Service deployed: {deploy_result['service_id']}")
print(f"Artifact: {deploy_result['pipeline_artifact_urn']}")
```

**Alternative: Use Pre-Built Artifact**

If you've already uploaded an artifact via CLI:

```bash
# Upload artifact
ivcap artifact upload my-pipeline.tar.gz \
  --name "my-pipeline-v1.0" \
  --output json > artifact.json

ARTIFACT_ID=$(jq -r '.id' artifact.json)
```

Then deploy via MCP:

```python
deploy_result = nextflow_create(
    service_id="urn:ivcap:service:...",
    artifact_id="urn:ivcap:artifact:..."
)
```

### Running Jobs (Both CLI and MCP)

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

### Step 3: Monitor Running Jobs

Stream events for a running job:

```bash
# Stream events in real-time
ivcap nextflow events <service-id> <job-id>

# Limit number of events
ivcap nextflow events --max-messages 50 <service-id> <job-id>

# Resume from a specific event
ivcap nextflow events --last-event-id <event-id> <service-id> <job-id>
```

The `events` command displays job progress, logs, and status updates as they occur. Use `ivcap job events` for the generic equivalent outside the `nextflow` subcommand group.

---

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

## Next Steps

After deployment:
- **Using MCP Tools**: `skills://nextflow-mcp-tools/SKILL.md`
- **Debugging**: `skills://nextflow-debugging/SKILL.md`
- **Production Practices**: `skills://nextflow-production-practices/SKILL.md`
