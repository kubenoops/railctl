package contexts

import (
	"fmt"
	"sort"
	"strings"
)

// Credential returns the credential with the given name, or nil if none
// exists.
func (f *File) Credential(name string) *Credential {
	for i := range f.Credentials {
		if f.Credentials[i].Name == name {
			return &f.Credentials[i]
		}
	}
	return nil
}

// SetCredential upserts a credential. TokenType is detected by the caller
// (one live probe at write time, research D2) and persisted here.
func (f *File) SetCredential(name, token, tokenType string) {
	for i := range f.Credentials {
		if f.Credentials[i].Name == name {
			f.Credentials[i].Token = token
			f.Credentials[i].TokenType = tokenType
			return
		}
	}
	f.Credentials = append(f.Credentials, Credential{Name: name, Token: token, TokenType: tokenType})
}

// DeleteCredential removes a credential unless any context references it;
// a referenced credential is refused with an error naming the referencing
// contexts, so dangling references cannot arise via the CLI (FR-015).
func (f *File) DeleteCredential(name string) error {
	var referencing []string
	for _, ctx := range f.Contexts {
		if ctx.Credentials == name {
			referencing = append(referencing, ctx.Name)
		}
	}
	if len(referencing) > 0 {
		sort.Strings(referencing)
		return fmt.Errorf("credential '%s' is referenced by contexts: %s; delete or update those contexts first",
			name, strings.Join(referencing, ", "))
	}
	for i := range f.Credentials {
		if f.Credentials[i].Name == name {
			f.Credentials = append(f.Credentials[:i], f.Credentials[i+1:]...)
			return nil
		}
	}
	return fmt.Errorf("unknown credential '%s'", name)
}
