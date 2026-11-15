package constructor

import (
	"github.com/coroot/coroot/model"
	"github.com/coroot/coroot/timeseries"
)

func opensearch(instance *model.Instance, queryName string, m *model.MetricValues) {
	if instance == nil {
		return
	}
	if instance.Opensearch == nil {
		instance.Opensearch = model.NewOpensearch()
	}
	o := instance.Opensearch
	switch queryName {
	case "opensearch_up":
		o.Up = merge(o.Up, m.Values, timeseries.Any)
	case "opensearch_scrape_error":
		o.Error.Update(m.Values, m.Labels["error"])
	case "opensearch_info":
		o.Version.Update(m.Values, m.Labels["version"])
		o.Roles.Update(m.Values, m.Labels["roles"])
		o.ClusterName.Update(m.Values, m.Labels["cluster"])
	case "opensearch_cluster_status":
		o.ClusterStatus.Update(m.Values, m.Labels["status"])
	case "opensearch_cluster_manager":
		o.ClusterManager = merge(o.ClusterManager, m.Values, timeseries.Any)
	case "opensearch_cluster_nodes":
		o.ClusterNodes = merge(o.ClusterNodes, m.Values, timeseries.Any)
	case "opensearch_cluster_shards":
		s := m.Labels["state"]
		o.ClusterShards[s] = merge(o.ClusterShards[s], m.Values, timeseries.Any)
	case "opensearch_cluster_pending_tasks":
		o.ClusterPendingTasks = merge(o.ClusterPendingTasks, m.Values, timeseries.Any)
	case "opensearch_indices_docs":
		o.Docs = merge(o.Docs, m.Values, timeseries.Any)
	case "opensearch_indices_store_size_bytes":
		o.StoreSize = merge(o.StoreSize, m.Values, timeseries.Any)
	case "opensearch_indexing_ops_total":
		o.IndexingOps = merge(o.IndexingOps, m.Values, timeseries.Any)
	case "opensearch_indexing_seconds_total":
		o.IndexingTime = merge(o.IndexingTime, m.Values, timeseries.Any)
	case "opensearch_indexing_failed_total":
		o.IndexingFailed = merge(o.IndexingFailed, m.Values, timeseries.Any)
	case "opensearch_search_ops_total":
		p := m.Labels["phase"]
		o.SearchOps[p] = merge(o.SearchOps[p], m.Values, timeseries.Any)
	case "opensearch_search_seconds_total":
		p := m.Labels["phase"]
		o.SearchTime[p] = merge(o.SearchTime[p], m.Values, timeseries.Any)
	case "opensearch_jvm_heap_used_bytes":
		o.HeapUsed = merge(o.HeapUsed, m.Values, timeseries.Any)
	case "opensearch_jvm_heap_max_bytes":
		o.HeapMax = merge(o.HeapMax, m.Values, timeseries.Any)
	case "opensearch_jvm_gc_seconds_total":
		gc := m.Labels["gc"]
		o.GcTime[gc] = merge(o.GcTime[gc], m.Values, timeseries.Any)
	case "opensearch_thread_pool_rejected_total":
		p := m.Labels["pool"]
		o.PoolRejected[p] = merge(o.PoolRejected[p], m.Values, timeseries.Any)
	case "opensearch_thread_pool_queue":
		p := m.Labels["pool"]
		o.PoolQueue[p] = merge(o.PoolQueue[p], m.Values, timeseries.Any)
	case "opensearch_breaker_tripped_total":
		b := m.Labels["breaker"]
		o.BreakerTripped[b] = merge(o.BreakerTripped[b], m.Values, timeseries.Any)
	}
}
