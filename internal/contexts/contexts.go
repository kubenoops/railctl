package contexts

import (
	"fmt"
	"sort"
	"strings"
)

// ContextUpdate carries explicitly-provided fields for upsert. A nil pointer
// means "field not provided" — on update the existing value is kept (FR-006);
// pointers let the CLI distinguish an omitted flag from an empty value.
type ContextUpdate struct {
	Workspace   *string
	Project     *string
	Environment *string
	Credentials *string
}

// Context returns the context with the given name, or nil if none exists.
func (f *File) Context(name string) *Context {
	for i := range f.Contexts {
		if f.Contexts[i].Name == name {
			return &f.Contexts[i]
		}
	}
	return nil
}

// ContextNames returns all context names, sorted for stable messages.
func (f *File) ContextNames() []string {
	names := make([]string, len(f.Contexts))
	for i, c := range f.Contexts {
		names[i] = c.Name
	}
	sort.Strings(names)
	return names
}

// UpsertContext creates or updates the named context (FR-005, FR-006).
// Creating requires Project. A provided Credentials reference must name an
// existing credential, on create and update alike.
func (f *File) UpsertContext(name string, u ContextUpdate) error {
	if u.Credentials != nil && *u.Credentials != "" && f.Credential(*u.Credentials) == nil {
		return fmt.Errorf("credential '%s' does not exist; available credentials: %s",
			*u.Credentials, strings.Join(credentialNames(f), ", "))
	}

	existing := f.Context(name)
	if existing == nil {
		if u.Project == nil || *u.Project == "" {
			return fmt.Errorf("project is required when creating a context (-p/--project)")
		}
		ctx := Context{Name: name, Project: *u.Project}
		f.Contexts = append(f.Contexts, ctx)
		existing = &f.Contexts[len(f.Contexts)-1]
	}

	if u.Workspace != nil {
		existing.Workspace = *u.Workspace
	}
	if u.Project != nil {
		existing.Project = *u.Project
	}
	if u.Environment != nil {
		existing.Environment = *u.Environment
	}
	if u.Credentials != nil {
		existing.Credentials = *u.Credentials
	}
	return nil
}

// SetActive marks the named context active. It is a pure in-memory change —
// callers persist it with Save; no API validation ever happens here
// (FR-010).
func (f *File) SetActive(name string) error {
	if f.Context(name) == nil {
		return fmt.Errorf("unknown context '%s'; available contexts: %s",
			name, strings.Join(f.ContextNames(), ", "))
	}
	f.CurrentContext = name
	return nil
}

// DeleteContext removes the named context. When the active context is
// removed, the active slot is cleared and clearedActive reports true so the
// CLI can emit its notice (FR-011).
func (f *File) DeleteContext(name string) (clearedActive bool, err error) {
	idx := -1
	for i := range f.Contexts {
		if f.Contexts[i].Name == name {
			idx = i
			break
		}
	}
	if idx == -1 {
		return false, fmt.Errorf("unknown context '%s'; available contexts: %s",
			name, strings.Join(f.ContextNames(), ", "))
	}
	f.Contexts = append(f.Contexts[:idx], f.Contexts[idx+1:]...)
	if f.CurrentContext == name {
		f.CurrentContext = ""
		return true, nil
	}
	return false, nil
}

// credentialNames returns all credential names, sorted for stable messages.
func credentialNames(f *File) []string {
	names := make([]string, len(f.Credentials))
	for i, c := range f.Credentials {
		names[i] = c.Name
	}
	sort.Strings(names)
	return names
}
