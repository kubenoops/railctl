package resolver

import (
	"strings"
	"testing"

	"github.com/kubenoops/railctl/internal/types"
)

func TestResolveProject_ExactMatch(t *testing.T) {
	projects := []types.Project{
		{ID: "1", Name: "my-app"},
		{ID: "2", Name: "my-app-staging"},
		{ID: "3", Name: "other"},
	}

	result, err := ResolveProject(projects, "my-app")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.ID != "1" {
		t.Errorf("expected project ID '1', got '%s'", result.ID)
	}
}

func TestResolveProject_SubstringMatch_Single(t *testing.T) {
	projects := []types.Project{
		{ID: "1", Name: "my-app"},
		{ID: "2", Name: "other-service"},
	}

	result, err := ResolveProject(projects, "app")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.ID != "1" {
		t.Errorf("expected project ID '1', got '%s'", result.ID)
	}
}

func TestResolveProject_SubstringMatch_CaseInsensitive(t *testing.T) {
	projects := []types.Project{
		{ID: "1", Name: "MyApp"},
		{ID: "2", Name: "other"},
	}

	result, err := ResolveProject(projects, "myapp")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.ID != "1" {
		t.Errorf("expected project ID '1', got '%s'", result.ID)
	}
}

func TestResolveProject_NotFound(t *testing.T) {
	projects := []types.Project{
		{ID: "1", Name: "my-app"},
	}

	_, err := ResolveProject(projects, "nonexistent")
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	notFound, ok := err.(ErrNotFound)
	if !ok {
		t.Fatalf("expected ErrNotFound, got %T", err)
	}
	if notFound.Name != "nonexistent" {
		t.Errorf("expected name 'nonexistent', got '%s'", notFound.Name)
	}
}

func TestResolveProject_Ambiguous(t *testing.T) {
	projects := []types.Project{
		{ID: "1", Name: "my-app"},
		{ID: "2", Name: "my-app-staging"},
		{ID: "3", Name: "my-app-prod"},
	}

	_, err := ResolveProject(projects, "my-app-")
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	ambiguous, ok := err.(ErrAmbiguous)
	if !ok {
		t.Fatalf("expected ErrAmbiguous, got %T", err)
	}
	if len(ambiguous.Matches) != 2 {
		t.Errorf("expected 2 matches, got %d", len(ambiguous.Matches))
	}
}

func TestResolveProject_EmptyList(t *testing.T) {
	projects := []types.Project{}

	_, err := ResolveProject(projects, "anything")
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	_, ok := err.(ErrNotFound)
	if !ok {
		t.Fatalf("expected ErrNotFound, got %T", err)
	}
}

func TestErrNotFound_Error(t *testing.T) {
	err := ErrNotFound{Resource: "project", Name: "my-app"}
	expected := "project 'my-app' not found"
	if err.Error() != expected {
		t.Errorf("Error() = %q, expected %q", err.Error(), expected)
	}
}

func TestErrAmbiguous_Error(t *testing.T) {
	err := ErrAmbiguous{
		Resource: "project",
		Name:     "api",
		Matches:  []string{"my-api", "other-api"},
	}

	result := err.Error()
	if !strings.Contains(result, "ambiguous") {
		t.Errorf("expected 'ambiguous' in error, got: %s", result)
	}
	if !strings.Contains(result, "my-api") {
		t.Errorf("expected 'my-api' in error, got: %s", result)
	}
	if !strings.Contains(result, "other-api") {
		t.Errorf("expected 'other-api' in error, got: %s", result)
	}
}

// Tests for ResolveEnvironment
func TestResolveEnvironment_ExactMatch(t *testing.T) {
	envs := []types.Environment{
		{ID: "1", Name: "production"},
		{ID: "2", Name: "staging"},
	}

	result, err := ResolveEnvironment(envs, "production")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.ID != "1" {
		t.Errorf("expected ID '1', got '%s'", result.ID)
	}
}

func TestResolveEnvironment_SubstringMatch(t *testing.T) {
	envs := []types.Environment{
		{ID: "1", Name: "production"},
		{ID: "2", Name: "development"},
	}

	result, err := ResolveEnvironment(envs, "prod")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.ID != "1" {
		t.Errorf("expected ID '1', got '%s'", result.ID)
	}
}

func TestResolveEnvironment_IDMatch(t *testing.T) {
	envs := []types.Environment{
		{ID: "env-abc-123", Name: "production"},
		{ID: "env-def-456", Name: "staging"},
	}

	result, err := ResolveEnvironment(envs, "env-def-456")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Name != "staging" {
		t.Errorf("expected name 'staging', got '%s'", result.Name)
	}
}

func TestResolveEnvironment_NameMatchPreferredOverID(t *testing.T) {
	// An environment literally named like another's ID: exact name match wins.
	envs := []types.Environment{
		{ID: "env-1", Name: "env-2"},
		{ID: "env-2", Name: "production"},
	}

	result, err := ResolveEnvironment(envs, "env-2")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.ID != "env-1" {
		t.Errorf("expected name match (ID 'env-1') to win over ID match, got ID '%s'", result.ID)
	}
}

func TestResolveEnvironment_IDMatchEmptyList(t *testing.T) {
	_, err := ResolveEnvironment(nil, "env-abc-123")
	if err == nil {
		t.Fatal("expected error for empty environment list, got nil")
	}
	if _, ok := err.(ErrNotFound); !ok {
		t.Fatalf("expected ErrNotFound, got %T", err)
	}
}

func TestResolveEnvironment_NotFound(t *testing.T) {
	envs := []types.Environment{
		{ID: "1", Name: "production"},
	}

	_, err := ResolveEnvironment(envs, "staging")
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	notFound, ok := err.(ErrNotFound)
	if !ok {
		t.Fatalf("expected ErrNotFound, got %T", err)
	}
	if notFound.Resource != "environment" {
		t.Errorf("expected resource 'environment', got '%s'", notFound.Resource)
	}
}

func TestResolveEnvironment_Ambiguous(t *testing.T) {
	envs := []types.Environment{
		{ID: "1", Name: "dev-us"},
		{ID: "2", Name: "dev-eu"},
	}

	_, err := ResolveEnvironment(envs, "dev")
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	ambiguous, ok := err.(ErrAmbiguous)
	if !ok {
		t.Fatalf("expected ErrAmbiguous, got %T", err)
	}
	if len(ambiguous.Matches) != 2 {
		t.Errorf("expected 2 matches, got %d", len(ambiguous.Matches))
	}
}

// Tests for Resolve (generic resource resolver)
func TestResolve_ExactMatch(t *testing.T) {
	resources := []Resource{
		{ID: "svc-1", Name: "api-service"},
		{ID: "svc-2", Name: "web-service"},
	}

	id, err := Resolve("api-service", resources)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if id != "svc-1" {
		t.Errorf("expected ID 'svc-1', got '%s'", id)
	}
}

func TestResolve_SubstringMatch(t *testing.T) {
	resources := []Resource{
		{ID: "svc-1", Name: "my-api-service"},
		{ID: "svc-2", Name: "web-service"},
	}

	id, err := Resolve("api", resources)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if id != "svc-1" {
		t.Errorf("expected ID 'svc-1', got '%s'", id)
	}
}

func TestResolve_NotFound(t *testing.T) {
	resources := []Resource{
		{ID: "svc-1", Name: "api-service"},
	}

	_, err := Resolve("nonexistent", resources)
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	_, ok := err.(ErrNotFound)
	if !ok {
		t.Fatalf("expected ErrNotFound, got %T", err)
	}
}

func TestResolve_Ambiguous(t *testing.T) {
	resources := []Resource{
		{ID: "svc-1", Name: "api-v1"},
		{ID: "svc-2", Name: "api-v2"},
	}

	_, err := Resolve("api", resources)
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	_, ok := err.(ErrAmbiguous)
	if !ok {
		t.Fatalf("expected ErrAmbiguous, got %T", err)
	}
}

// Tests for ResolveWithName
func TestResolveWithName_ExactMatch(t *testing.T) {
	resources := []Resource{
		{ID: "svc-1", Name: "api-service"},
		{ID: "svc-2", Name: "web-service"},
	}

	id, name, err := ResolveWithName("api-service", resources)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if id != "svc-1" {
		t.Errorf("expected ID 'svc-1', got '%s'", id)
	}
	if name != "api-service" {
		t.Errorf("expected name 'api-service', got '%s'", name)
	}
}

func TestResolveWithName_SubstringMatch(t *testing.T) {
	resources := []Resource{
		{ID: "svc-1", Name: "my-api-service"},
		{ID: "svc-2", Name: "web-service"},
	}

	id, name, err := ResolveWithName("API", resources)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if id != "svc-1" {
		t.Errorf("expected ID 'svc-1', got '%s'", id)
	}
	if name != "my-api-service" {
		t.Errorf("expected name 'my-api-service', got '%s'", name)
	}
}

func TestResolveWithName_NotFound(t *testing.T) {
	resources := []Resource{
		{ID: "svc-1", Name: "api-service"},
	}

	_, _, err := ResolveWithName("nonexistent", resources)
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	notFound, ok := err.(ErrNotFound)
	if !ok {
		t.Fatalf("expected ErrNotFound, got %T", err)
	}
	if notFound.Resource != "resource" {
		t.Errorf("expected resource 'resource', got '%s'", notFound.Resource)
	}
}

func TestResolveWithName_Ambiguous(t *testing.T) {
	resources := []Resource{
		{ID: "svc-1", Name: "api-v1"},
		{ID: "svc-2", Name: "api-v2"},
	}

	_, _, err := ResolveWithName("api", resources)
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	ambiguous, ok := err.(ErrAmbiguous)
	if !ok {
		t.Fatalf("expected ErrAmbiguous, got %T", err)
	}
	if len(ambiguous.Matches) != 2 {
		t.Errorf("expected 2 matches, got %d", len(ambiguous.Matches))
	}
}

// Tests for ResolveService
func TestResolveService_ExactMatch(t *testing.T) {
	services := []types.ServiceDetail{
		{ID: "svc-1", Name: "web-api"},
		{ID: "svc-2", Name: "worker"},
		{ID: "svc-3", Name: "web-frontend"},
	}

	result, err := ResolveService(services, "worker")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.ID != "svc-2" {
		t.Errorf("expected ID 'svc-2', got '%s'", result.ID)
	}
	if result.Name != "worker" {
		t.Errorf("expected name 'worker', got '%s'", result.Name)
	}
}

func TestResolveService_SubstringMatch(t *testing.T) {
	services := []types.ServiceDetail{
		{ID: "svc-1", Name: "web-api"},
		{ID: "svc-2", Name: "worker"},
		{ID: "svc-3", Name: "database"},
	}

	result, err := ResolveService(services, "work")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.ID != "svc-2" {
		t.Errorf("expected ID 'svc-2', got '%s'", result.ID)
	}
}

func TestResolveService_CaseInsensitive(t *testing.T) {
	services := []types.ServiceDetail{
		{ID: "svc-1", Name: "WebAPI"},
		{ID: "svc-2", Name: "Worker"},
	}

	result, err := ResolveService(services, "worker")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.ID != "svc-2" {
		t.Errorf("expected ID 'svc-2', got '%s'", result.ID)
	}
}

func TestResolveService_NotFound(t *testing.T) {
	services := []types.ServiceDetail{
		{ID: "svc-1", Name: "web-api"},
	}

	_, err := ResolveService(services, "database")
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	notFound, ok := err.(ErrNotFound)
	if !ok {
		t.Fatalf("expected ErrNotFound, got %T", err)
	}
	if notFound.Resource != "service" {
		t.Errorf("expected resource 'service', got '%s'", notFound.Resource)
	}
}

func TestResolveService_Ambiguous(t *testing.T) {
	services := []types.ServiceDetail{
		{ID: "svc-1", Name: "web-api"},
		{ID: "svc-2", Name: "web-frontend"},
		{ID: "svc-3", Name: "worker"},
	}

	_, err := ResolveService(services, "web")
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	ambiguous, ok := err.(ErrAmbiguous)
	if !ok {
		t.Fatalf("expected ErrAmbiguous, got %T", err)
	}
	if len(ambiguous.Matches) != 2 {
		t.Errorf("expected 2 matches, got %d", len(ambiguous.Matches))
	}
}

func TestResolveService_EmptyList(t *testing.T) {
	services := []types.ServiceDetail{}

	_, err := ResolveService(services, "anything")
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	_, ok := err.(ErrNotFound)
	if !ok {
		t.Fatalf("expected ErrNotFound, got %T", err)
	}
}
