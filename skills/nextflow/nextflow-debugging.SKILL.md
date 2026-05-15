---
name: nextflow-debugging
version: 0.1.0
description: >
  Debugging Nextflow pipeline execution failures, including service ID reuse,
  log access, debugging patterns, and MCP agent approaches.
requires:
  bins: ["ivcap"]
---

# Nextflow Pipeline Debugging

This skill covers debugging Nextflow pipeline execution failures, log analysis, and iterative development patterns.

**See also:**
- Production Practices: `skills://nextflow-production-practices/SKILL.md`
- MCP Debugging: `skills://nextflow-mcp-debugging/SKILL.md`
- MCP Tools: `skills://nextflow-mcp-tools/SKILL.md`

---

## 🐛 Debugging Workflow: Reuse Service IDs with `nextflow_update`

**When debugging or iteratively developing a pipeline, DO NOT create a new service ID each time.**

Instead, create the service ID once, then use `nextflow_update` (or `nextflow_create` with the same service-id) to update the pipeline package for that service.

### Why Reuse Service IDs During Development?

Creating a new service ID for every minor change:
- ❌ Pollutes the service registry with duplicate/abandoned services
- ❌ Breaks workflow history and makes tracking versions difficult
- ❌ Makes cleanup harder (multiple service IDs to manage)
- ❌ Loses context about which service is "current"

Reusing the same service ID:
- ✅ Maintains a clean service registry
- ✅ Preserves service metadata and history
- ✅ Makes it clear which service is being actively developed
- ✅ Simplifies testing (same service ID, updated implementation)

### Debugging Workflow Pattern

#### Step 1: Create Service ID Once (First Time Only)

```bash
# Generate a stable service ID for your pipeline
SERVICE_ID="urn:ivcap:service:$(uuidgen | tr '[:upper:]' '[:lower:]')"
echo "Your service ID: $SERVICE_ID"
# Save this! You'll reuse it for all updates during development
```

Or when using MCP tools, generate the service ID and save it:

```json
{
  "service_id": "urn:ivcap:service:a98b81a8-9279-509f-9c0e-40d39e83058a"
}
```

#### Step 2: Initial Pipeline Creation

Use `nextflow_create` (MCP) or `ivcap nextflow create` (CLI) with your chosen service ID:

**CLI:**
```bash
ivcap nextflow create \
  --service-id "$SERVICE_ID" \
  -f pipeline-package.tar.gz
```

**MCP Tool:**
```json
{
  "tool": "nextflow_create",
  "arguments": {
    "service_id": "urn:ivcap:service:a98b81a8-9279-509f-9c0e-40d39e83058a",
    "sources": [
      {
        "path": "main.nf",
        "type": "text",
        "text": "// Your Nextflow code"
      },
      {
        "path": "ivcap.yaml",
        "type": "text",
        "text": "..."
      }
    ]
  }
}
```

#### Step 3: Debug and Iterate

After making changes to your pipeline:

**CLI (Preferred for Debugging):**
```bash
# Update the same service with new pipeline code
ivcap nextflow update \
  "$SERVICE_ID" \
  -f pipeline-package.tar.gz
```

**MCP Tool (When Available):**
```json
{
  "tool": "nextflow_update",
  "arguments": {
    "service_id": "urn:ivcap:service:a98b81a8-9279-509f-9c0e-40d39e83058a",
    "sources": [
      {
        "path": "main.nf",
        "type": "text",
        "text": "// Your UPDATED Nextflow code"
      },
      {
        "path": "ivcap.yaml",
        "type": "text",
        "text": "..."
      }
    ]
  }
}
```

**Note:** If `nextflow_update` MCP tool is not yet available, you can use `nextflow_create` with the same `service_id` - it will update the existing service.

#### Step 4: Test the Updated Pipeline

```bash
# Run the updated pipeline
ivcap nextflow run "$SERVICE_ID" -f input.json

# Or with MCP:
# Use nextflow_run with the same service_id
```

### Typical Debugging Cycle

```
1. Edit pipeline code (main.nf, modules, scripts)
   ↓
2. Update ivcap.yaml if needed (new parameters, samples schema changes)
   ↓
3. Package changes into tar.gz (or prepare MCP sources)
   ↓
4. Run: ivcap nextflow update $SERVICE_ID -f package.tar.gz
   ↓
5. Run: ivcap nextflow run $SERVICE_ID -f test-input.json
   ↓
6. Review job results and logs
   ↓
7. Repeat from step 1 until working correctly
```

---

## Accessing Nextflow Logs When Runs Fail

When a Nextflow pipeline fails, the job result now includes a separate `log_urn` artifact containing the Nextflow execution logs (no need to extract from a large results artifact).

**Job result structure (new format):**

```yaml
$schema: urn:ivcap:schema:nextflow.result.1
job_id: urn:ivcap:job:9c1db38c-e180-4570-9874-e85db9bda90f
status: failed
log_urn: urn:ivcap:artifact:4d8fcdff-273a-4498-936a-a92333935696
output_urn: urn:ivcap:artifact:5e9gdfee-374b-5599-a47b-b93444a46707
results:
  process_name: urn:ivcap:artifact:...
```

### Getting the Nextflow Log (New, Simpler Way)

The `log_urn` contains the Nextflow log file directly - **no extraction needed!**

**CLI Method:**

```bash
# 1. Get the job result (contains log_urn, output_urn, results)
JOB_ID="urn:ivcap:job:9c1db38c-e180-4570-9874-e85db9bda90f"
ivcap nextflow job-get "$JOB_ID" --output json > job_result.json

# 2. Extract the log artifact URN
LOG_ARTIFACT=$(jq -r '.log_urn' job_result.json)

# 3. Download and view the log directly
ivcap artifact download "$LOG_ARTIFACT" -o nextflow.log
cat nextflow.log
```

**Even Simpler - Use the Built-in Command:**

```bash
# View logs directly without manual extraction
ivcap nextflow job-result "$JOB_ID" --logs

# Or download logs to a directory
ivcap nextflow job-result "$JOB_ID" --logs -f /path/to/logs
```

**MCP Method:**

```json
{
  "tool": "artifact_get",
  "arguments": {
    "id": "urn:ivcap:artifact:4d8fcdff-273a-4498-936a-a92333935696",
    "accept": ["text/plain"]
  }
}
```

**Key Difference:** The log artifact is now standalone - the content is the log file itself, not a tar archive that needs extraction. Always include `"accept": ["text/plain"]` to get readable text instead of base64-encoded data.

**Supported text formats:**
- `"accept": ["text/plain"]` - For log files, plain text files (also accepts CSV and TSV as plain text)
- `"accept": ["text/csv"]` - For CSV files specifically
- `"accept": ["text/tsv"]` - For TSV files specifically
- `"accept": ["text/*"]` - For any text-based format (universal fallback)
- `"accept": ["application/json"]` - For JSON files

**What's in the Nextflow log:**
- Complete execution trace
- Process execution details
- Error messages and stack traces
- Resource usage information
- Nextflow configuration used

---

## Common Debugging Patterns Using Logs

### Pattern 1: Container Not Found
```bash
# Look for container pull errors in the log
grep -i "container" ./$JOB_UUID/.nextflow.log
grep -i "docker" ./$JOB_UUID/.nextflow.log
grep -i "singularity" ./$JOB_UUID/.nextflow.log
```

Common error:
```
Error pulling container 'quay.io/biocontainers/biopython:1.83'
```

**Fix:** Verify the container image exists (see "Verifying Container Images" in production-practices)

### Pattern 2: Input File Not Found
```bash
# Look for file access errors
grep -i "no such file" ./$JOB_UUID/.nextflow.log
grep -i "cannot find" ./$JOB_UUID/.nextflow.log
```

Common error:
```
Cannot find file: /work/sample_data.fastq
```

**Fix:** Check that sample URNs are correct and accessible

### Pattern 3: Process Script Failures
```bash
# Look for process-specific errors
grep -i "process.*failed" ./$JOB_UUID/.nextflow.log
grep -i "exit status" ./$JOB_UUID/.nextflow.log
```

Common error:
```
Process 'ANALYZE' terminated with an error exit status (1)
```

**Fix:** Check the process script logic and dependencies

### Pattern 4: Resource Limits
```bash
# Look for memory or CPU issues
grep -i "out of memory" ./$JOB_UUID/.nextflow.log
grep -i "killed" ./$JOB_UUID/.nextflow.log
```

Common error:
```
Process exceeded memory limit
```

**Fix:** Increase memory allocation in process directives

---

## Complete Debugging Example with Logs

```bash
# 1. Pipeline fails
ivcap nextflow run "$SERVICE_ID" -f test.json --output json > run_result.json

# 2. Check status
STATUS=$(jq -r '.status' run_result.json)
echo "Status: $STATUS"  # Output: failed

# 3. Get job ID and results artifact
JOB_ID=$(jq -r '.job_id' run_result.json)
RESULTS_ARTIFACT=$(jq -r '.results_artifact_urn' run_result.json)
JOB_UUID=$(echo "$JOB_ID" | sed 's/.*://')

# 4. Download and extract logs
ivcap artifact download "$RESULTS_ARTIFACT" -o results.tar.gz
tar xzf results.tar.gz

# 5. Examine the Nextflow log
echo "=== Last 50 lines of Nextflow log ==="
tail -n 50 "./$JOB_UUID/.nextflow.log"

# 6. Search for specific errors
echo "=== Container errors ==="
grep -i "container" "./$JOB_UUID/.nextflow.log" | grep -i error

echo "=== Process failures ==="
grep -i "failed" "./$JOB_UUID/.nextflow.log"

# 7. Based on findings, fix the issue and update
# Fix main.nf (e.g., correct container directive)
tar czf fixed.tar.gz main.nf nextflow.config ivcap.yaml

# 8. Update the service
ivcap nextflow update "$SERVICE_ID" -f fixed.tar.gz

# 9. Test again
ivcap nextflow run "$SERVICE_ID" -f test.json
```

---

## MCP Agent Pattern for Log Analysis

If you are an MCP agent debugging a failed pipeline:

1. **Check job result for failure**
   ```json
   {
     "tool": "nextflow_run",
     "arguments": {
       "service_id": "urn:ivcap:service:...",
       "input": {...}
     }
   }
   ```
   Response includes: `status: failed`, `results_artifact_urn`

2. **Fetch the Nextflow log**
   ```json
   {
     "tool": "artifact_get",
     "arguments": {
       "id": "<results_artifact_urn>",
       "path": "<job_uuid>/.nextflow.log",
       "accept": ["text/plain"]
     }
   }
   ```

3. **Analyze log content**
   - Look for error patterns (container, file, process failures)
   - Identify the failing process and line number
   - Determine root cause

4. **Suggest fix and update pipeline**
   ```json
   {
     "tool": "nextflow_update",
     "arguments": {
       "service_id": "urn:ivcap:service:...",
       "sources": [
         {
           "path": "main.nf",
           "type": "text",
           "text": "// Fixed code based on log analysis"
         }
       ]
     }
   }
   ```

**Example agent response:**

```
I see the pipeline failed. Let me check the Nextflow log...

[Fetches log via artifact_get]

I found the issue: The container 'quay.io/biocontainers/biopython:1.83'
could not be pulled. This image may not exist.

Let me verify and fix:
1. The correct image is 'docker.io/library/python:3.11-slim' (verified official image)
2. We can install biopython via pip in the script

I'll update the pipeline with the working container...
```

---

## When to Create a New Service ID

**ALWAYS create a new service ID when:**
- ✅ **Pipeline is production-ready and others are using it** - Once users rely on your pipeline for real work, any substantial changes require a new service ID
- ✅ **Adding substantial new features** - Major functionality changes (not just bug fixes) warrant a new version
- ✅ Starting a completely different pipeline
- ✅ You want to publish a new major version alongside the existing one
- ✅ You're forking/branching pipeline functionality significantly
- ✅ Moving from development to production

**DO NOT create a new service ID for:**
- ❌ Minor bug fixes during active development
- ❌ Parameter adjustments during debugging
- ❌ Debugging iterations before first production use
- ❌ Container image updates (unless they change functionality)
- ❌ Documentation improvements
- ❌ Small feature additions during active development

---

## MCP Agent Guidance

**If you are an MCP agent helping debug a Nextflow pipeline:**

1. **Check if service ID exists in conversation history**
   - If yes: Use `nextflow_update` (or `nextflow_create` with same ID)
   - If no: Ask user if they have a service ID to reuse, or generate one

2. **Always suggest reusing service IDs during debugging**
   ```
   "I see we're debugging the pipeline. Should I update the existing service
   (urn:ivcap:service:...) or create a new one? I recommend updating the
   existing service to keep your registry clean."
   ```

3. **Track service ID across conversation**
   - Store the service ID when first created/mentioned
   - Reuse it for all subsequent updates in the same debugging session

4. **Use `nextflow_update` for iterations**
   ```json
   {
     "tool": "nextflow_update",
     "arguments": {
       "service_id": "<previously-used-service-id>",
       "sources": [...]
     }
   }
   ```

---

## Fetching Pipeline Results (CSV, TSV, JSON)

Many Nextflow pipelines output tabular results as CSV or TSV files. When fetching these from the results artifact, use the appropriate accept parameter to get them as readable text instead of base64:

**Example: Fetching a CSV results file**

```json
{
  "tool": "artifact_get",
  "arguments": {
    "id": "urn:ivcap:artifact:4d8fcdff-273a-4498-936a-a92333935696",
    "path": "9c1db38c-e180-4570-9874-e85db9bda90f/results/differential_expression.csv",
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
    "accept": ["text/tab-separated-values"]
  }
}
```

---

## Next Steps

- **Production Practices**: `skills://nextflow-production-practices/SKILL.md`
- **MCP Debugging**: `skills://nextflow-mcp-debugging/SKILL.md`
- **Examples**: `skills://nextflow-examples/SKILL.md`
