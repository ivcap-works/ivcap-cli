# Nextflow Troubleshooting Reference

General Nextflow errors and fixes. For pipeline-specific errors see the
domain skill's troubleshooting reference.

---

## Exit status 127 — command not found

```
.command.sh: line 3: my_script.py: command not found
Command exit status: 127
```

**Cause**: The venv or script directory is not on PATH inside Nextflow work dirs.

**Checklist:**
```bash
chmod +x bin/*.py                    # scripts must be executable
head -1 bin/my_script.py             # must be: #!/usr/bin/env python3
ls .venv/bin/python3                 # venv must exist
grep PATH nextflow.config            # must use $PWD not $projectDir
```

**Fix** in `nextflow.config`:
```groovy
env { PATH = "$PWD/.venv/bin:$PATH" }
```

---

## Report file already exists

```
AbortOperationException: Report file already exists: results/pipeline_report.html
```

**Fix:**
```groovy
report   { enabled = true; file = "results/pipeline_report.html";   overwrite = true }
timeline { enabled = true; file = "results/pipeline_timeline.html"; overwrite = true }
```

---

## Unknown config attribute `report.params.*`

```
ERROR ~ Unknown config attribute `report.params.outdir`
```

**Cause**: `params` not available at config parse time.

**Fix**: Use literal string in report/timeline file paths.

---

## workflow.onComplete error

```
ERROR ~ No signature of method: groovy.util.ConfigObject.onComplete()
```

**Cause**: `workflow.onComplete` placed in `nextflow.config`.

**Fix**: Move to `main.nf`, after the `workflow {}` block.

---

## `first` is useless on value channel

```
WARN: The operator `first` is useless when applied to a value channel
```

**Cause**: `.first()` called on a channel that already emits a single value
(e.g. output of a process that produces one file).

**Fix**: Remove `.first()`. Only use it on `Channel.fromPath()` queue channels
where multiple emissions are possible.

---

## Wrong branch taken — optional params not null

**Symptom**: Pipeline takes the `if (params.fasta)` local-files branch even
though you want auto-fetch.

**Cause**: Optional params have a default path string instead of `null`.

**Fix**:
```groovy
params.fasta    = null   // ✓
params.variants = null   // ✓
// NOT: params.fasta = "$projectDir/data/protein.fasta"  ✗
```

If the cache has stale values:
```bash
rm -rf work .nextflow*
nextflow run main.nf
```

---

## Python NameError from JavaScript in f-string

```
NameError: name 'MY_VAR' is not defined
```

**Cause**: JavaScript template literals `${}` inside a Python f-string are
interpreted as Python variable substitutions.

**Fix**: Escape all JS `${}` and bare `{}` as `${{}}` and `{{}}`:
```python
# ✗  Python tries to evaluate JS_VAR as Python:
f"const x = `${{JS_VAR}}`"

# ✓  Double braces escape from f-string:
f"const x = `${{{{JS_VAR}}}}`"
```

---

## 301 redirect when serving HTML with `serve`

**Cause**: `npx serve file.html` — serve expects a directory, not a file.

**Fix**:
```bash
# Serve the directory, include filename in URL:
npx serve results/ -p 8080
# → http://localhost:8080/variant_viewer.html

# Or skip the server — modern browsers open file:// directly:
open results/output.html
```
