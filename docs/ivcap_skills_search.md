## ivcap skills search

Search skills, best match first

### Synopsis

Rank the embedded skills against a free-text query using BM25.

Matches in a skill's name and description count for more than matches in its
body. Only skills matching at least one query term are returned. The search is
local and needs no network access or login.

```
ivcap skills search <query>... [flags]
```

### Examples

```
  ivcap skills search upload a file
  ivcap --output json skills search "debug nextflow pipeline" --limit 3
```

### Options

```
  -h, --help        help for search
  -l, --limit int   max number of skills returned (0 = all matches) (default 10)
```

### Options inherited from parent commands

```
      --access-token string   Access token to use for authentication with API server [IVCAP_ACCESS_TOKEN]
      --context string        Context (deployment) to use
      --debug                 Set logging level to DEBUG
      --no-history            Do not store history
  -o, --output string         Set format for displaying output [json, yaml]
      --silent                Do not show any progress information
      --timeout int           Max. number of seconds to wait for completion (default 30)
```

### SEE ALSO

* [ivcap skills](ivcap_skills.md)	 - List, search and show agent skill docs embedded in this CLI release

