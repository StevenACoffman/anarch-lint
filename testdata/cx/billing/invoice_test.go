package billing

import "testing"

// TestApprove is the test that enforces an invariant: read from source.
//
//arch:invariant admin-approves
func TestApprove(t *testing.T) {
	if !Approve(Invoice{Role: "admin"}) {
		t.Fatal("an admin's invoice is not approved")
	}
}
