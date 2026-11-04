package contexts

import (
	"strings"
	"testing"
)

func strPtr(s string) *string { return &s }

// Creating a context requires a project; the credential reference, when
// given, must name an existing credential (FR-005).
func TestUpsertContext_CreateValidation(t *testing.T) {
	t.Run("create without project fails", func(t *testing.T) {
		f := &File{}
		err := f.UpsertContext("shop", ContextUpdate{Project: nil})
		if err == nil || !strings.Contains(err.Error(), "project is required") {
			t.Fatalf("expected project-required error, got: %v", err)
		}
	})

	t.Run("create with project succeeds", func(t *testing.T) {
		f := &File{}
		err := f.UpsertContext("shop", ContextUpdate{Project: strPtr("shop")})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(f.Contexts) != 1 || f.Contexts[0].Project != "shop" {
			t.Fatalf("context not created correctly: %+v", f.Contexts)
		}
	})

	t.Run("unknown credential reference fails naming existing", func(t *testing.T) {
		f := &File{Credentials: []Credential{{Name: "work", Token: "t", TokenType: "account"}}}
		err := f.UpsertContext("shop", ContextUpdate{Project: strPtr("shop"), Credentials: strPtr("ghost")})
		if err == nil {
			t.Fatal("expected unknown-credential error, got nil")
		}
		if !strings.Contains(err.Error(), "ghost") || !strings.Contains(err.Error(), "work") {
			t.Fatalf("error must name unknown and existing credentials, got: %v", err)
		}
	})

	t.Run("valid credential reference succeeds", func(t *testing.T) {
		f := &File{Credentials: []Credential{{Name: "work", Token: "t", TokenType: "account"}}}
		err := f.UpsertContext("shop", ContextUpdate{Project: strPtr("shop"), Credentials: strPtr("work")})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if f.Contexts[0].Credentials != "work" {
			t.Fatalf("credential reference not set: %+v", f.Contexts[0])
		}
	})
}

// Updating an existing context must change only the fields whose flags were
// provided; omitted fields keep their values (FR-006, kubectl upsert).
func TestUpsertContext_UpdateOnlyFlaggedFields(t *testing.T) {
	f := &File{Contexts: []Context{{
		Name: "shop", Workspace: "team", Project: "shop", Environment: "production",
	}}}

	err := f.UpsertContext("shop", ContextUpdate{Environment: strPtr("staging")})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	got := f.Contexts[0]
	want := Context{Name: "shop", Workspace: "team", Project: "shop", Environment: "staging"}
	if got != want {
		t.Fatalf("only environment must change:\n got %+v\nwant %+v", got, want)
	}
	if len(f.Contexts) != 1 {
		t.Fatalf("update must not create a second context: %+v", f.Contexts)
	}
}

// Setting the active context with an unknown name must fail listing the
// available contexts and leave the file unmutated (FR-010).
func TestSetActive_UnknownNameListsAvailableNoMutation(t *testing.T) {
	f := &File{Contexts: []Context{{Name: "a", Project: "p1"}, {Name: "b", Project: "p2"}}}

	err := f.SetActive("nope")

	if err == nil || !strings.Contains(err.Error(), "a") || !strings.Contains(err.Error(), "b") {
		t.Fatalf("expected error listing available contexts, got: %v", err)
	}
	if f.CurrentContext != "" {
		t.Fatalf("file must stay unmutated, current=%q", f.CurrentContext)
	}
}

func TestSetActive_SetsCurrent(t *testing.T) {
	f := &File{Contexts: []Context{{Name: "a", Project: "p1"}}}
	if err := f.SetActive("a"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if f.CurrentContext != "a" {
		t.Fatalf("current not set: %q", f.CurrentContext)
	}
}

// Deleting the active context must clear the active slot and report it;
// deleting an inactive context must not touch the slot (FR-011). Unknown
// names fail listing available contexts.
func TestDeleteContext_ActiveSlotHandling(t *testing.T) {
	f := &File{
		CurrentContext: "a",
		Contexts:       []Context{{Name: "a", Project: "p1"}, {Name: "b", Project: "p2"}},
	}

	cleared, err := f.DeleteContext("a")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !cleared {
		t.Fatal("deleting the active context must report cleared=true")
	}
	if f.CurrentContext != "" {
		t.Fatalf("active slot must be cleared, got %q", f.CurrentContext)
	}
	if len(f.Contexts) != 1 || f.Contexts[0].Name != "b" {
		t.Fatalf("context not removed: %+v", f.Contexts)
	}

	f.CurrentContext = "b"
	cleared, err = f.DeleteContext("b")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !cleared {
		t.Fatal("deleting active 'b' must also report cleared=true")
	}

	if _, err = f.DeleteContext("ghost"); err == nil || !strings.Contains(err.Error(), "available") {
		t.Fatalf("expected unknown-context error listing available, got: %v", err)
	}
}
