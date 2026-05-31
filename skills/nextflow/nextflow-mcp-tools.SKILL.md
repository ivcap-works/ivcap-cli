---
name: nextflow-mcp-tools
version: 0.2.0
description: >
  Using MCP tools for Nextflow pipeline deployment and execution, including
  artifact_build for creating pipeline packages and troubleshooting.
requires:
  bins: ["ivcap"]
---

# Nextflow MCP Tools Usage

This skill covers using MCP tools (`artifact_build`, `nextflow_create`, `nextflow_run`) for programmatic pipeline deployment and execution.

**See also:**
- Pipeline Basics: `skills://nextflow-pipeline-basics/SKILL.md`
- Deployment: `skills://nextflow-pipeline-deployment/SKILL.md`
- MCP Debugging: `skills://nextflow-mcp-debugging/SKILL.md`

---

## ⚠️ CRITICAL: Nextflow Deployment Workflow

The `nextflow_create` tool **NO LONGER accepts inline sources**. You must first build and upload an artifact using the `artifact_build` tool, then deploy it using `nextflow_create`.

### Required Two-Step Workflow

**⚠️ CRITICAL: Do NOT Pre-Tar Your Files**

The `artifact_build` tool **automatically tars your files for you**. You must:

- ❌ **DON'T** Package files → create tar.gz → upload tar
- ✅ **DO** Call init → add individual files → call submit

The tool will handle all tarring and compression internally. Provide individual files (main.nf, nextflow.config, ivcap.yaml, etc.) and `artifact_build` will package them into a tar.gz archive.

---

**Step 1: Build and Upload Pipeline Package**
Use `artifact_build` to create a tar.gz containing your pipeline files:

```json
// 1a. Initialize build session
{
  "tool": "artifact_build",
  "arguments": {
    "stage": "init"
  }
}
```
Returns: `{"id": "session-uuid-123...", ...}`

```json
// 1b. Add pipeline files incrementally
{
  "tool": "artifact_build",
  "arguments": {
    "stage": "add",
    "id": "session-uuid-123...",
    "files": [
      {
        "path_name": "main.nf",
        "content": "IyEvdXNyL2Jpbi9lbnYgbmV4dGZsb3cKbmV4dGZsb3cuZW5hYmxlLmRzbCA9IDIKCndvcmtmbG93IHsKICAgIGxvZy5pbmZvICJIZWxsbyBmcm9tIE5leHRmbG93ISIKfQo=",
        "mime_type": "text/x-nextflow"
      },
      {
        "path_name": "nextflow.config",
        "content": "cGFyYW1zLm91dGRpciA9ICIke3BhcmFtcy5yZXN1bHRzX2RpcjovdG1wfS9yZXN1bHRzIgo=",
        "mime_type": "text/plain"
      },
      {
        "path_name": "ivcap.yaml",
        "content": "bmFtZTogbXktcGlwZWxpbmUKZGVzY3JpcHRpb246IEEgc2FtcGxlIE5leHRmbG93IHBpcGVsaW5lCnBhcmFtZXRlcnM6CiAgc2FtcGxlX2NvdW50OgogICAgdHlwZTogaW50ZWdlcgogICAgZGVzY3JpcHRpb246IE51bWJlciBvZiBzYW1wbGVzIHRvIHByb2Nlc3MKICAgIGRlZmF1bHQ6IDUK",
        "mime_type": "application/x-yaml"
      }
    ]
  }
}
```

```json
// 1c. Submit to create artifact
{
  "tool": "artifact_build",
  "arguments": {
    "stage": "submit",
    "id": "session-uuid-123...",
    "name": "my-pipeline-v1.0"
  }
}
```
Returns: `{"id": "urn:ivcap:artifact:abc123...", ...}`

**Step 2: Deploy Pipeline Service**
Use `nextflow_create` to deploy the artifact as a service:

```json
{
  "tool": "nextflow_create",
  "arguments": {
    "service_id": "urn:ivcap:service:a98b81a8-9279-509f-9c0e-40d39e83058a",
    "artifact_id": "urn:ivcap:artifact:abc123..."
  }
}
```

---

## Complete Deployment Example

```python
import uuid
import base64

# Generate service ID (UUIDv5 from pipeline name)
namespace = uuid.NAMESPACE_DNS
service_uuid = uuid.uuid5(namespace, "my-pipeline")
service_id = f"urn:ivcap:service:{service_uuid}"

# Step 1: Initialize build session
init_response = artifact_build(stage="init")
session_id = init_response["id"]

# Step 2: Add pipeline files
pipeline_files = {
    "main.nf": """#!/usr/bin/env nextflow
nextflow.enable.dsl = 2

workflow {
    log.info "Processing samples..."
}
""",
    "nextflow.config": """
params.outdir = "${params.results_dir:/tmp}/results"
""",
    "ivcap.yaml": """
name: my-pipeline
description: My Nextflow pipeline
parameters:
  sample_count:
    type: integer
    description: Number of samples to process
    default: 5
"""
}

# Add files to the build session
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

# Step 3: Submit to create artifact
submit_result = artifact_build(
    stage="submit",
    id=session_id,
    name="my-pipeline-v1.0"
)
artifact_id = submit_result["id"]

# Step 4: Deploy the pipeline service
deploy_result = nextflow_create(
    service_id=service_id,
    artifact_id=artifact_id
)

print(f"Service deployed: {deploy_result['service_id']}")
print(f"Artifact: {deploy_result['pipeline_artifact_urn']}")
```

---

## Using Pre-Built Artifacts (CLI Upload)

If you've already uploaded an artifact using the CLI, you can deploy it directly:

```bash
# Upload artifact via CLI
ivcap artifact upload my-pipeline.tar.gz \
  --name "my-pipeline-v1.0" \
  --output json > artifact.json

ARTIFACT_ID=$(jq -r '.id' artifact.json)
```

Then deploy via MCP:

```json
{
  "tool": "nextflow_create",
  "arguments": {
    "service_id": "urn:ivcap:service:...",
    "artifact_id": "urn:ivcap:artifact:..."
  }
}
```

---

## When to Use MCP Tools vs CLI

### Use MCP Tools When:
- Deploying from within an agent/automation context
- Building pipelines programmatically
- Integrating with IVCAP Data Fabric aspects
- Need structured job tracking and artifact management
- Agent has access to MCP server

### Use CLI When:
- Interactive terminal sessions
- Manual deployment and testing
- Shell scripts and CI/CD pipelines
- Direct human control needed
- Working with local tar.gz files

---

## Choosing Between artifact_create and artifact_build

### ⚠️ CRITICAL: When to Use Which Tool

**Use `artifact_build` when:**
- ✅ You are assembling files **in your agent's sandbox** (e.g., `/home/claude/`, `/tmp/`)
- ✅ You are creating files programmatically and need to upload them
- ✅ Building multi-file archives from content you generate
- ✅ Files don't exist in a location accessible to the MCP server

**Use `artifact_create` when:**
- ✅ Files already exist on the **MCP server's filesystem** (not your sandbox)
- ✅ Referencing URLs that the MCP server can access (http://, https://)
- ✅ Using existing artifacts by URN

### Common Mistake: Using artifact_create with Sandbox Files

❌ **This will FAIL:**
```json
{
  "tool": "artifact_create",
  "arguments": {
    "content": [
      {
        "name": "my-pipeline.tar.gz",
        "source": {
          "type": "url",
          "url": "file:///home/claude/my-pipeline.tar.gz"
        }
      }
    ]
  }
}
```
**Why:** The MCP server cannot access files in your agent's sandbox (`/home/claude/`).

✅ **Use artifact_build instead:**
```json
// 1. Initialize
{"tool": "artifact_build", "arguments": {"stage": "init"}}

// 2. Read and add files from your sandbox
{"tool": "artifact_build", "arguments": {
  "stage": "add",
  "id": "session-id",
  "files": [{"path_name": "main.nf", "content": "base64...", "mime_type": "text/plain"}]
}}

// 3. Submit
{"tool": "artifact_build", "arguments": {"stage": "submit", "id": "session-id"}}
```

---

## Choosing Between artifact_build 'add' vs 'add_remote' Stages

### ⚠️ CRITICAL: Size Constraints and RPC Patterns

The `artifact_build` tool has two ways to add files:

#### 'add' Stage: For Small Files (Base64)
- ✅ **Use when:** File content fits in RPC payload (typically < 1MB)
- ✅ **How it works:** You encode the file content as base64 and send it inline
- ✅ **Best for:** Source code files (main.nf, config, scripts < 1MB)
- ❌ **DON'T use for:** Large data files, reference genomes, archives
- **Protocol:** `{"path_name": "file.txt", "content": "base64-encoded-content"}`

#### 'add_remote' Stage: For Large Files (URL Download)
- ✅ **Use when:** Files are large or already hosted at URLs
- ✅ **How it works:** You provide a URL; MCP server downloads the file directly
- ✅ **Best for:** Large data files, binaries, archives already on web
- ✅ **Avoids:** Encoding/decoding overhead, RPC payload limits
- ❌ **DON'T use for:** Small files that fit easily in base64
- **Protocol:** `{"path_name": "data.tar.gz", "url": "https://..."}`

### Key Differences

| Aspect | 'add' (Base64) | 'add_remote' (URL) |
|--------|----------------|-------------------|
| **File Size** | Small (< 1MB typical) | Large (unlimited) |
| **Content Encoding** | Base64 (3:4 overhead) | Binary (no overhead) |
| **Network** | RPC payload | Direct HTTP download |
| **Best For** | Source code, configs | Data, archives, binaries |
| **Example** | `main.nf`, `ivcap.yaml` | `reference-genome.tar.gz` |

### Recommended Pattern

```python
# For a mixed pipeline package:

# 1. Add source code files (small) with 'add'
artifact_build(
    stage="add",
    id=session_id,
    files=[
        {"path_name": "main.nf", "content": base64_encode(main_nf_content)},
        {"path_name": "ivcap.yaml", "content": base64_encode(ivcap_yaml_content)},
    ]
)

# 2. Add large data files with 'add_remote'
artifact_build(
    stage="add_remote",
    id=session_id,
    files=[
        {"path_name": "data/reference.tar.gz", "url": "https://example.com/reference-genome.tar.gz"},
        {"path_name": "data/annotation.gff", "url": "https://example.com/genes.gff"},
    ]
)

# 3. Submit when all files are added
artifact_build(
    stage="submit",
    id=session_id,
    name="my-pipeline-v1.0"
)
```

---

## Verifying URLs Before Download

When using `add_remote` to download files from URLs, you can use the `verify_url` tool to check if a URL is accessible before attempting the download. This is particularly useful for:

- Checking that remote files exist before adding them to artifacts
- Verifying HTTP status codes and availability
- Inspecting content metadata (Content-Type, Content-Length) without downloading the full file
- Building robust agent workflows that validate URLs before processing
- **Validating URLs in generated pipeline code before deployment** (CRITICAL for Nextflow)

### ⚠️ CRITICAL: Verify URLs in Generated Nextflow Code

When you **generate or create Nextflow pipeline code** that includes hardcoded download URLs (in `bin/` scripts, within process definitions, or in configuration files), you **MUST verify those URLs before deploying the pipeline**:

1. **Before Creating Pipeline Artifacts:** Use `verify_url` to confirm:
   - The URL actually exists and is accessible
   - The Content-Type matches expectations (e.g., `application/gzip` for `.tar.gz`)
   - The Content-Length matches the expected file size (validates the URL points to the correct version)

2. **During Code Generation:** If your agent generates download scripts like:
   ```bash
   # Inside a Nextflow process
   wget https://example.com/reference-genome-v2.0.tar.gz
   ```
   Verify the URL BEFORE adding this code to the artifact.

3. **Before artifact_build submission:** Use this pattern:
   ```python
   # Extract all URLs from generated code
   urls_in_code = extract_urls_from_pipeline_code(generated_main_nf)

   # Verify each URL exists and has correct metadata
   for url in urls_in_code:
       verify_result = verify_url(url=url)
       if not verify_result.get("success"):
           raise ValueError(f"URL verification failed: {url}")

       # Validate content-type and content-length expectations
       if expected_content_type and verify_result.get("content_type") != expected_content_type:
           raise ValueError(f"Unexpected content-type for {url}")

   # Only proceed with artifact creation after all URLs verified
   artifact_build(stage="submit", id=session_id, name="pipeline")
   ```

### Why This Matters for Nextflow

Nextflow pipelines often hardcode external data URLs. If these URLs are:
- ❌ **Typos or outdated:** Pipeline jobs will fail with cryptic download errors
- ❌ **Redirects to wrong version:** Pipeline results will be wrong without obvious cause
- ❌ **Content-Type mismatch:** Scripts may fail trying to decompress wrong file type

By verifying upfront:
- ✅ Catch URL problems **before deploying** the pipeline service
- ✅ Validate **Content-Type** matches what the pipeline expects
- ✅ Confirm **Content-Length** to detect version mismatches
- ✅ Agents can self-correct and regenerate code with correct URLs


### verify_url Tool

```json
{
  "tool": "verify_url",
  "arguments": {
    "url": "https://example.com/reference-genome.tar.gz"
  }
}
```

**Returns:**
```json
{
  "url": "https://example.com/reference-genome.tar.gz",
  "success": true,
  "status_code": 200,
  "status": "200 OK",
  "content_type": "application/gzip",
  "content_length": "5368709120",
  "last_modified": "Mon, 01 Jan 2024 00:00:00 GMT",
  "etag": "\"abc123\""
}
```

### Usage Pattern

```python
# Before adding a remote file, verify it exists
verify_result = verify_url(url="https://example.com/data.tar.gz")

if verify_result.get("success"):
    # URL is accessible, proceed with artifact_build
    artifact_build(
        stage="add_remote",
        id=session_id,
        files=[{
            "path_name": "data.tar.gz",
            "url": "https://example.com/data.tar.gz",
            "size": int(verify_result.get("content_length", 0))
        }]
    )
else:
    # URL is not accessible, handle error
    print(f"Error accessing URL: {verify_result.get('error')}")
    print(f"Status code: {verify_result.get('status_code')}")
```

### Key Fields

| Field | Purpose |
|-------|---------|
| **success** | Boolean indicating HTTP < 400 status code |
| **status_code** | HTTP status code (200, 404, 500, etc.) |
| **status** | HTTP status message (e.g., "200 OK") |
| **content_type** | MIME type of the resource |
| **content_length** | Size in bytes (useful for validating expected file size) |
| **last_modified** | Last modification timestamp |
| **etag** | Entity tag for cache validation |

### Error Handling

When a URL is not accessible, `verify_url` returns:

```json
{
  "url": "https://example.com/missing-file.tar.gz",
  "success": false,
  "status_code": 404,
  "status": "404 Not Found",
  "error": "HTTP 404: Not Found"
}
```

---


## MCP Tool Reference

### artifact_build()

Creates and uploads a tar.gz artifact incrementally **from content you provide**.

**Stage: init**
```json
{
  "tool": "artifact_build",
  "arguments": {
    "stage": "init"
  }
}
```
Returns: `{"id": "session-uuid", "staging_dir": "/tmp/...", "created_at": "..."}`

**Stage: add** (For Small Files - Base64 Encoded)
```json
{
  "tool": "artifact_build",
  "arguments": {
    "stage": "add",
    "id": "session-uuid",
    "files": [
      {
        "path_name": "path/to/file.txt",
        "content": "base64-encoded-content",
        "mime_type": "text/plain",
        "size": 1234
      }
    ]
  }
}
```
Returns: `{"id": "session-uuid", "files_added": 1, "total_files": 5, ...}`

**⚠️ Note on 'add' stage:**
- Use for small files that fit in RPC payload (typically < 1MB)
- Content must be base64-encoded
- Has 3:4 size overhead from base64 encoding
- Best for source code: main.nf, nextflow.config, ivcap.yaml, scripts

**Stage: add_remote** (For Large Files - URL Download)
```json
{
  "tool": "artifact_build",
  "arguments": {
    "stage": "add_remote",
    "id": "session-uuid",
    "files": [
      {
        "path_name": "data/reference-genome.tar.gz",
        "url": "https://example.com/genomes/reference-v2.tar.gz",
        "mime_type": "application/gzip",
        "size": 5368709120
      }
    ]
  }
}
```
Returns: `{"id": "session-uuid", "files_added": 1, "total_files": 5, ...}`

**⚠️ Note on 'add_remote' stage:**
- Use for large files (> 1MB) or files already hosted at URLs
- MCP server downloads the file directly from the provided URL
- No base64 encoding overhead
- No RPC payload size limits
- Best for: large data files, binaries, reference genomes, archives
- URL must be publicly accessible or have proper auth headers
- Optional `mime_type` is inferred from Content-Type header if omitted
- Optional `size` parameter validates downloaded content matches expected size

**Stage: list**
```json
{
  "tool": "artifact_build",
  "arguments": {
    "stage": "list",
    "id": "session-uuid"
  }
}
```
Returns: `{"id": "session-uuid", "file_count": 5, "total_size": 12345, "files": [...]}`

**Stage: submit**
```json
{
  "tool": "artifact_build",
  "arguments": {
    "stage": "submit",
    "id": "session-uuid",
    "name": "my-artifact",
    "policy": "optional-policy-id"
  }
}
```
Returns: `{"id": "urn:ivcap:artifact:...", "name": "...", "size": 12345, ...}`

### nextflow_create()

Deploys a Nextflow pipeline from a pre-built artifact.

| Parameter | Required | Type | Description |
|-----------|----------|------|-------------|
| service_id | YES | string | Service URN (e.g., `urn:ivcap:service:a98b81a8-9279-509f-9c0e-40d39e83058a`) |
| artifact_id | YES | string | Artifact URN containing the pipeline tar.gz |
| name | NO | string | Optional name for display purposes |
| policy | NO | string | Optional access policy |

**Example:**
```json
{
  "tool": "nextflow_create",
  "arguments": {
    "service_id": "urn:ivcap:service:a98b81a8-9279-509f-9c0e-40d39e83058a",
    "artifact_id": "urn:ivcap:artifact:abc123...",
    "name": "My Pipeline"
  }
}
```

**Returns:**
```json
{
  "ok": true,
  "service_id": "urn:ivcap:service:...",
  "pipeline_artifact_urn": "urn:ivcap:artifact:...",
  "service_aspect_record_id": "urn:ivcap:aspect:...",
  "tool": {
    "name": "my-pipeline",
    "description": "...",
    "service_id": "...",
    "source": "ivcap.yaml"
  }
}
```

### nextflow_run()

Runs a Nextflow pipeline job.

| Parameter | Required | Type | Description |
|-----------|----------|------|-------------|
| service_id | YES | string | Service URN to run |
| input | NO* | object | Inline job input payload (JSON object) |
| aspect_urn | NO* | string | URN of aspect containing job parameters |

*Either input or aspect_urn required

**Example:**
```json
{
  "tool": "nextflow_run",
  "arguments": {
    "service_id": "urn:ivcap:service:...",
    "input": {
      "parameters": {
        "sample_count": 10
      }
    }
  }
}
```

---

## Handling Long-Running Jobs

Most Nextflow pipelines complete within 30 seconds. If longer:

### Pattern 1: Immediate Result (Fast Path)
```json
Response from nextflow_run():
{
  "job_id": "urn:ivcap:job:...",
  "status": "succeeded",
  "result": {
    "status": "succeeded",
    "log_urn": "urn:ivcap:artifact:...",
    "output_urn": "urn:ivcap:artifact:...",
    "results": {
      "process_name": "urn:ivcap:artifact:..."
    }
  }
}
```
✓ Job completed immediately
✓ **New format:** Results are now split into separate artifacts:
  - `log_urn`: Direct access to Nextflow execution log (no extraction needed!)
  - `output_urn`: Pipeline output directory
  - `results`: Per-process results (map of process names to artifact URNs)

### Pattern 2: Polling Required (Slow Path)
```json
Response from nextflow_run():
{
  "job_id": "urn:ivcap:job:...",
  "status": "executing",
  "message": "Job still executing...",
  "_meta": {
    "job_id": "urn:ivcap:job:...",
    "status": "executing",
    "poll_after_seconds": 30
  }
}
```
⏳ Job is running
→ Call `job_status(job_id=...)` after 30 seconds (use `_meta.poll_after_seconds`)
→ Repeat until status is "succeeded" or "failed"

---

## Accessing Pipeline Results via MCP

Results are stored in IVCAP artifacts. Use `artifact_get()` to retrieve them.

### ⚠️ CRITICAL: Always Use `accept` Parameter for LLM-Friendly Responses

When calling `artifact_get`, **always specify the `accept` parameter** to get plain text responses instead of Base64-encoded blobs. This is essential for agent/LLM workflows because:

- ✅ **With `accept`:** Response is flat, readable text (LLM-friendly)
- ❌ **Without `accept`:** Response contains nested Base64 blobs (cognitive overhead for LLMs)

### Pattern: Fetch and Parse Results

**For Text Files (Logs, Config, etc.):**
```python
# ✅ CORRECT: Use accept parameter for plain text
artifact_content = artifact_get(
    id="urn:ivcap:artifact:...",
    path="/logs/pipeline.log",
    accept=["text/plain"]
)

# Content is now returned as plain text - easy to parse!
# You can directly work with the content
for line in artifact_content.splitlines():
    if "ERROR" in line:
        print(f"Found error: {line}")
```

**For CSV Results:**
```python
# ✅ CORRECT: Use accept with text/csv
csv_content = artifact_get(
    id="urn:ivcap:artifact:...",
    path="/results/results.csv",
    accept=["text/csv"]
)

# Parse CSV directly from plain text response
import csv
reader = csv.DictReader(csv_content.splitlines())
for row in reader:
    print(row)
```

**For JSON Results:**
```python
# ✅ CORRECT: Use accept with application/json
json_content = artifact_get(
    id="urn:ivcap:artifact:...",
    path="/results/data.json",
    accept=["application/json"]
)

# Parse directly from the response
import json
data = json.loads(json_content)
print(data["key"])
```

### Response Structure with `accept`

When using `accept` parameter, the response is optimized for readability:

```json
{
  "content": [
    {
      "type": "text",
      "text": "May-05 10:35:01.011 [main] DEBUG nextflow.cli.Launcher - Setting http proxy...\nProcessing sample 1...\nCompleted successfully"
    }
  ],
  "isError": false
}
```

Extract the content with:
```python
response = artifact_get(id="...", path="...", accept=["text/plain"])
# response is already the plain text content - use it directly!
for line in response.splitlines():
    process(line)
```

### ⚠️ What NOT to Do

❌ **This pattern is LLM-unfriendly:**
```python
# WRONG: Omitting accept parameter returns Base64
artifact_content = artifact_get(
    id="urn:ivcap:artifact:...",
    path="/results/results.csv"
    # No accept parameter!
)

# Response is nested Base64 blob:
# {
#   "result": {
#     "content": [
#       {
#         "resource": {
#           "blob": "VGhpcyBpcyBCYXNlNjQgZW5jb2RlZCBkYXRhLi4u"
#         }
#       }
#     ]
#   }
# }

# LLMs struggle to navigate this nested structure and decode Base64!
```

### Accept Parameter Values

| Accept Type | Best For | Returns |
|-----------|----------|---------|
| `["text/plain"]` | Logs, text files, any readable text | Plain text |
| `["text/csv"]` | CSV files, tabular data | Plain text CSV |
| `["text/json"]` or `["application/json"]` | JSON files | Plain JSON text |
| `["text/*"]` | Any text format | Plain text |
| (omitted) | Binary files only | Base64-encoded blob |

### Troubleshooting artifact_get with LLMs

**Problem:** Agent says "I can't find the content" or returns Base64 strings

**Solution:**
1. Always include `accept` parameter matching the file type
2. Verify file path exists in the artifact
3. Use `artifact_get(id="...", path="/")` to list contents if unsure of path

**Example: Debugging artifact contents**
```python
# List what's in the artifact
list_result = artifact_get(
    id="urn:ivcap:artifact:...",
    path="/",  # List root directory
    accept=["text/plain"]
)

# Now you know what files exist
# Then fetch specific file with correct path
```


---

## Troubleshooting

### Error: "missing artifact_id"

**Cause:** You're trying to use the old workflow with inline sources.

**Solution:** Use the two-step workflow:
1. Build artifact with `artifact_build`
2. Deploy with `nextflow_create` using the artifact ID

### Error: "neither 'ivcap.yaml' nor 'ivcap-tool.yaml' found"

**Cause:** Your artifact doesn't contain the required tool descriptor file.

**Solution:** Ensure your `artifact_build` session includes `ivcap.yaml` or `ivcap-tool.yaml`:
```json
{
  "stage": "add",
  "id": "session-id",
  "files": [
    {
      "path_name": "ivcap.yaml",
      "content": "base64-encoded-yaml-content"
    }
  ]
}
```

### Error: "failed to download artifact"

**Cause:** Invalid artifact ID or authentication issue.

**Solution:**
- Verify the artifact ID is correct (format: `urn:ivcap:artifact:...`)
- Check authentication with `ivcap context list`

### Error: "nextflow_create failed" or "no main.nf found in archive"

**Cause:** You pre-tarred your files before uploading them.

**❌ This is WRONG:**
```python
# WRONG: Don't do this!
import tarfile
import base64

# Create tar.gz first
tar_data = create_tar_gz([
    ("main.nf", main_nf_content),
    ("ivcap.yaml", ivcap_yaml_content),
])

# Then try to upload it as a single file
artifact_build(
    stage="add",
    id=session_id,
    files=[{
        "path_name": "my-pipeline.tar.gz",  # ❌ WRONG!
        "content": base64.b64encode(tar_data).decode(),
    }]
)
```
**Why it fails:** The tar.gz file is treated as-is, not extracted. When `nextflow_create` tries to find `main.nf` inside, it finds `my-pipeline.tar.gz/my-pipeline/main.nf` or nothing at all.

**✅ This is CORRECT:**
```python
# CORRECT: Add individual files, artifact_build tars them
artifact_build(
    stage="add",
    id=session_id,
    files=[
        {
            "path_name": "main.nf",
            "content": base64.b64encode(main_nf_content.encode()).decode(),
        },
        {
            "path_name": "ivcap.yaml",
            "content": base64.b64encode(ivcap_yaml_content.encode()).decode(),
        },
    ]
)

# artifact_build automatically creates the tar.gz with correct structure
# nextflow_create finds main.nf at the root of the archive
```

**Key Point:**
- ❌ **DON'T** package files → create tar.gz → upload tar
- ✅ **DO** add individual files → let artifact_build tar them

---

## Next Steps

- **MCP Debugging**: `skills://nextflow-mcp-debugging/SKILL.md`
- **Production Practices**: `skills://nextflow-production-practices/SKILL.md`
- **General Debugging**: `skills://nextflow-debugging/SKILL.md`
