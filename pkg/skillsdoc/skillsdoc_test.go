package skillsdoc_test

import (
	"testing"

	"github.com/ivcap-works/ivcap-cli/pkg/skillsdoc"
	asset "github.com/ivcap-works/ivcap-cli/skills"
)

func TestLoadAllSkillDocs(t *testing.T) {
	docs, err := skillsdoc.LoadAllSkillDocs(asset.FS)
	if err != nil {
		t.Fatalf("LoadAllSkillDocs: %v", err)
	}
	if len(docs) == 0 {
		t.Fatalf("expected at least one skill doc")
	}
	for _, d := range docs {
		if d.Name == "" || d.Version == "" || d.Description == "" {
			t.Fatalf("invalid doc metadata: %+v", d)
		}
		if d.SHA256 == "" {
			t.Fatalf("missing sha256 for %s", d.Name)
		}
		if d.Path == "" {
			t.Fatalf("missing path for %s", d.Name)
		}
		if d.Content == "" {
			t.Fatalf("missing content for %s", d.Name)
		}
	}
}
