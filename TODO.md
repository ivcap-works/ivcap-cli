# TODO

Open issues and ideas for ivcap-cli, collected as they come up. Remove an item when it is done.

## Platform (needs an API change, not a CLI fix)

- **Create an artifact as part of its upload.** An upload is two steps today: create the artifact record,
  then upload content to it. A failure in between leaves an artifact that exists but is empty or partial,
  and nothing lets a client tell a good artifact from a broken one afterwards. The API should create the
  artifact together with its content, or only mark it available once the content is complete, so that a
  failed upload leaves nothing behind. This affects `artifact create`, `ivcap genome import` and the Nextflow
  package upload. There is also no `ivcap artifact delete`, so broken artifacts stay.

## genome import

- **Repair a failed import (parked until the platform fix above).** A failure after the artifact record is
  created leaves the artifact behind, and the temporary package and the aspect documents are deleted when
  the command returns. Planned approach: `import --artifact URN`, which
  1. reads the artifact and checks that it exists, has status `available` (not a partial upload) and a
     non-zero size,
  2. does not upload anything,
  3. attaches the report aspect, skipping it if the artifact already has one with that schema
     (`findAspectID`).

  Limits to note: the size cannot be compared with NCBI's package, because NCBI sends no `Content-Length`
  and the download-summary size is only an estimate. The manifest needs the package's file list, so it is
  open whether repair skips it, rebuilds it from the artifact content, or re-downloads the package.
  Alternative: find an existing artifact for the accession through its report aspect, so that re-running
  `import` reuses it.
- **Pre-flight login check.** Login problems and permissions only surface at `CreateArtifact`, after the NCBI
  download. A cheap authenticated call before the download would catch them earlier. This needs a design
  choice: which call, and how it behaves with expiring tokens.
- **Open question:** are the files in `schema/` registered with the platform, and are aspects validated
  against registered schemas? Today the code does not use them.

## Archive manifests (artifact create and upload)

- **RO-Crate identifier and checksums.** `license` is done (`--license`; imports link to NCBI's data usage
  policy). `identifier` is a file name or accession. The 1.2 spec tolerates a plain string, but recommends a
  `PropertyValue` for a persistent identifier, and the artifact URN would suit generic archives. The manifest
  is built before the artifact exists, so that needs a small refactor. Entries carry no checksums.
- **Manifest size.** There is no cap on archives with huge file counts. Skip the manifest with a warning, or truncate it?
- **Collections.** Should `collection create --dir` get its own `--without-manifest`? It uploads through the same code but has no such flag.
- **Nextflow package uploads** use their own upload code and get no manifest.

## Tests

- Validate the generated aspects against `schema/*.json`. This needs a JSON-schema validator dependency:
  `go.mod` only has `invopop/jsonschema`, which generates schemas.
- End-to-end test of `import` against a real deployment. Only mocks so far.

## Housekeeping

- Commit `a495746` mixes two features and has a typo in its message. Split and reword before pushing.
- Optional: a percentage progress bar for `ivcap genome download`. HEAD gets 405 and the POST has no
  `Content-Length`, so it would need the download-summary estimate.
