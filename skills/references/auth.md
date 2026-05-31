# Authentication Errors

If any IVCAP tool returns an authentication error, the session token has
expired. Tell the user:

> "Your IVCAP session has expired. Please run `ivcap context login` in your
> terminal, then let me know when done and I'll retry."

Do NOT retry automatically — the login requires user interaction in the
terminal. Wait for confirmation before proceeding.

## Common auth error signals

- Tool returns `isError: true` with content containing "Authentication required" or "session has expired"
- Tool returns error code -32603 with "ivcap context login" in the message (legacy format)
- Tool returns content with "authentication" or "unauthorized" keywords
- All tools start failing simultaneously (indicates session-wide expiry)

## Expected error format

When authentication fails, MCP tools now return a result with `isError: true`:

```json
{
  "jsonrpc": "2.0",
  "id": 6,
  "result": {
    "content": [{
      "type": "text",
      "text": "Authentication required. Your IVCAP session has expired. Please run 'ivcap context login' in your terminal to refresh your credentials, then retry."
    }],
    "isError": true
  }
}
```

## Authentication lifecycle

1. **Initial login**: User runs `ivcap context login` to authenticate
2. **Session active**: Access token is valid for a limited time
3. **Session expires**: Token becomes invalid after timeout or explicit logout
4. **Re-authentication**: User must run `ivcap context login` again

## Handling in agent workflows

When you detect an authentication error:

1. **Stop** the current operation immediately
2. **Inform** the user their session has expired
3. **Request** the user run `ivcap context login` in their terminal
4. **Wait** for user confirmation before retrying
5. **Resume** the operation after confirmation

Never attempt to retry automatically, as the login process requires:
- User interaction in their terminal
- Potentially opening a browser for OAuth flow
- Explicit credential entry
