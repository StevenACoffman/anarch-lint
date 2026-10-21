package gopackages_test

import (
	"context"
	"os"
	"runtime"
	"testing"

	"github.com/StevenACoffman/anarch-lint/internal/gopackages"
)

// The platform is the one the go command builds for: the environment's
// GOOS and GOARCH, which default to the toolchain's own.
func TestPlatform(t *testing.T) {
	t.Parallel()
	cases := map[string]struct {
		env  []string
		want string
	}{
		"another platform": {
			env:  append(os.Environ(), "GOOS=plan9", "GOARCH=386"),
			want: "plan9/386",
		},
		"the default": {
			env:  append(os.Environ(), "GOOS=", "GOARCH="),
			want: runtime.GOOS + "/" + runtime.GOARCH,
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			got, err := gopackages.Platform(context.Background(), fixtureDir, tc.env)
			if err != nil {
				t.Fatal(err)
			}
			equals(t, tc.want, got)
		})
	}
}
