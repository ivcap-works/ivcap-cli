// Copyright 2026 Commonwealth Scientific and Industrial Research Organisation (CSIRO) ABN 41 687 119 230
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

package cmd

import (
	"fmt"
	"io"
)

// manifestForUpload prepares the archive manifest for an upload. The manifest is a
// convenience, so a file that cannot be indexed is reported as a warning and uploaded
// without one.
func manifestForUpload(fileName string, warn io.Writer) []byte {
	content, err := prepareArtifactManifest(fileName)
	if err != nil {
		fmt.Fprintf(warn, "Warning: not adding a manifest for '%s': %v\n", fileName, err)
		return nil
	}
	return content
}

// warnManifestNotAttached reports that the artifact is complete but its manifest aspect could
// not be added. Running the same command again retries only the manifest.
func warnManifestNotAttached(warn io.Writer, artifactID string, err error) {
	fmt.Fprintf(warn, "Warning: artifact '%s' is uploaded, but its manifest aspect could not be added: %v\n"+
		"Run the same command again to retry the manifest.\n", artifactID, err)
}
