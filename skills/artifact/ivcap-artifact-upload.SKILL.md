---
name: ivcap-artifact-upload
version: 0.1.0
description: Upload a local file as an artifact.
requires:
  bins: ["ivcap"]
---

# Skill: Upload an artifact (agent-safe)

## Best practices
- Confirm intent before uploading.
- Avoid path traversal; prefer a local relative path.

## Example

```bash
ivcap --output json artifact upload -f ./artifact.bin
```
