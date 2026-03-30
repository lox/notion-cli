package mcp

import "testing"

func TestParseUsersResponse(t *testing.T) {
	raw := `{"results":[{"type":"person","id":"user-123","name":"Person Example","email":"person@example.com"}],"has_more":false}`

	users, err := parseUsersResponse(raw)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(users) != 1 {
		t.Fatalf("expected 1 user, got %d", len(users))
	}
	if users[0].Name != "Person Example" {
		t.Fatalf("expected name %q, got %q", "Person Example", users[0].Name)
	}
	if users[0].Person == nil || users[0].Person.Email != "person@example.com" {
		t.Fatalf("expected email %q, got %#v", "person@example.com", users[0].Person)
	}
}
