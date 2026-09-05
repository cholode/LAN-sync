package application

import "testing"

func TestNormalizeObjectKey(t *testing.T) {
	tests := []struct {
		name    string
		raw     string
		want    string
		wantErr bool
	}{
		{name: "normal", raw: "2026-09-05/42/a.txt", want: "2026-09-05/42/a.txt"},
		{name: "leading slash", raw: "/2026-09-05/42/a.txt", want: "2026-09-05/42/a.txt"},
		{name: "traversal", raw: "../../secret", wantErr: true},
		{name: "empty", raw: "", wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := normalizeObjectKey(tt.raw)
			if (err != nil) != tt.wantErr {
				t.Fatalf("normalizeObjectKey() error = %v, wantErr %v", err, tt.wantErr)
			}
			if got != tt.want {
				t.Fatalf("normalizeObjectKey() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestObjectKeyBelongsToUser(t *testing.T) {
	if !objectKeyBelongsToUser("2026-09-05/42/a.txt", 42) {
		t.Fatal("expected generated object key to belong to user")
	}
	if objectKeyBelongsToUser("2026-09-05/43/a.txt", 42) {
		t.Fatal("must reject another user's object key")
	}
	if objectKeyBelongsToUser("42/a.txt", 42) {
		t.Fatal("must reject malformed object key")
	}
}
