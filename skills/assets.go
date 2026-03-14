// Copyright 2026 Commonwealth Scientific and Industrial Research Organisation (CSIRO)
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package skills

import "embed"

// NOTE: This file exists solely to host the `//go:embed` directive.
//
// Go embed patterns are evaluated relative to the *package directory*, and Go
// does not allow embedding files from a parent directory (no `../skills`).
// Therefore we keep this small Go file in the `skills/` directory so the
// distributed CLI can embed the version-matched `*.SKILL.md` docs that live
// alongside it.
//
// FS contains the version-matched skill markdown files embedded into the CLI.
//
// Skill docs are intended to be cheap and reliable for agents to access at
// runtime without any network call.
//
//go:embed *.SKILL.md CONTEXT.md
var FS embed.FS
