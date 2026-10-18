---
name: ivcap-artifact-upload
version: 0.2.0
description: Upload a local file as an artifact.
requires:
  bins: ["ivcap"]
---

# Skill: Upload an artifact (agent-safe)

## Best practices
- Confirm intent before uploading.
- Avoid path traversal; prefer a local relative path.
- Local files named `*.zip`, `*.tar`, `*.tar.gz` or `*.tgz` also get a file manifest aspect by default (an
  RO-Crate 1.2 JSON-LD document, schema `urn:ivcap:schema:artifact.archive-manifest.1`). Only the file name
  decides. Pass `--without-manifest` if the user does not want a manifest. Content read from stdin
  (`-f -`) is never indexed.
- `--license` records the license of the content in the manifest: an SPDX identifier such as `CC-BY-4.0`, a
  license URL, or `undefined` (the default). Do not guess a license: keep the default unless the user states one.
- The manifest is a convenience. If the archive cannot be indexed, or the manifest aspect cannot be added,
  the command prints a warning and the upload still completes. Run the same command again to retry the
  manifest; it never adds a second one.
- Leave `--chunk-size` alone unless there is a reason. It must be at least 65536 (bytes), or -1 for no
  chunking, which does not work for stdin.

## Example

```bash
ivcap --output json artifact create -f ./artifact.bin
```

To resume an interrupted upload of an existing artifact:

```bash
ivcap artifact upload urn:ivcap:artifact:... -f ./artifact.bin
```
