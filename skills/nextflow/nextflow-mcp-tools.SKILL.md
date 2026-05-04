---
name: nextflow-mcp-tools
version: 0.1.0
description: >
  Using MCP tools for Nextflow pipeline deployment and execution, including
  artifact_build for large pipelines, source types, size limits, and troubleshooting.
requires:
  bins: ["ivcap"]
---

# Nextflow MCP Tools Usage

This skill covers using MCP tools (`nextflow_create`, `nextflow_run`, `artifact_build`) for programmatic pipeline deployment and execution.

**See also:**
- Pipeline Basics: `skills://nextflow-pipeline-basics/SKILL.md`
- Deployment: `skills://nextflow-pipeline-deployment/SKILL.md`
- MCP Debugging: `skills://nextflow-mcp-debugging/SKILL.md`

---

## ⚠️ CRITICAL: Handling Large Pipelines (Best Practice)

When your pipeline has many files (>5 files) or large total size (>50KB), **DO NOT inline everything in a single `nextflow_create` call**. This causes:
- Request payloads too large (>100KB typically fails)
- Token limit exhaustion in MCP calls
- Parser timeouts on massive nested JSON
- Silent failures without error messages

### Anti-Pattern: Inline Everything ❌

```json
{
  "tool": "nextflow_create",
  "arguments": {
    "service_id": "urn:ivcap:service:...",
    "sources": [
      {"path": "main.nf", "type": "text", "text": "... 5KB ..."},
      {"path": "nextflow.config", "type": "text", "text": "... 2KB ..."},
      {"path": "ivcap.yaml", "type": "text", "text": "... 8KB ..."},
      {"path": "bin/analyze.py", "type": "text", "text": "... 8KB ..."},
      {"path": "bin/fetch.py", "type": "text", "text": "... 6KB ..."},
      {"path": "bin/process.py", "type": "text", "text": "... 12KB ..."},
      // ... 7 more Python files inlined (50KB+ more)
    ]
  }
}
```

**Result:** ❌ Request too large, timeout, silent failure

### Pattern 1: Use `artifact_build` MCP Tool (Recommended for Agents) ✅

The `artifact_build` MCP tool provides **incremental building** for large pipelines:

**Step 1: Initialize build session**
```json
{
  "tool": "artifact_build",
  "arguments": {
    "stage": "init"
  }
}
```
Returns: `{"session_id": "uuid-abc123...", "message": "..."}`

**Step 2: Add files incrementally (multiple calls)**
```json
{
  "tool": "artifact_build",
  "arguments": {
    "stage": "add",
    "session_id": "uuid-abc123...",
    "path_name": "main.nf",
    "content_base64": "IyEvdXNyL2Jpbi9lbnYg...",
    "mime_type": "text/x-nextflow"
  }
}
```

Repeat for each file. Each call is small (<10KB per file).

**Step 3: List staged files (optional verification)**
```json
{
  "tool": "artifact_build",
  "arguments": {
    "stage": "list",
    "session_id": "uuid-abc123..."
  }
}
```

**Step 4: Submit the artifact**
```json
{
  "tool": "artifact_build",
  "arguments": {
    "stage": "submit",
    "session_id": "uuid-abc123...",
    "name": "p53-pipeline-v1.0"
  }
}
```
Returns: `{"artifact_id": "urn:ivcap:artifact:xyz...", ...}`

**Step 5: Deploy pipeline using the artifact**
```json
{
  "tool": "nextflow_create",
  "arguments": {
    "service_id": "urn:ivcap:service:...",
    "sources": [
      {
        "path": ".",
        "type": "artifact",
        "artifact_id": "urn:ivcap:artifact:xyz..."
      }
    ]
  }
}
```

**Advantages:**
- ✅ No size limits (handles 100+ files)
- ✅ Each call is small and fast
- ✅ Can verify with `list` before submit
- ✅ Reproducible (same artifact always)
- ✅ Session-based (can resume if interrupted)

### Pattern 2: Pre-Build Locally and Upload (Recommended for CLI) ✅

**For production pipelines**, always build locally:

```bash
# 1. On your system (or in agent bash_tool):
cd /path/to/pipeline
tar -czf p53-pipeline.tar.gz \
  main.nf \
  nextflow.config \
  ivcap.yaml \
  bin/*.py

# 2. Upload to IVCAP as artifact
ivcap artifact upload p53-pipeline.tar.gz \
  --name "p53-pipeline-v1.0" \
  --output json > artifact.json

ARTIFACT_ID=$(jq -r '.id' artifact.json)

# 3. Reference the artifact in nextflow create
ivcap nextflow create \
  --service-id "urn:ivcap:service:..." \
  --artifact "$ARTIFACT_ID" \
  --output json
```

**Or via MCP after upload:**
```json
{
  "tool": "nextflow_create",
  "arguments": {
    "service_id": "urn:ivcap:service:...",
    "sources": [
      {
        "path": ".",
        "type": "artifact",
        "artifact_id": "urn:ivcap:artifact:xyz123..."
      }
    ]
  }
}
```

**Advantages:**
- ✅ Single artifact upload, no size limits
- ✅ Exact same file permissions/structure preserved
- ✅ Faster execution (tar already built)
- ✅ Reproducible (same tar.gz always)

### Pattern 3: Staged Sources (For Development Only) ⚠️

**During development**, use smaller batches:

```json
// Call 1: Core files only (~15KB total)
{
  "tool": "nextflow_create",
  "arguments": {
    "service_id": "urn:ivcap:service:abc123...",
    "sources": [
      {"path": "main.nf", "type": "text", "text": "..."},
      {"path": "nextflow.config", "type": "text", "text": "..."},
      {"path": "ivcap.yaml", "type": "text", "text": "..."}
    ]
  }
}

// Call 2: Add scripts via update (3-4 files, ~20KB)
{
  "tool": "nextflow_update",
  "arguments": {
    "service_id": "urn:ivcap:service:abc123...",  // SAME ID
    "sources": [
      {"path": "bin/fetch_data.py", "type": "text", "text": "..."},
      {"path": "bin/analyze.py", "type": "text", "text": "..."}
    ]
  }
}
```

**Note:** Calling `nextflow_create` with the same `service_id` updates the existing service (idempotent).

**Trade-offs:**
- ⚠️ Multiple round trips (slower)
- ⚠️ Can hide partial failures (first call fails, second succeeds partially)
- ✅ Good for iterative development
- ❌ NOT recommended for production

### Size Limits Reference

| Source Type | Recommended Max | Hard Limit | Notes |
|-------------|----------------|------------|-------|
| Single `type: "text"` file | <10KB | ~50KB | Larger files slow parsing |
| Total inline sources per call | <50KB | ~100KB | Above this, expect timeouts |
| `type: "artifact"` | Unlimited | No limit | Stored in IVCAP, not inlined |
| `type: "url"` (external HTTP) | Unlimited | No limit | Fetched by server |
| `artifact_build` per file | <10KB | 100KB | Per `add` call |
| `artifact_build` total | Unlimited | No limit | Can add 1000+ files |

### Anti-Pattern: `type: "url"` for Local Files ❌

```json
{
  "path": "bin/script.py",
  "type": "url",
  "url": "file:///home/claude/pipeline/bin/script.py"  // ❌ Won't work (sandboxed)
}
```

**Why it fails:**
- MCP server runs in a sandboxed environment
- `file://` URLs are not accessible
- Only `http://` and `https://` URLs work

✅ **Do this instead:**
```json
{
  "path": "bin/script.py",
  "type": "text",
  "text": "#!/usr/bin/env python3\n..."  // Inline for small files
}

// OR use artifact_build for large files
// OR upload to external server and reference via https://
```

### Troubleshooting Silent Failures

If `nextflow_create` returns without error but service isn't created:

#### 1. Check Total Size

If you're an agent with bash access:
```bash
# Estimate inline sources size
du -sh pipeline/  # Should be <50KB for all inline content
```

Or in Python:
```python
total_size = sum(len(src["text"]) for src in sources if src.get("type") == "text")
print(f"Total inline size: {total_size / 1024:.1f} KB")

if total_size > 50_000:
    print("⚠️ TOO LARGE - Use artifact_build or pre-built artifact")
```

#### 2. Reduce to Bare Minimum

Try with just essential files:
```json
{
  "service_id": "...",
  "sources": [
    {"path": "main.nf", "type": "text", "text": "..."},
    {"path": "ivcap.yaml", "type": "text", "text": "..."}
  ]
}
```

If THIS works, the issue was request size. Add other files via:
- Pattern 1 (artifact_build)
- Pattern 2 (pre-built artifact)
- Pattern 3 (staged updates)

#### 3. Verify Service Was Created

```json
{
  "tool": "service_list",
  "arguments": {
    "search": "my-pipeline"
  }
}
```

Check if your service ID appears in results.

#### 4. Check for Idempotency Issues

Calling `nextflow_create` twice with the same service_id updates (doesn't error).

**This is intentional** but can hide issues:
- First call partially fails (e.g., missing files)
- Second call silently succeeds with incomplete sources
- Pipeline appears created but is broken

**Solution:** Always verify after create:
```json
{
  "tool": "service_get",
  "arguments": {
    "id": "urn:ivcap:service:..."
  }
}
```

### Real-World Example: P53 Pipeline (10+ Files)

**Scenario:** Building a p53 mutation analysis pipeline with:
- `main.nf` (5KB)
- `nextflow.config` (2KB)
- `ivcap.yaml` (8KB)
- `bin/fetch_ensembl.py` (12KB)
- `bin/parse_vcf.py` (8KB)
- `bin/analyze_mutations.py` (15KB)
- `bin/generate_report.py` (10KB)
- `bin/utils.py` (6KB)
- `bin/plotting.py` (8KB)
- ... (3 more Python files, 20KB)

**Total: ~100KB inline would FAIL**

**Solution using artifact_build:**

```python
# Step 1: Initialize
session = artifact_build(stage="init")
session_id = session["session_id"]

# Step 2: Add each file incrementally
files = [
    ("main.nf", "text/x-nextflow", read_file("main.nf")),
    ("nextflow.config", "text/plain", read_file("nextflow.config")),
    ("ivcap.yaml", "application/x-yaml", read_file("ivcap.yaml")),
    ("bin/fetch_ensembl.py", "text/x-python", read_file("bin/fetch_ensembl.py")),
    ("bin/parse_vcf.py", "text/x-python", read_file("bin/parse_vcf.py")),
    # ... all other files
]

for path, mime, content in files:
    import base64
    content_b64 = base64.b64encode(content.encode()).decode()

    artifact_build(
        stage="add",
        session_id=session_id,
        path_name=path,
        content_base64=content_b64,
        mime_type=mime
    )

# Step 3: List to verify (optional)
staged = artifact_build(stage="list", session_id=session_id)
print(f"Staged {len(staged['files'])} files")

# Step 4: Submit
result = artifact_build(
    stage="submit",
    session_id=session_id,
    name="p53-mutation-pipeline-v1.0"
)

artifact_id = result["artifact_id"]

# Step 5: Deploy pipeline
nextflow_create(
    service_id="urn:ivcap:service:...",
    sources=[{
        "path": ".",
        "type": "artifact",
        "artifact_id": artifact_id
    }]
)
```

**Result:** ✅ All 10+ files deployed successfully, no size issues

### Decision Tree: Which Pattern to Use?

```
How many files in your pipeline?
├─ 1-3 files, <20KB total
│  └─> Use inline sources (type: "text") ✓
│
├─ 4-6 files, 20-50KB total
│  ├─ Agent with MCP?
│  │  └─> Use artifact_build (Pattern 1) ✓
│  └─ CLI user?
│     └─> Pre-build tar.gz (Pattern 2) ✓
│
└─ 7+ files, >50KB total
   └─> MUST use:
       ├─ artifact_build (Pattern 1) for agents ✓
       └─ Pre-built artifact (Pattern 2) for CLI ✓
```

**Never use Pattern 3 (staged updates) for >50KB pipelines in production.**

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

---

## MCP Tool Gotchas

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

---

## Sources Parameter Format

The `sources` array defines the pipeline package contents. Each source becomes a file in the tar.gz archive.

### Source Types

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

### Required Source Fields per Type

| Type | path | type | Content Field | Optional |
|------|------|------|---|---|
| text | ✓ | ✓ | text | - |
| base64 | ✓ | ✓ | base64 | media_type |
| url | ✓ | ✓ | url | - |
| artifact | ✓ | ✓ | artifact_id, artifact_path | - |

### Common Mistakes

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

---

## MCP Tool Execution Flow

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
    "results_artifact_urn": "urn:ivcap:artifact:..."
  }
}
```
✓ Job completed immediately
✓ Results are in `result.results_artifact_urn`

### Pattern 2: Polling Required (Slow Path)
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

### Polling Code Pattern

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

### Important Notes
- ✅ Set appropriate `poll_after_seconds` (usually 30s)
- ✅ Always check both `.status` and `.result.status`
- ❌ Don't poll in tight loops (respect poll_after_seconds)
- ❌ Don't assume immediate completion

---

## Accessing Pipeline Results via MCP

Results are stored in IVCAP artifacts. Use `artifact_get()` to retrieve them.

### Pattern: Fetch CSV Results
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

### Pattern: Fetch Text Report
```python
report = artifact_get(
    id="urn:ivcap:artifact:...",
    path="/results/report.txt",
    accept=["text/plain"]
)
print(report)
```

### Critical: Use `accept` Parameter
- ✅ `accept: ["text/csv"]` → Returns readable CSV
- ✅ `accept: ["text/plain"]` → Returns readable text
- ✅ `accept: ["text/*"]` → Returns any text format
- ❌ Omit `accept` → Returns base64-encoded data (harder to parse)

### Results Artifact Structure
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

---

## MCP Tools Quick Reference

### nextflow_create()
| Parameter | Required | Type | Example |
|-----------|----------|------|---------|
| service_id | YES | string | `"urn:ivcap:service:fc51f603-1514-5dd0-a259-e7cb08970874"` |
| sources | YES | array | `[{path: "main.nf", type: "text", text: "..."}]` |
| name | NO | string | `"my-pipeline"` |
| collection | NO | string | `"urn:ivcap:collection:..."` |

### nextflow_run()
| Parameter | Required | Type | Example |
|-----------|----------|------|---------|
| service_id | YES | string | `"urn:ivcap:service:fc51f603..."` |
| input | NO* | object | `{parameters: {top_variants: 30}}` |
| aspect_urn | NO* | string | `"urn:ivcap:aspect:..."` |
| watch | NO | boolean | `true` (wait for completion) |

*Either input or aspect_urn required

### job_status()
| Parameter | Required | Type | Example |
|-----------|----------|------|---------|
| job_id | YES | string | `"urn:ivcap:job:13a363f5..."` |

### artifact_get()
| Parameter | Required | Type | Example |
|-----------|----------|------|---------|
| id | YES | string | `"urn:ivcap:artifact:99ce0551..."` |
| path | NO | string | `"13a363f5.../results/output.csv"` |
| accept | NO | array | `["text/csv"]` |

---

## Next Steps

- **MCP Debugging**: `skills://nextflow-mcp-debugging/SKILL.md`
- **Production Practices**: `skills://nextflow-production-practices/SKILL.md`
- **General Debugging**: `skills://nextflow-debugging/SKILL.md`
