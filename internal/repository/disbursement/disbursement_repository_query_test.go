package disbursement

import "testing"

func TestOrderClauseWhitelist(t *testing.T) {
	tests := []struct {
		name      string
		sortBy    string
		sortOrder string
		want      string
	}{
		{"allowed column and order pass through", "amount", "asc", "amount asc, id asc"},
		{"status is allowed", "status", "desc", "status desc, id desc"},
		{"unknown column falls back to created_at", "internal_notes", "asc", "created_at asc, id asc"},
		{"sql injection attempt falls back to created_at", "id; DROP TABLE disbursements;--", "asc", "created_at asc, id asc"},
		{"unknown order falls back to desc", "amount", "sideways", "amount desc, id desc"},
		{"empty inputs fall back to defaults", "", "", "created_at desc, id desc"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := orderClause(tt.sortBy, tt.sortOrder); got != tt.want {
				t.Errorf("orderClause(%q, %q) = %q, want %q", tt.sortBy, tt.sortOrder, got, tt.want)
			}
		})
	}
}
