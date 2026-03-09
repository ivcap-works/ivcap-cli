package skills

import (
	"io/fs"
	"testing"
)

func TestEmbeddedAgentContextExists(t *testing.T) {
	if _, err := fs.ReadFile(FS, "CONTEXT.md"); err != nil {
		t.Fatalf("expected embedded CONTEXT.md, got error: %v", err)
	}
}
