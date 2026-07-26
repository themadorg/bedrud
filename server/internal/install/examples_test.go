package install

import (
	"os"
	"path/filepath"
	"testing"
)

func TestInstallDocExamples(t *testing.T) {
	base := t.TempDir()
	setDocExamplesBase(base)
	t.Cleanup(func() {
		setDocExamplesBase("/usr/share/doc/bedrud")
	})

	if err := installDocExamples(); err != nil {
		t.Fatalf("installDocExamples: %v", err)
	}

	for _, name := range []string{"config.yaml.example", "livekit.yaml.example", "README"} {
		p := filepath.Join(docExamplesDir, name)
		st, err := os.Stat(p)
		if err != nil {
			t.Fatalf("missing %s: %v", p, err)
		}
		if st.Size() == 0 {
			t.Fatalf("%s is empty", p)
		}
	}

	// Overwrite is idempotent.
	if err := installDocExamples(); err != nil {
		t.Fatalf("second installDocExamples: %v", err)
	}

	if err := removeDocTree(); err != nil {
		t.Fatalf("removeDocTree: %v", err)
	}
	if _, err := os.Stat(docDir); !os.IsNotExist(err) {
		t.Fatalf("docDir still present: %v", err)
	}
}
