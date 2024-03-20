package prometheus

import (
	"github.com/prometheus/client_golang/prometheus/testutil/promlint"
	dto "github.com/prometheus/client_model/go"

	"k8s.io/apimachinery/pkg/util/sets"
)

// exceptionMetrics is an exception list of metrics which violates promlint
// rules.
//
// The original entries come from the existing metrics when we introduced
// promlint.
// We setup this list to allow and not fail on the current violations.
// Generally speaking, you need to fix the problem for a new metric rather than
// add it into the list.
var exceptionMetrics = sets.NewString(
	// library-go
	"event_recorder_total_events_count",

	// k8s.io/apiextensions-apiserver/pkg/controller/openapi
	"apiextensions_openapi_v2_regeneration_count",

	// k8s.io/kubernetes/pkg/controller/volume/persistentvolume/metrics
	"pv_collector_bound_pv_count",
	"pv_collector_bound_pvc_count",
	"pv_collector_unbound_pv_count",
	"pv_collector_unbound_pvc_count",
	"volume_operation_total_errors",

	// default/apiserver
	"aggregator_openapi_v2_regeneration_count",
	"apiserver_admission_step_admission_duration_seconds_summary",
	"apiserver_admission_webhook_rejection_count",
	"apiserver_current_inflight_requests",
	"apiserver_flowcontrol_current_executing_requests",
	"apiserver_flowcontrol_current_inqueue_requests",
	"apiserver_flowcontrol_dispatched_requests_total",
	"apiserver_flowcontrol_request_concurrency_limit",
	"apiserver_longrunning_gauge",
	"apiserver_request_total",
	"authenticated_user_requests",
	"authentication_attempts",
	"authentication_token_cache_active_fetch_count",
	"get_token_count",
	"get_token_fail_count",
	"ssh_tunnel_open_count",
	"ssh_tunnel_open_fail_count",

	// kube-system/kubelet
	"cadvisor_version_info",
	"container_fs_inodes_total",
	"container_memory_failcnt",
	"kubelet_certificate_manager_client_expiration_renew_errors",
	"kubelet_pleg_discard_events",
	"kubelet_running_container_count",
	"kubelet_running_pod_count",
	"kubelet_server_expiration_renew_errors",
	"storage_operation_status_count",

	// kube-system/crio
	"container_runtime_crio_image_pulls_by_digest",
	"container_runtime_crio_image_pulls_by_name",
	"container_runtime_crio_image_pulls_by_name_skipped",
	"container_runtime_crio_image_pulls_failures",
	"container_runtime_crio_image_pulls_successes",
	"container_runtime_crio_operations",
	"container_runtime_crio_operations_errors",
	"container_runtime_crio_operations_latency_microseconds",

	// openshift-apiserver/check-endpoints
	"openshift_apiserver_build_info",
	"openshift_apiserver_endpoint_check_count",
	"openshift_apiserver_endpoint_check_tcp_connect_latency_gauge",

	// openshift-authentication/oauth-openshift
	"openshift_auth_basic_password_count",
	"openshift_auth_basic_password_count_result",
	"openshift_auth_form_password_count",
	"openshift_auth_form_password_count_result",

	// openshift-authentication-operator/metrics
	"openshift_authentication_operator_build_info",

	// openshift-config-operator/metrics
	"openshift_config_operator_build_info",

	// openshift-controller-manager/controller-manager
	"openshift_apps_deploymentconfigs_active_rollouts_duration_seconds",
	"openshift_apps_deploymentconfigs_complete_rollouts_total",
	"openshift_apps_deploymentconfigs_strategy_total",
	"openshift_build_total",
	"openshift_imagestreamcontroller_error_count",
	"openshift_imagestreamcontroller_success_count",

	// openshift-etcd-operator/metrics
	"openshift_etcd_operator_build_info",

	// openshift-etcd/etcd
	"etcd_debugging_lease_ttl_total",
	"etcd_debugging_mvcc_db_compaction_pause_duration_milliseconds",
	"etcd_debugging_mvcc_db_compaction_total_duration_milliseconds",
	"etcd_debugging_mvcc_index_compaction_pause_duration_milliseconds",
	"etcd_debugging_mvcc_keys_total",
	"etcd_debugging_mvcc_pending_events_total",
	"etcd_debugging_mvcc_slow_watcher_total",
	"etcd_debugging_mvcc_watch_stream_total",
	"etcd_debugging_mvcc_watcher_total",
	"etcd_disk_wal_write_bytes_total",
	"etcd_grpc_proxy_cache_hits_total",
	"etcd_grpc_proxy_cache_keys_total",
	"etcd_grpc_proxy_cache_misses_total",
	"etcd_grpc_proxy_watchers_coalescing_total",
	"etcd_server_health_failures",
	"etcd_server_health_success",
	"etcd_server_learner_promote_successes",
	"etcd_server_proposals_applied_total",
	"etcd_server_proposals_committed_total",
	"etcd_server_snapshot_apply_in_progress_total",

	// openshift-image-registry/image-registry
	"imageregistry_build_info",

	// openshift-ingress/router-internal-default
	"haproxy_backend_bytes_in_total",
	"haproxy_backend_bytes_out_total",
	"haproxy_backend_connection_errors_total",
	"haproxy_backend_connections_reused_total",
	"haproxy_backend_connections_total",
	"haproxy_backend_http_average_connect_latency_milliseconds",
	"haproxy_backend_http_average_queue_latency_milliseconds",
	"haproxy_backend_http_average_response_latency_milliseconds",
	"haproxy_backend_http_responses_total",
	"haproxy_backend_response_errors_total",
	"haproxy_exporter_csv_parse_failures",
	"haproxy_exporter_total_scrapes",
	"haproxy_frontend_bytes_in_total",
	"haproxy_frontend_bytes_out_total",
	"haproxy_frontend_connections_total",
	"haproxy_frontend_http_responses_total",
	"haproxy_server_bytes_in_total",
	"haproxy_server_bytes_out_total",
	"haproxy_server_check_failures_total",
	"haproxy_server_connection_errors_total",
	"haproxy_server_connections_reused_total",
	"haproxy_server_connections_total",
	"haproxy_server_downtime_seconds_total",
	"haproxy_server_http_average_connect_latency_milliseconds",
	"haproxy_server_http_average_queue_latency_milliseconds",
	"haproxy_server_http_average_response_latency_milliseconds",
	"haproxy_server_http_responses_total",
	"haproxy_server_response_errors_total",

	// openshift-insights/metrics
	"apiserver_storage_data_key_generation_latencies_microseconds",

	// openshift-kube-apiserver-operator/metrics
	"openshift_kube_apiserver_operator_build_info",
	"openshift_kube_apiserver_termination_count",
	"openshift_kube_apiserver_termination_event_time",

	// openshift-kube-controller-manager/kube-controller-manager
	"attachdetach_controller_forced_detaches",
	"cloudprovider_gce_api_request_errors",
	"endpoint_slice_controller_changes",
	"node_collector_evictions_number",
	"node_ipam_controller_cidrset_cidrs_allocations_total",
	"node_ipam_controller_cidrset_usage_cidrs",
	"openshift_kube_controller_manager_operator_build_info",

	// openshift-kube-scheduler-operator/metrics
	"openshift_kube_scheduler_operator_build_info",

	// openshift-kube-scheduler/scheduler
	"scheduler_total_preemption_attempts",

	// openshift-machine-config-operator/machine-config-daemon
	"ssh_accessed",

	// openshift-monitoring/grafana
	"grafana_alerting_execution_time_milliseconds",
	"grafana_api_dashboard_get_milliseconds",
	"grafana_api_dashboard_save_milliseconds",
	"grafana_api_dashboard_search_milliseconds",
	"grafana_api_dataproxy_request_all_milliseconds",

	// openshift-monitoring/kube-state-metrics
	"kube_pod_labels",
	"kube_replicaset_labels",
	"kube_storageclass_info",

	// openshift-monitoring/node-exporter
	"node_entropy_available_bits",
	"node_memory_AnonHugePages_bytes",
	"node_memory_AnonPages_bytes",
	"node_memory_CommitLimit_bytes",
	"node_memory_DirectMap1G_bytes",
	"node_memory_DirectMap2M_bytes",
	"node_memory_DirectMap4k_bytes",
	"node_memory_HardwareCorrupted_bytes",
	"node_memory_HugePages_Free",
	"node_memory_HugePages_Rsvd",
	"node_memory_HugePages_Surp",
	"node_memory_HugePages_Total",
	"node_memory_KernelStack_bytes",
	"node_memory_MemAvailable_bytes",
	"node_memory_MemFree_bytes",
	"node_memory_MemTotal_bytes",
	"node_memory_PageTables_bytes",
	"node_memory_ShmemHugePages_bytes",
	"node_memory_ShmemPmdMapped_bytes",
	"node_memory_SwapCached_bytes",
	"node_memory_SwapFree_bytes",
	"node_memory_SwapTotal_bytes",
	"node_memory_VmallocChunk_bytes",
	"node_memory_VmallocTotal_bytes",
	"node_memory_VmallocUsed_bytes",
	"node_memory_WritebackTmp_bytes",
	"node_netstat_Icmp6_InErrors",
	"node_netstat_Icmp6_InMsgs",
	"node_netstat_Icmp6_OutMsgs",
	"node_netstat_Icmp_InErrors",
	"node_netstat_Icmp_InMsgs",
	"node_netstat_Icmp_OutMsgs",
	"node_netstat_Ip6_InOctets",
	"node_netstat_Ip6_OutOctets",
	"node_netstat_IpExt_InOctets",
	"node_netstat_IpExt_OutOctets",
	"node_netstat_TcpExt_ListenDrops",
	"node_netstat_TcpExt_ListenOverflows",
	"node_netstat_TcpExt_SyncookiesFailed",
	"node_netstat_TcpExt_SyncookiesRecv",
	"node_netstat_TcpExt_SyncookiesSent",
	"node_netstat_TcpExt_TCPSynRetrans",
	"node_netstat_Tcp_ActiveOpens",
	"node_netstat_Tcp_CurrEstab",
	"node_netstat_Tcp_InErrs",
	"node_netstat_Tcp_InSegs",
	"node_netstat_Tcp_OutSegs",
	"node_netstat_Tcp_PassiveOpens",
	"node_netstat_Tcp_RetransSegs",
	"node_netstat_Udp6_InDatagrams",
	"node_netstat_Udp6_InErrors",
	"node_netstat_Udp6_NoPorts",
	"node_netstat_Udp6_OutDatagrams",
	"node_netstat_Udp6_RcvbufErrors",
	"node_netstat_Udp6_SndbufErrors",
	"node_netstat_UdpLite6_InErrors",
	"node_netstat_UdpLite_InErrors",
	"node_netstat_Udp_InDatagrams",
	"node_netstat_Udp_InErrors",
	"node_netstat_Udp_NoPorts",
	"node_netstat_Udp_OutDatagrams",
	"node_netstat_Udp_RcvbufErrors",
	"node_netstat_Udp_SndbufErrors",

	// openshift-monitoring/openshift-state-metrics
	"openshift_build_status_phase_total",

	// openshift-operator-lifecycle-manager/catalog-operator-metrics
	"catalog_source_count",
	"install_plan_count",
	"subscription_count",

	// openshift-operator-lifecycle-manager/olm-operator-metrics
	"csv_count",
	"csv_upgrade_count",

	// openshift-sdn/sdn
	"openshift_sdn_vnid_not_found_errors",
	"openshift_sdn_ovs_operations",
	"openshift_sdn_pod_operations_errors",

	// openshift-service-ca-operator/metrics
	"openshift_service_ca_operator_build_info",
)

// A Problem is an issue detected by a Linter.
type Problem promlint.Problem

// problemSet is a set of Problem, implemented via map[Problem]struct{} for
// minimal memory consumption.
type problemSet map[Problem]sets.Empty

// newProblemSet creates a ProblemSet from a list of promlint.Problem.
func newProblemSet(problems ...promlint.Problem) problemSet {
	set := make(problemSet, len(problems))
	for _, problem := range problems {
		problem := Problem(problem)
		if _, ok := set[problem]; !ok {
			set[problem] = sets.Empty{}
		}
	}
	return set
}

// A Linter is a Prometheus metrics linter. It identifies issues with metric
// names, types, and metadata, and reports them to the caller.
type Linter struct {
	promLinter *promlint.Linter
}

// Lint performs a linting pass, returning a slice of Problems indicating any
// issues found in the metrics stream. The slice is sorted by metric name
// and issue description.
func (l *Linter) Lint() ([]Problem, error) {
	promProblems, err := l.promLinter.Lint()
	if err != nil {
		return nil, err
	}

	problemSet := newProblemSet(promProblems...)
	problems := make([]Problem, 0, len(problemSet))

	// Ignore metrics in exception list
	for problem := range problemSet {
		if !exceptionMetrics.Has(problem.Metric) {
			problems = append(problems, problem)
		}
	}

	return problems, nil
}

// NewPromLinterWithMetricFamilies creates a new Linter that reads from a slice
// of MetricFamily protobuf messages.
func NewPromLinterWithMetricFamilies(families []*dto.MetricFamily) *Linter {
	return &Linter{
		promLinter: promlint.NewWithMetricFamilies(families),
	}
}
