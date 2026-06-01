package tools

import "testing"

func TestCanonicalToolSpecs_CoversPortedTools(t *testing.T) {
	want := map[string]PermissionMode{
		"bash":        PermissionDangerFullAccess,
		"read_file":   PermissionReadOnly,
		"write_file":  PermissionWorkspaceWrite,
		"edit_file":   PermissionWorkspaceWrite,
		"glob_search": PermissionReadOnly,
		"grep_search": PermissionReadOnly,
		"web":         PermissionReadOnly,
		"notebook":    PermissionWorkspaceWrite,
		"agent":       PermissionDangerFullAccess,
		"skill":       PermissionReadOnly,
	}

	registry := NewToolRegistry(nil)
	specs := registry.List()
	if len(specs) != len(want) {
		t.Fatalf("got %d specs, want %d", len(specs), len(want))
	}
	for name, perm := range want {
		spec, ok := registry.Find(name)
		if !ok {
			t.Errorf("missing tool %q", name)
			continue
		}
		if spec.RequiredPermission != perm {
			t.Errorf("tool %q permission=%s want %s", name, spec.RequiredPermission, perm)
		}
		if spec.InputSchema == nil {
			t.Errorf("tool %q missing input schema", name)
		}
		if spec.InputSchema["type"] != "object" {
			t.Errorf("tool %q schema type=%v want object", name, spec.InputSchema["type"])
		}
	}
}

func TestPermissionMode_Allows(t *testing.T) {
	if !PermissionDangerFullAccess.Allows(PermissionWorkspaceWrite) {
		t.Error("danger-full-access should satisfy workspace-write")
	}
	if !PermissionWorkspaceWrite.Allows(PermissionReadOnly) {
		t.Error("workspace-write should satisfy read-only")
	}
	if PermissionReadOnly.Allows(PermissionWorkspaceWrite) {
		t.Error("read-only must not satisfy workspace-write")
	}
	if PermissionReadOnly.Allows(PermissionDangerFullAccess) {
		t.Error("read-only must not satisfy danger-full-access")
	}
}
