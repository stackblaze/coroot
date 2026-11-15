package constructor

import (
	"testing"

	"github.com/coroot/coroot/model"
	"github.com/coroot/coroot/timeseries"
)

func TestPodLabelsOperatorClusters(t *testing.T) {
	values := timeseries.New(0, 2, 15)
	values.Set(0, 1)
	values.Set(15, 1)

	cases := []struct {
		name    string
		labels  model.Labels
		cluster string
	}{
		{
			name:    "clickhouse replica",
			labels:  model.Labels{"label_clickhouse_altinity_com_chi": "ch"},
			cluster: "ch",
		},
		{
			name: "clickhouse keeper carrying the installation label",
			labels: model.Labels{
				"label_clickhouse_keeper_altinity_com_chk": "ch-k",
				"label_clickhouse_altinity_com_chi":        "ch",
			},
			cluster: "ch-k",
		},
		{
			name: "opensearch node",
			labels: model.Labels{
				"label_opensearch_org_opensearch_cluster":  "search",
				"label_opensearch_org_opensearch_nodepool": "nodes",
			},
			cluster: "search",
		},
		{
			name:    "opensearch job pod (no node pool)",
			labels:  model.Labels{"label_opensearch_org_opensearch_cluster": "search"},
			cluster: "",
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			instance := &model.Instance{}
			labels := model.Labels{"uid": "u1"}
			for k, v := range c.labels {
				labels[k] = v
			}
			podLabels([]*model.MetricValues{{Labels: labels, Values: values}}, map[string]*model.Instance{"u1": instance})
			if got := instance.ClusterName.Value(); got != c.cluster {
				t.Fatalf("cluster = %q, want %q", got, c.cluster)
			}
		})
	}
}
