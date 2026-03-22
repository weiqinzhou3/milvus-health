package milvus

import "testing"

func TestParseBinlogMetrics_SnakeCasePayload(t *testing.T) {
	t.Parallel()

	payload := `{"quota_metrics":{"total_binlog_size":2800,"collection_binlog_size":{"1001":700,"1002":1000,"1003":1100}}}`

	metrics, err := parseBinlogMetrics(payload)
	if err != nil {
		t.Fatalf("parseBinlogMetrics() error = %v", err)
	}
	if metrics.TotalBinlogSize == nil || *metrics.TotalBinlogSize != 2800 {
		t.Fatalf("TotalBinlogSize = %#v, want 2800", metrics.TotalBinlogSize)
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
	if metrics.TotalBinlogSize == nil || *metrics.TotalBinlogSize != 4509715660 {
		t.Fatalf("TotalBinlogSize = %#v, want 4509715660", metrics.TotalBinlogSize)
	}
	if got := metrics.CollectionBinlogSize[451866866319598777]; got != 2254857830 {
		t.Fatalf("CollectionBinlogSize[451866866319598777] = %d, want 2254857830", got)
	}
}

func TestParseBinlogMetrics_ObservedReal247Payload(t *testing.T) {
	t.Parallel()

	payload := `{
		"nodes_info":[
			{
				"identifier":35,
				"infos":{
					"type":"datacoord",
					"quota_metrics":{
						"TotalBinlogSize":227871396,
						"CollectionBinlogSize":{
							"465017790840049125":227871396
						},
						"PartitionsBinlogSize":{
							"465017790840049125":{
								"465017790840049126":227871396
							}
						},
						"CollectionL0RowCount":{}
					},
					"collection_metrics":{
						"Collections":{
							"465017790840049125":{
								"NumEntitiesTotal":415200
							}
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
	if metrics.TotalBinlogSize == nil || *metrics.TotalBinlogSize != 227871396 {
		t.Fatalf("TotalBinlogSize = %#v, want 227871396", metrics.TotalBinlogSize)
	}
	if got := metrics.CollectionBinlogSize[465017790840049125]; got != 227871396 {
		t.Fatalf("CollectionBinlogSize[465017790840049125] = %d, want 227871396", got)
	}
}

func TestParseBinlogMetrics_MissingTotalKey_PreservesCollectionMap(t *testing.T) {
	t.Parallel()

	payload := `{"quota_metrics":{"collection_binlog_size":{"1001":700}}}`

	metrics, err := parseBinlogMetrics(payload)
	if err != nil {
		t.Fatalf("parseBinlogMetrics() error = %v", err)
	}
	if metrics.TotalBinlogSize != nil {
		t.Fatalf("TotalBinlogSize = %#v, want nil", metrics.TotalBinlogSize)
	}
	if got := metrics.CollectionBinlogSize[1001]; got != 700 {
		t.Fatalf("CollectionBinlogSize[1001] = %d, want 700", got)
	}
}

func TestParseBinlogMetrics_MissingCollectionMap_PreservesTotal(t *testing.T) {
	t.Parallel()

	payload := `{"quota_metrics":{"total_binlog_size":2800}}`

	metrics, err := parseBinlogMetrics(payload)
	if err != nil {
		t.Fatalf("parseBinlogMetrics() error = %v", err)
	}
	if metrics.TotalBinlogSize == nil || *metrics.TotalBinlogSize != 2800 {
		t.Fatalf("TotalBinlogSize = %#v, want 2800", metrics.TotalBinlogSize)
	}
	if len(metrics.CollectionBinlogSize) != 0 {
		t.Fatalf("CollectionBinlogSize = %#v, want empty map", metrics.CollectionBinlogSize)
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
