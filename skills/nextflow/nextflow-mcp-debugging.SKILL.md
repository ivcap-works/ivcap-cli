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
        "sources_count": 12,
        "total_size_bytes": 85000,
        "source_types": ["text", "text", "artifact"],
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
| Silent success, no resource | Size limit exceeded | Report with request size, suggest validation |
| Timeout after 30s | Large payload parsing | Report with payload size, suggest streaming |
| "Invalid JSON" on valid JSON | Special characters unescaped | Report with sample, suggest escaping |
| Works in CLI, fails in MCP | Parameter mapping issue | Report parameter differences |
| Intermittent success | Race condition | Report timing details, suggest locking |

---

## Example: Reporting a Tool Improvement

**Scenario:** `nextflow_create` silently fails with large sources

```markdown
## MCP Tool Issue Report

**Tool:** nextflow_create (ivcap-cli MCP server)

**Issue:** Silent failure on large source payloads without error message

**Expected Behavior:**
Tool should either:
1. Accept the payload and create the service, OR
2. Return clear error: "Payload too large (85KB). Maximum: 50KB. Use artifact_build for larger pipelines."

**Actual Behavior:**
- Returns HTTP 200 with success message
- No service created
- No error logged
- User has no indication of failure until verification

**Reproduction:**
```python
nextflow_create(
    service_id="urn:ivcap:service:...",
    sources=[
        {"path": "main.nf", "type": "text", "text": "..." * 5000},  # 5KB
        {"path": "config", "type": "text", "text": "..." * 2000},   # 2KB
        # ... 10 more files totaling 85KB
    ]
)
# Returns: {"status": "success", "service_id": "..."}
# But: service_list() shows no matching service
```

**Impact:**
- HIGH: Affects any multi-file pipeline
- User confusion (success reported but nothing created)
- Wasted time debugging the wrong layer

**Suggested Implementation:**
```go
// In pkg/mcp/nextflow.go
func (s *Server) handleNextflowCreate(args map[string]any) (any, error) {
    // Add size validation
    totalSize := 0
    for _, source := range sources {
        if source.Type == "text" {
            totalSize += len(source.Text)
        }
    }

    const maxInlineSize = 50 * 1024  // 50KB
    if totalSize > maxInlineSize {
        return nil, fmt.Errorf(
            "inline sources too large (%d bytes). Maximum: %d bytes. "+
            "For larger pipelines, use artifact_build tool to create artifact, "+
            "then reference with type='artifact'",
            totalSize, maxInlineSize,
        )
    }

    // ... rest of implementation
}
```

**Documentation Update Needed:**
Update tool description in MCP schema to mention:
- Maximum inline payload size (50KB recommended, 100KB hard limit)
- Recommend artifact_build for larger pipelines
- Link to examples in skills docs

**Workaround (for users now):**
Use artifact_build pattern as documented in skills/nextflow/nextflow-mcp-tools.SKILL.md

**Related:**
- artifact_build tool was added to solve this exact problem
- CLI handles this better (direct tar.gz upload, no size issues)
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

### Check Job Status
```python
response = job_status(job_id="urn:ivcap:job:...")
print(response["result"]["status"])  # "succeeded" or "failed"
results_urn = response["result"]["results_artifact_urn"]
```

### Fetch Nextflow Log from Failed Job
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

### Common Failure Causes

| Issue | Solution |
|-------|----------|
| Container image not found | Verify image exists: `docker manifest inspect <image>` |
| Script file not found | Use `bin/script.py` path, not bare `script.py` |
| Missing `ivcap.yaml` | Ensure ivcap.yaml is in sources with `path: "ivcap.yaml"` |
| Malformed ivcap.yaml | Run YAML validator: `yaml.safe_load(ivcap_content)` |
| Parameter type mismatch | Check parameter types in ivcap.yaml match input |

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

## Next Steps

- **MCP Tools Usage**: `skills://nextflow-mcp-tools/SKILL.md`
- **General Debugging**: `skills://nextflow-debugging/SKILL.md`
- **Production Practices**: `skills://nextflow-production-practices/SKILL.md`
