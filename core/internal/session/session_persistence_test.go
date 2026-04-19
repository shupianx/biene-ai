package session

import (
	"testing"

	"biene/internal/permission"
	"biene/internal/permission/webperm"
	"biene/internal/store"
)

func TestPersistentMetaLockedKeepsPendingPermission(t *testing.T) {
	sess := &Session{
		ID:   "sess_test",
		Name: "Test",
		pendingPermission: &PermissionRequestPayload{
			RequestID:   "perm_test",
			Permission:  "write",
			ToolName:    "write_file",
			ToolSummary: "write file",
		},
	}

	meta := sess.persistentMetaLocked()
	if meta.PendingPermission == nil {
		t.Fatalf("expected pending permission to be preserved")
	}
	if meta.PendingPermission.RequestID != "perm_test" {
		t.Fatalf("unexpected request id: %q", meta.PendingPermission.RequestID)
	}
}

func TestSetPendingPermissionPersistsMeta(t *testing.T) {
	st, err := store.Open(t.TempDir())
	if err != nil {
		t.Fatalf("Open store: %v", err)
	}
	defer st.Close()

	sess := &Session{
		ID:    "sess_test",
		Name:  "Test",
		store: st,
	}

	sess.setPendingPermission(webperm.PermissionRequest{
		RequestID:   "perm_test",
		Permission:  "write",
		ToolName:    "write_file",
		ToolSummary: "write file",
	})

	var meta SessionMeta
	if err := st.LoadMeta(&meta); err != nil {
		t.Fatalf("LoadMeta: %v", err)
	}
	if meta.PendingPermission == nil {
		t.Fatalf("expected pending permission in persisted meta")
	}
	if meta.PendingPermission.RequestID != "perm_test" {
		t.Fatalf("unexpected request id: %q", meta.PendingPermission.RequestID)
	}
	if meta.PendingPermission.Expired {
		t.Fatalf("expected live permission request, got expired")
	}
}

func TestResolvePermissionClearsExpiredRequest(t *testing.T) {
	sess := &Session{
		ID:   "sess_test",
		Name: "Test",
		pendingPermission: &PermissionRequestPayload{
			RequestID: "perm_test",
			Expired:   true,
		},
	}

	meta, err := sess.ResolvePermission("perm_test", permission.DecisionDeny)
	if err != nil {
		t.Fatalf("ResolvePermission returned error: %v", err)
	}
	if meta.PendingPermission != nil {
		t.Fatalf("expected expired pending permission to be cleared")
	}
}
