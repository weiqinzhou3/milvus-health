package milvus

import "testing"

func TestParseBinlogMetrics_SnakeCasePayload(t *testing.T) {
	t.Parallel()

	payload := `{"quota_metrics":{"total_binlog_size":2800,"collection_binlog_size":{"1001":700,"1002":1000,"1003":1100}}}`

	metrics, err := parseBinlogMetrics(payload)
	if err != nil {
		t.Fatalf("parseBinlogMetrics() error = %v", err)
	}
	if metrics.TotalBinlogSize != 2800 {
		t.Fatalf("TotalBinlogSize = %d, want 2800", metrics.TotalBinlogSize)
	}
	if got := metrics.CollectionBinlogSize[1002]; got != 1000 {
		t.Fatalf("CollectionBinlogSize[1002] = %d, want 1000", got)
	}
}

func TestParseBinlogMetrics_Real247NestedPayload(t *testing.T) {
	t.Parallel()

	payload := `{
		"nodes_info":[
			{
				"infos":{
					"quota_metrics":{
						"TotalBinlogSize":"4509715660",
						"CollectionBinlogSize":{
							"451866866319598777":"2254857830",
							"451866866319598778":"2254857830"
						}
					}
				}
			}
		]
	}`

	metrics, err := parseBinlogMetrics(payload)
	if err != nil {
		t.Fatalf("parseBinlogMetrics() error = %v", err)
	}
	if metrics.TotalBinlogSize != 4509715660 {
		t.Fatalf("TotalBinlogSize = %d, want 4509715660", metrics.TotalBinlogSize)
	}
	if got := metrics.CollectionBinlogSize[451866866319598777]; got != 2254857830 {
		t.Fatalf("CollectionBinlogSize[451866866319598777] = %d, want 2254857830", got)
	}
}

func TestParseBinlogMetrics_MissingQuotaMetrics(t *testing.T) {
	t.Parallel()

	payload := `{"nodes_info":[{"infos":{"hardware_infos":{"cpu_core_count":8}}}]}`

	_, err := parseBinlogMetrics(payload)
	if err == nil {
		t.Fatal("parseBinlogMetrics() error = nil, want error")
	}
}

func TestParseBinlogMetrics_MalformedPayload(t *testing.T) {
	t.Parallel()

	payload := `{"quota_metrics":`

	_, err := parseBinlogMetrics(payload)
	if err == nil {
		t.Fatal("parseBinlogMetrics() error = nil, want error")
	}
}
