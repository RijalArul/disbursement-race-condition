package audit

import "testing"

func TestAuditOrderClauseWhitelist(t *testing.T) {
	tests := []struct {
		name      string
		sortBy    string
		sortOrder string
		want      string
	}{
		{"allowed column and order pass through", "created_at", "asc", "created_at asc"},
		{"unknown column falls back to created_at", "actor_id", "asc", "created_at asc"},
		{"sql injection attempt falls back to created_at", "id; DROP TABLE audit_logs;--", "asc", "created_at asc"},
		{"unknown order falls back to desc", "created_at", "sideways", "created_at desc"},
		{"empty inputs fall back to defaults", "", "", "created_at desc"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := auditOrderClause(tt.sortBy, tt.sortOrder); got != tt.want {
				t.Errorf("auditOrderClause(%q, %q) = %q, want %q", tt.sortBy, tt.sortOrder, got, tt.want)
			}
		})
	}
}
