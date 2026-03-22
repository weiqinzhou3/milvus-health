package milvus

import "testing"

func TestParseCollectionRowCount(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		stats   map[string]string
		want    int64
		wantErr bool
	}{
		{
			name:  "valid",
			stats: map[string]string{"row_count": "123"},
			want:  123,
		},
		{
			name:    "missing row_count",
			stats:   map[string]string{"other": "1"},
			wantErr: true,
		},
		{
			name:    "invalid row_count",
			stats:   map[string]string{"row_count": "oops"},
			wantErr: true,
		},
		{
			name:    "negative row_count",
			stats:   map[string]string{"row_count": "-1"},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := parseCollectionRowCount(tt.stats)
			if (err != nil) != tt.wantErr {
				t.Fatalf("parseCollectionRowCount() error = %v, wantErr %v", err, tt.wantErr)
			}
			if err == nil && got != tt.want {
				t.Fatalf("parseCollectionRowCount() = %d, want %d", got, tt.want)
			}
		})
	}
}
