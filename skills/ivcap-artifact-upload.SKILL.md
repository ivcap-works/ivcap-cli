---
name: ivcap-artifact-upload
version: 0.1.0
description: Upload artifact content (create new artifact, then upload/resume content) using safe paths.
requires:
  bins: ["ivcap"]
---

# Skill: Upload an artifact (agent-safe)

In IVCAP, artifacts are blob-like data objects. Uploading typically has two phases:
1) **Create** the artifact record (metadata)
2) **Upload** the artifact content (bytes)

## Best practices
- Prefer explicit URNs copied from JSON output.
- Do **not** write outside the current working directory unless explicitly requested.
- For large files, rely on chunking and resume (`artifact upload`) rather than re-sending everything.

## Steps

### 1) Create a new artifact (metadata + initial content)

If you already have the file locally, `artifact create` can upload content in one step:

```bash
ivcap --output json artifact create -n "my-dataset" -f ./dataset.tar.gz -t application/gzip
```

### 2) Upload/resume content to an existing artifact

If you have an existing artifact id/URN that needs content upload/resume:

```bash
ivcap --output json artifact upload <artifact-urn-or-id> -f ./dataset.tar.gz -t application/gzip
```

### 3) Verify (optional)

```bash
ivcap --output json artifact get <artifact-urn>
```
