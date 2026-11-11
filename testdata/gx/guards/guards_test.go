package guards_test

import (
	"os"
	"testing"
	"time"

	"example.com/gx/guards"
)

// TestNewMoney sleeps twice and reads the environment, as a test should not.
func TestNewMoney(t *testing.T) {
	time.Sleep(time.Millisecond)
	time.Sleep(time.Millisecond)
	_ = os.Getenv("HOME")
	if _, err := guards.NewMoney(-1); err == nil {
		t.Fatal("want an error")
	}
}

// TestShadowed calls through a local named os, which is not the package.
func TestShadowed(t *testing.T) {
	os := fakeOS{}
	os.Getenv("HOME")
}

type fakeOS struct{}

func (fakeOS) Getenv(string) string { return "" }
