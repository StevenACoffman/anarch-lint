package arch_test

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/StevenACoffman/anarch-lint/internal/arch"
)

// TestFormat: sets of patterns are sorted and without duplicates, while the
// layers, the rules and the lists whose order means something keep theirs.
func TestFormat(t *testing.T) {
	t.Parallel()
	m, err := arch.DecodeModel(strings.NewReader(`{"version": 1,
		"subsystems": [{"name": "z", "packages": ["m/z/...", "m/a", "m/z/..."]},
			{"name": "a", "packages": ["m/b"]}],
		"subsystemOrder": ["z", "a"],
		"isolations": [{"from": "m/z/...", "mayImport": ["m/z", "m/b"], "reason": "r"},
			{"from": "m/a", "mayImport": ["m/a"], "reason": "r"}],
		"fitness": {"protobuf": [{"name": "p", "files": ["b.proto", "a.proto"],
			"importPaths": ["vendor", "proto"]}]}}`))
	if err != nil {
		t.Fatal(err)
	}
	want := `{
  "version": 1,
  "subsystems": [
    {
      "name": "z",
      "packages": [
        "m/a",
        "m/z/..."
      ]
    },
    {
      "name": "a",
      "packages": [
        "m/b"
      ]
    }
  ],
  "subsystemOrder": [
    "z",
    "a"
  ],
  "isolations": [
    {
      "from": "m/z/...",
      "mayImport": [
        "m/b",
        "m/z"
      ],
      "reason": "r"
    },
    {
      "from": "m/a",
      "mayImport": [
        "m/a"
      ],
      "reason": "r"
    }
  ],
  "fitness": {
    "protobuf": [
      {
        "name": "p",
        "files": [
          "a.proto",
          "b.proto"
        ],
        "importPaths": [
          "vendor",
          "proto"
        ]
      }
    ]
  }
}
`
	equals(t, want, string(m.Format()))
}

// TestFormatKeepsTheHash: on every model in the repository, formatting
// keeps the model's hash, and formatting again changes nothing.
func TestFormatKeepsTheHash(t *testing.T) {
	t.Parallel()
	files, err := filepath.Glob(filepath.Join("..", "..", "testdata", "corpus", "*.json"))
	if err != nil {
		t.Fatal(err)
	}
	files = append(files, filepath.Join("..", "..", "anarch.json"))
	for _, file := range files {
		t.Run(filepath.Base(file), func(t *testing.T) {
			t.Parallel()
			src, err := os.ReadFile(file)
			if err != nil {
				t.Fatal(err)
			}
			m, err := arch.DecodeModel(bytes.NewReader(src))
			if err != nil {
				t.Fatal(err)
			}
			once := m.Format()
			f, err := arch.DecodeModel(bytes.NewReader(once))
			if err != nil {
				t.Fatalf("the formatted model does not decode: %v", err)
			}
			equals(t, m.Hash(), f.Hash())
			equals(t, string(once), string(f.Format()))
		})
	}
}
