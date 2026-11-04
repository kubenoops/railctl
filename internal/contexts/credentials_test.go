package contexts

import (
	"strings"
	"testing"
)

// SetCredential upserts name+token+tokenType (FR-012 domain half; type is
// detected by the CLI before calling this).
func TestSetCredential_Upsert(t *testing.T) {
	f := &File{}

	f.SetCredential("work", "tok-1", "workspace")
	if got := f.Credential("work"); got == nil || got.Token != "tok-1" || got.TokenType != "workspace" {
		t.Fatalf("credential not stored correctly: %+v", got)
	}

	f.SetCredential("work", "tok-2", "account")
	if got := f.Credential("work"); got == nil || got.Token != "tok-2" || got.TokenType != "account" {
		t.Fatalf("credential not updated in place: %+v", got)
	}
	if len(f.Credentials) != 1 {
		t.Fatalf("upsert must not duplicate: %+v", f.Credentials)
	}
}

// Deleting a credential referenced by any context must be refused with an
// error naming the referencing contexts; unreferenced credentials delete
// normally (FR-015).
func TestDeleteCredential_RefusedWhileReferenced(t *testing.T) {
	f := &File{
		Contexts: []Context{
			{Name: "shop-prod", Project: "shop", Credentials: "work"},
			{Name: "shop-staging", Project: "shop", Credentials: "work"},
		},
		Credentials: []Credential{{Name: "work", Token: "t", TokenType: "account"}},
	}

	err := f.DeleteCredential("work")

	if err == nil {
		t.Fatal("expected refusal error, got nil")
	}
	for _, want := range []string{"shop-prod", "shop-staging"} {
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("error must name referencing context %q, got: %v", want, err)
		}
	}
	if f.Credential("work") == nil {
		t.Fatal("refused delete must keep the credential")
	}

	if err := f.DeleteCredential("unused"); err == nil {
		t.Fatal("expected unknown-credential error, got nil")
	}
}
