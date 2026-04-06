# nextflow.config — Annotated Template

```groovy
/*
 * nextflow.config
 * Adjust resource limits and profiles to match the execution environment.
 */

// ── Python venv PATH ─────────────────────────────────────────
// Use $PWD (shell variable, runtime) not $projectDir (Groovy, parse-time).
// $projectDir does NOT resolve correctly inside env {} blocks.
env {
    PATH = "$PWD/.venv/bin:$PATH"
}

// ── Default process resources ────────────────────────────────
process {
    cpus     = 1
    memory   = '2 GB'
    time     = '10 min'
    maxForks = 10    // cap parallel jobs — important on laptops
}

// ── Execution reports ────────────────────────────────────────
// overwrite = true is REQUIRED to avoid AbortOperationException on re-runs.
// Use literal paths — params.* cannot be used here (evaluated before params).
report {
    enabled   = true
    file      = "results/pipeline_report.html"
    overwrite = true
}

timeline {
    enabled   = true
    file      = "results/pipeline_timeline.html"
    overwrite = true
}

// ── Profiles ─────────────────────────────────────────────────
profiles {

    local {
        process.executor = 'local'
    }

    conda {
        conda.enabled    = true
        process.conda    = "$projectDir/environment.yml"
    }

    docker {
        docker.enabled    = true
        process.container = 'python:3.11-slim'
    }

    singularity {
        singularity.enabled    = true
        singularity.autoMounts = true
        process.container      = 'docker://python:3.11-slim'
    }

    slurm {
        process.executor = 'slurm'
        process.queue    = 'short'
    }
}
```

## Config Block Rules (common mistakes)

**Rule 1: `$PWD` not `$projectDir` in `env` block**
```groovy
env { PATH = "$PWD/.venv/bin:$PATH" }       // ✓ runtime shell variable
env { PATH = "$projectDir/.venv/bin:$PATH" } // ✗ Groovy var, breaks in env block
```

**Rule 2: `overwrite = true` on report/timeline**
Without this, every re-run after the first throws:
`AbortOperationException: Report file already exists`

**Rule 3: No `params.*` in config blocks**
Config blocks are parsed before params are resolved:
```groovy
report { file = "results/report.html" }           // ✓
report { file = "${params.outdir}/report.html" }  // ✗ parse error
```

**Rule 4: `workflow.onComplete` in `main.nf` only**
```groovy
// nextflow.config  ✗ — causes: No signature of method: ConfigObject.onComplete()
workflow.onComplete { log.info "done" }

// main.nf  ✓ — place AFTER the workflow {} block
workflow.onComplete { log.info "done" }
```
