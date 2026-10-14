package linter

import "testing"

func TestTopologicalSort_DisabledRequirementSkipped(t *testing.T) {
	dep := &Rule{Name: "dep"}
	user := &Rule{Name: "user", Requires: []*Rule{dep}}
	other := &Rule{Name: "other"}

	// dep is not in the list (disabled): user must still be scheduled.
	got, err := topologicalSort([]*Rule{user, other})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("got %d rules, want 2", len(got))
	}

	// When present, ordering is still honored.
	got, err = topologicalSort([]*Rule{user, dep})
	if err != nil {
		t.Fatal(err)
	}
	if got[0].Name != "dep" || got[1].Name != "user" {
		t.Errorf("order = %s,%s; want dep,user", got[0].Name, got[1].Name)
	}
}
