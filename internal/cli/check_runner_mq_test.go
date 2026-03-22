package cli

import (
	"testing"

	"github.com/weiqinzhou3/milvus-health/internal/model"
)

func TestResolveMQType(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		current   string
		cfg       *model.Config
		inventory model.K8sInventory
		want      string
	}{
		{
			name:    "keeps existing known value",
			current: "pulsar",
			cfg: &model.Config{
				Dependencies: model.DependenciesConfig{
					MQ: model.MQConfig{Type: "kafka"},
				},
			},
			want: "pulsar",
		},
		{
			name: "uses config rocksmq",
			cfg: &model.Config{
				Dependencies: model.DependenciesConfig{
					MQ: model.MQConfig{Type: "rocksmq"},
				},
			},
			want: "rocksmq",
		},
		{
			name: "uses service pulsar signature",
			inventory: model.K8sInventory{
				Services: []model.ServiceInventory{
					{Name: "my-pulsar-proxy"},
				},
			},
			want: "pulsar",
		},
		{
			name: "uses service kafka signature",
			inventory: model.K8sInventory{
				Services: []model.ServiceInventory{
					{Name: "kafka-bootstrap"},
				},
			},
			want: "kafka",
		},
		{
			name: "ambiguous services stay unknown",
			inventory: model.K8sInventory{
				Services: []model.ServiceInventory{
					{Name: "pulsar-proxy"},
					{Name: "kafka-bootstrap"},
				},
			},
			want: "unknown",
		},
		{
			name: "no signal stays unknown",
			cfg:  &model.Config{},
			want: "unknown",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := resolveMQType(tt.current, tt.cfg, tt.inventory); got != tt.want {
				t.Fatalf("resolveMQType() = %q, want %q", got, tt.want)
			}
		})
	}
}
