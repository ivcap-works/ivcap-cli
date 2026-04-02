## MCP tool: artifact_create

`artifact_create` is a **built-in** tool exposed by `ivcap mcp` (it does not rely on platform tool-aspects).

It creates an IVCAP artifact from an LLM-style `content[]` array:

- If `content` has **one** element, that element becomes the artifact's content.
- If `content` has **multiple** elements, they are packed into a single **tar.gz** and uploaded as `application/gzip`.
  - The optional `name` field of each content part is used as the **path inside the tar**.
  - `/` is allowed to create directories.
  - A `MANIFEST.json` is added to the tarball listing all packaged files (including optional `title` / `context`).

### Supported `source` types

- `source.type: "base64"` with `media_type` + `data`
- `source.type: "url"` with `url`
  - The server downloads the URL and uses the HTTP response `Content-Type` as `media_type` (unless you explicitly set `media_type`).

### Example payload

```json
{
  "name": "comparison-inputs",
  "content": [
    {
      "type": "image",
      "source": { "type": "base64", "media_type": "image/png", "data": "<img1 base64>" },
      "name": "images/img1.png"
    },
    {
      "type": "image",
      "source": { "type": "base64", "media_type": "image/jpeg", "data": "<img2 base64>" },
      "name": "images/img2.jpg"
    },
    {
      "type": "text",
      "text": "Compare these two images.",
      "name": "prompt.txt"
    },
    {
      "type": "document",
      "source": { "type": "base64", "media_type": "application/pdf", "data": "<pdf base64>" },
      "name": "data/contract.pdf",
      "title": "Contract",
      "context": "Signed version."
    },
    {
      "type": "document",
      "source": { "type": "url", "url": "https://example.com/file.pdf" },
      "name": "data/from-url.pdf"
    }
  ]
}
```

### Notes / safety

- Tar paths are sanitized to prevent path traversal (`..`) and absolute paths.
