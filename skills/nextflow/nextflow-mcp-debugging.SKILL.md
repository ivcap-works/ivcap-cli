---
name: nextflow-mcp-debugging
version: 0.1.0
description: >
  Debugging MCP tool failures for Nextflow pipelines, including diagnostic
  information gathering, actionable feedback format, and tool improvement reporting.
requires:
  bins: ["ivcap"]
---

# Nextflow MCP Tool Debugging

This skill covers diagnosing and reporting MCP tool failures when deploying or running Nextflow pipelines.

**See also:**
- MCP Tools Usage: `skills://nextflow-mcp-tools/SKILL.md`
- General Debugging: `skills://nextflow-debugging/SKILL.md`
- Production Practices: `skills://nextflow-production-practices/SKILL.md`

---

## Debugging MCP Tool Failures

**When an MCP tool call fails unexpectedly, provide actionable debugging information rather than just defaulting to CLI.**

### Step 1: Diagnose the Failure Type

**User Error vs Tool Error:**

```python
# User error indicators:
- "Missing required parameter: service_id"
- "Invalid URN format"
- "Parameter type mismatch"
→ Fix: Correct the parameters and retry

# Tool error indicators:
- Unexpected timeout without error message
- "Internal server error"
- Silent failure (no response)
- Success response but resource not created
→ Action: Report the tool issue with details below
```

### Step 2: Gather Diagnostic Information

When an MCP tool fails unexpectedly, collect:

```python
diagnostic_info = {
    "tool": "nextflow_create",
    "timestamp": "2026-04-05T11:06:00Z",
    "parameters": {
        "service_id": "urn:ivcap:service:...",
        "artifact_id": "urn:ivcap:artifact:...",
    },
    "response": {
        "status_code": 200,  # or error code
        "body": "...",  # truncated if large
        "duration_ms": 45000,
    },
    "expected_behavior": "Service should be created with returned URN",
    "actual_behavior": "No error but service_list shows no matching service",
}
```

### Step 3: Provide Actionable Feedback

**Good feedback format:**

```
The nextflow_create MCP tool appears to have an issue:

SYMPTOMS:
- Tool returns 200 OK without error
- No service created (verified via service_list)
- Request size: 85KB (12 text sources)

REPRODUCTION:
1. Call nextflow_create with 12 inline text sources (~85KB total)
2. Tool returns success
3. service_list shows no service with that ID

HYPOTHESIS:
Request size (85KB) may be near/over undocumented limit causing silent failure.

SUGGESTED FIX:
1. Add request size validation in nextflow_create tool
2. Return clear error: "Request too large (85KB). Use artifact_build for >50KB"
3. Update tool description with size limits

WORKAROUND USED:
Switched to artifact_build pattern (worked successfully)

IMPACT:
High - Affects any pipeline with >5 files without clear error message
```

**Bad feedback format:**
```
❌ "nextflow_create doesn't work, use CLI instead"
❌ "MCP tool broken, switching to command line"
❌ "Tool failed, no idea why"
```

### Step 4: Pattern Recognition for Common Tool Issues

| Pattern | Root Cause | Agent Action |
|---------|------------|--------------|
| "missing artifact_id" error | Using old inline sources syntax | Guide to new artifact_build workflow |
| "artifact not found" | Invalid artifact URN | Verify artifact exists with artifact_get |
| "neither ivcap.yaml nor ivcap-tool.yaml found" | Missing descriptor in artifact | Show how to add descriptor to artifact_build |
| Works in CLI, fails in MCP | Parameter mapping issue | Report parameter differences |
| Intermittent success | Race condition | Report timing details, suggest locking |
| artifact_create fails with file URL | Using sandbox path MCP can't access | Use artifact_build instead |

---

## Example: Reporting a Tool Improvement

**Scenario:** `nextflow_create` fails with missing artifact

```markdown
## MCP Tool Issue Report

**Tool:** nextflow_create (ivcap-cli MCP server)

**Issue:** Unclear error when artifact doesn't contain required descriptor file

**Expected Behavior:**
Tool should return clear error: "Artifact 'urn:ivcap:artifact:...' does not contain required 'ivcap.yaml' or 'ivcap-tool.yaml' file. Please ensure your artifact_build session includes the tool descriptor."

**Actual Behavior:**
- Returns generic error: "neither 'ivcap.yaml' nor 'ivcap-tool.yaml' found in artifact"
- Doesn't guide user to artifact_build documentation
- Doesn't suggest how to fix the issue

**Reproduction:**
```python
# Build artifact without descriptor file
init = artifact_build(stage="init")
artifact_build(
    stage="add",
    id=init["id"],
    files=[
        {"path_name": "main.nf", "content": "...", "mime_type": "text/plain"},
        {"path_name": "nextflow.config", "content": "...", "mime_type": "text/plain"}
        # Missing ivcap.yaml!
    ]
)
result = artifact_build(stage="submit", id=init["id"])

# Try to deploy - fails with unclear error
nextflow_create(
    service_id="urn:ivcap:service:...",
    artifact_id=result["id"]
)
# Returns: Error: neither "ivcap.yaml" nor "ivcap-tool.yaml" found in artifact
```

**Impact:**
- MEDIUM: Affects users who forget to include descriptor
- Error message could be more helpful
- Should reference artifact_build workflow

**Suggested Implementation:**
```go
// In pkg/mcp/nextflow.go
if toolHdr == nil {
    return nil, fmt.Errorf(
        "artifact %s does not contain required 'ivcap.yaml' or 'ivcap-tool.yaml'. "+
        "When using artifact_build, ensure you add the descriptor file: "+
        `artifact_build(stage="add", files=[{"path_name": "ivcap.yaml", ...}]). `+
        "See skills://nextflow-mcp-tools/SKILL.md for examples",
        parsed.ArtifactID,
    )
}
```

**Workaround (for users now):**
Ensure artifact_build session includes ivcap.yaml:
```python
artifact_build(
    stage="add",
    id=session_id,
    files=[
        {"path_name": "ivcap.yaml", "content": base64_yaml, "mime_type": "application/x-yaml"}
    ]
)
```

**Related:**
- Common mistake when switching from old inline sources workflow
- Should be documented in migration guide
```

---

## Agent Responsibilities When Tool Fails

**DO:**
- ✅ Collect diagnostic information (size, params, response)
- ✅ Verify the failure (check resource was/wasn't created)
- ✅ Provide reproduction steps
- ✅ Suggest specific implementation fixes
- ✅ Document workaround used
- ✅ Report impact level

**DON'T:**
- ❌ Just say "use CLI instead" without explanation
- ❌ Assume tool is unfixable
- ❌ Blame the user without investigation
- ❌ Provide vague "doesn't work" reports
- ❌ Skip testing the workaround

---

## Escalation Path

```
1. Agent encounters unexpected MCP tool failure
   ↓
2. Agent collects diagnostic info (see Step 2)
   ↓
3. Agent tries documented workaround
   ↓
4. Agent provides detailed report to user with:
   - What failed
   - Why it failed (hypothesis)
   - How to fix the tool (specific code/config changes)
   - Workaround used (tested and working)
   ↓
5. User can file issue with complete context
   ↓
6. Tool maintainers have actionable information
```

---

## Debugging Failed Jobs

When a Nextflow job fails during execution:

### Check Job Status (Updated Format)
```python
# CLI: Get job details
job = ivcap nextflow job-get urn:ivcap:job:...
# Shows: IVCAP Status, Nxf Status, Log, and Processes

# MCP: Use job_status tool for completed jobs
job = job_status(job_id="urn:ivcap:job:...")

# MCP returns nextflow result structure (for Nextflow jobs):
# {
#   "nextflow_result": {
#     "status": "succeeded",
#     "log_urn": "urn:ivcap:artifact:...",
#     "output_urn": "urn:ivcap:artifact:...",
#     "results": {
#       "fastqc": "urn:ivcap:artifact:...",
#       "multiqc": "urn:ivcap:artifact:...",
#       "trimming": "urn:ivcap:artifact:..."
#     }
#   },
#   "_meta": {
#     "type": "nextflow",
#     "status": "succeeded"
#   }
# }

# Extract result artifacts
nxf_result = job["nextflow_result"]
log_urn = nxf_result["log_urn"]  # Direct log file (text artifact)
output_urn = nxf_result["output_urn"]  # Output directory (tar artifact)
results = nxf_result["results"]  # Process-specific results
```

### Fetch Nextflow Log from Job (Direct Access - No Extraction)
```python
# CLI: Log URN is displayed directly
ivcap nextflow job-get urn:ivcap:job:... | grep "Log"
# Output: Log  urn:ivcap:artifact:c0d9fea0-... (@N)

# Then download the log
ivcap artifact get @N > job.log
cat job.log | grep ERROR

# MCP: Log is a standalone text artifact
job = job_status(job_id="urn:ivcap:job:...")
log_urn = job["nextflow_result"]["log_urn"]

log_content = artifact_get(
    id=log_urn,
    accept=["text/plain"]
)

# Analyze the log
if "ERROR" in log_content:
    for line in log_content.split('\n'):
        if "ERROR" in line or "error" in line.lower():
            print(line)
```

### Example: Complete Job Debug Pattern (CLI)
```bash
# 1. Run job
ivcap nextflow run urn:ivcap:service:abc123 -f params.json

# 2. Check job status
ivcap nextflow job-get urn:ivcap:job:def456

# Output shows:
# IVCAP Status  succeeded
# Nxf Status    succeeded
# Log           urn:ivcap:artifact:... (@2)
# Processes
#   fastqc      urn:ivcap:artifact:... (@3)
#   multiqc     urn:ivcap:artifact:... (@4)

# 3. Download and examine log
ivcap artifact get @2 > pipeline.log
grep ERROR pipeline.log

# 4. Download process results
ivcap artifact get @3 -f ./fastqc_results/
```

### Example: Complete Job Debug Pattern (MCP)
```python
# 1. Run job and get result (if completes within 30s)
job_result = nextflow_run(service_id="urn:ivcap:service:...", input={...})

# 2. If job still running, poll with job_status
if job_result.get("poll_after_seconds"):
    # Job not finished, wait and poll
    import time
    time.sleep(job_result["poll_after_seconds"])
    job_result = job_status(job_id=job_result["job_id"])

# 3. Check Nextflow result
if job_result["_meta"]["type"] == "nextflow":
    nxf_result = job_result["nextflow_result"]

    if nxf_result["status"] == "failed":
        # Fetch log directly (no tar extraction needed)
        log_text = artifact_get(id=nxf_result["log_urn"], accept=["text/plain"])

        # Analyze the log
        for line in log_text.split('\n'):
            if "ERROR" in line:
                print(f"Error: {line}")

        # Access per-process results
        for process_name, result_urn in nxf_result["results"].items():
            print(f"Process {process_name}: {result_urn}")
```

### Common Failure Causes

| Issue | Solution |
|-------|----------|
| Container image not found | Verify image exists: `docker manifest inspect <image>` |
| Script file not found | Use `bin/script.py` path, not bare `script.py` |
| Missing `ivcap.yaml` | Ensure ivcap.yaml is in sources with `path: "ivcap.yaml"` |
| Malformed ivcap.yaml | Run YAML validator: `yaml.safe_load(ivcap_content)` |
| Parameter type mismatch | Check parameter types in ivcap.yaml match input |

---

## Common MCP Tool Mistakes

### ❌ Using artifact_create with Agent Sandbox Files

**Problem:**
```json
{
  "tool": "artifact_create",
  "arguments": {
    "content": [{
      "source": {
        "type": "url",
        "url": "file:///home/claude/my-pipeline.tar.gz"
      }
    }]
  }
}
```

**Error:** MCP server cannot access files in agent sandbox (`/home/claude/`, `/tmp/`, etc.)

**Solution:** Use `artifact_build` instead:
```python
# Read file from sandbox
with open('/home/claude/my-pipeline.tar.gz', 'rb') as f:
    content_b64 = base64.b64encode(f.read()).decode()

# Upload via artifact_build
init = artifact_build(stage="init")
artifact_build(
    stage="add",
    id=init["id"],
    files=[{
        "path_name": "pipeline.tar.gz",
        "content": content_b64,
        "mime_type": "application/gzip"
    }]
)
result = artifact_build(stage="submit", id=init["id"], name="my-pipeline")
```

**When to use each:**
- `artifact_create`: Files accessible to MCP server (URLs, existing artifacts)
- `artifact_build`: Files in your agent's sandbox that need to be uploaded

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
| artifact_create with sandbox file URL | Use `artifact_build` to upload from agent sandbox |

---

## Next Steps

- **MCP Tools Usage**: `skills://nextflow-mcp-tools/SKILL.md`
- **General Debugging**: `skills://nextflow-debugging/SKILL.md`
- **Production Practices**: `skills://nextflow-production-practices/SKILL.md`
