package milvus

import (
	"encoding/json"
	"testing"
)

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

func TestConstructMetricsRequest(t *testing.T) {
	t.Parallel()

	req, err := constructMetricsRequest("system_info")
	if err != nil {
		t.Fatalf("constructMetricsRequest() error = %v", err)
	}
	if req.GetRequest() == "" {
		t.Fatal("request payload is empty")
	}

	var payload map[string]string
	if err := json.Unmarshal([]byte(req.GetRequest()), &payload); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}
	if payload["metric_type"] != "system_info" {
		t.Fatalf("metric_type = %q, want system_info", payload["metric_type"])
	}
}
