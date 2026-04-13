package mapper

import "github.com/yuuki/lustre_exporter/internal/parser"

// MetricDef defines a public metric's name, help text, type, and expected labels.
type MetricDef struct {
	Name      string
	Help      string
	Type      parser.MetricType
	LabelKeys []string
}

// Registry maps internal MetricIDs to their public GSI-HPC compatible definitions.
var Registry = map[string]MetricDef{
	// Health
	"health_check": {
		Name:      "lustre_health_check",
		Help:      "Lustre filesystem health status (1 = healthy, 0 = unhealthy).",
		Type:      parser.Gauge,
		LabelKeys: nil,
	},

	// SPTLRPC encrypt_page_pools
	"physical_pages": {
		Name: "lustre_physical_pages",
		Help: "Total physical pages available for encryption page pools.",
		Type: parser.Gauge,
	},
	"pages_per_pool": {
		Name: "lustre_pages_per_pool",
		Help: "Number of pages allocated per encryption pool.",
		Type: parser.Gauge,
	},
	"maximum_pages": {
		Name: "lustre_maximum_pages",
		Help: "Maximum number of pages allowed in encryption pools.",
		Type: parser.Gauge,
	},
	"maximum_pools": {
		Name: "lustre_maximum_pools",
		Help: "Maximum number of encryption page pools.",
		Type: parser.Gauge,
	},
	"pages_in_pools": {
		Name: "lustre_pages_in_pools",
		Help: "Current number of pages in encryption pools.",
		Type: parser.Gauge,
	},
	"free_pages": {
		Name: "lustre_free_pages",
		Help: "Number of free pages in encryption pools.",
		Type: parser.Gauge,
	},
	"maximum_pages_reached_total": {
		Name: "lustre_maximum_pages_reached_total",
		Help: "Number of times the encryption pool page limit was reached.",
		Type: parser.Counter,
	},
	"grows_total": {
		Name: "lustre_grows_total",
		Help: "Number of times encryption page pools have grown.",
		Type: parser.Counter,
	},
	"grows_failure_total": {
		Name: "lustre_grows_failure_total",
		Help: "Number of times encryption page pool growth has failed.",
		Type: parser.Counter,
	},
	"shrinks_total": {
		Name: "lustre_shrinks_total",
		Help: "Number of times encryption page pools have shrunk.",
		Type: parser.Counter,
	},
	"cache_access_total": {
		Name: "lustre_cache_access_total",
		Help: "Total number of encryption page pool cache accesses.",
		Type: parser.Counter,
	},
	"cache_miss_total": {
		Name: "lustre_cache_miss_total",
		Help: "Total number of encryption page pool cache misses.",
		Type: parser.Counter,
	},
	"free_page_low": {
		Name: "lustre_free_page_low",
		Help: "Low watermark for free pages in encryption pools.",
		Type: parser.Gauge,
	},
	"maximum_waitqueue_depth": {
		Name: "lustre_maximum_waitqueue_depth",
		Help: "Maximum depth of the encryption pool wait queue.",
		Type: parser.Gauge,
	},
	"out_of_memory_request_total": {
		Name: "lustre_out_of_memory_request_total",
		Help: "Number of out-of-memory conditions in encryption page pools.",
		Type: parser.Counter,
	},

	// LNet stats
	"allocated": {
		Name: "lustre_allocated",
		Help: "Current number of LNet messages allocated.",
		Type: parser.Gauge,
	},
	"maximum": {
		Name: "lustre_maximum",
		Help: "Maximum number of LNet messages ever allocated.",
		Type: parser.Gauge,
	},
	"errors_total": {
		Name: "lustre_errors_total",
		Help: "Total number of LNet errors.",
		Type: parser.Counter,
	},
	"send_count_total": {
		Name: "lustre_send_count_total",
		Help: "Total number of LNet messages sent.",
		Type: parser.Counter,
	},
	"receive_count_total": {
		Name: "lustre_receive_count_total",
		Help: "Total number of LNet messages received.",
		Type: parser.Counter,
	},
	"route_count_total": {
		Name: "lustre_route_count_total",
		Help: "Total number of LNet messages routed.",
		Type: parser.Counter,
	},
	"drop_count_total": {
		Name: "lustre_drop_count_total",
		Help: "Total number of LNet messages dropped.",
		Type: parser.Counter,
	},
	"send_bytes_total": {
		Name: "lustre_send_bytes_total",
		Help: "Total bytes sent via LNet.",
		Type: parser.Counter,
	},
	"receive_bytes_total": {
		Name: "lustre_receive_bytes_total",
		Help: "Total bytes received via LNet.",
		Type: parser.Counter,
	},
	"route_bytes_total": {
		Name: "lustre_route_bytes_total",
		Help: "Total bytes routed via LNet.",
		Type: parser.Counter,
	},
	"drop_bytes_total": {
		Name: "lustre_drop_bytes_total",
		Help: "Total bytes dropped by LNet.",
		Type: parser.Counter,
	},

	// LNet params
	"console_backoff_enabled": {
		Name: "lustre_console_backoff_enabled",
		Help: "Whether LNet console message backoff is enabled.",
		Type: parser.Gauge,
	},
	"console_max_delay_centiseconds": {
		Name: "lustre_console_max_delay_centiseconds",
		Help: "Maximum delay between repeated LNet console messages in centiseconds.",
		Type: parser.Gauge,
	},
	"console_min_delay_centiseconds": {
		Name: "lustre_console_min_delay_centiseconds",
		Help: "Minimum delay between repeated LNet console messages in centiseconds.",
		Type: parser.Gauge,
	},
	"console_ratelimit_enabled": {
		Name: "lustre_console_ratelimit_enabled",
		Help: "Whether LNet console message rate limiting is enabled.",
		Type: parser.Gauge,
	},
	"debug_megabytes": {
		Name: "lustre_debug_megabytes",
		Help: "Size of the LNet debug buffer in megabytes.",
		Type: parser.Gauge,
	},
	"panic_on_lbug_enabled": {
		Name: "lustre_panic_on_lbug_enabled",
		Help: "Whether kernel panic on LBUG is enabled.",
		Type: parser.Gauge,
	},
	"watchdog_ratelimit_enabled": {
		Name: "lustre_watchdog_ratelimit_enabled",
		Help: "Whether LNet watchdog rate limiting is enabled.",
		Type: parser.Gauge,
	},
	"catastrophe_enabled": {
		Name: "lustre_catastrophe_enabled",
		Help: "Whether a catastrophic LNet error has occurred.",
		Type: parser.Gauge,
	},
	"lnet_memory_used_bytes": {
		Name: "lustre_lnet_memory_used_bytes",
		Help: "Current LNet memory usage in bytes.",
		Type: parser.Gauge,
	},
	"fail_error_total": {
		Name: "lustre_fail_error_total",
		Help: "Total number of LNet fail errors.",
		Type: parser.Counter,
	},
	"fail_maximum": {
		Name: "lustre_fail_maximum",
		Help: "Maximum LNet fail value.",
		Type: parser.Gauge,
	},

	// Client core metrics (llite)
	"blocksize_bytes": {
		Name:      "lustre_blocksize_bytes",
		Help:      "Lustre filesystem block size in bytes.",
		Type:      parser.Gauge,
		LabelKeys: []string{"component", "target"},
	},
	"inodes_free": {
		Name:      "lustre_inodes_free",
		Help:      "Number of free inodes on the Lustre filesystem.",
		Type:      parser.Gauge,
		LabelKeys: []string{"component", "target"},
	},
	"inodes_maximum": {
		Name:      "lustre_inodes_maximum",
		Help:      "Maximum number of inodes on the Lustre filesystem.",
		Type:      parser.Gauge,
		LabelKeys: []string{"component", "target"},
	},
	"available_kibibytes": {
		Name:      "lustre_available_kibibytes",
		Help:      "Available capacity of the Lustre filesystem in kibibytes.",
		Type:      parser.Gauge,
		LabelKeys: []string{"component", "target"},
	},
	"free_kibibytes": {
		Name:      "lustre_free_kibibytes",
		Help:      "Free capacity of the Lustre filesystem in kibibytes.",
		Type:      parser.Gauge,
		LabelKeys: []string{"component", "target"},
	},
	"capacity_kibibytes": {
		Name:      "lustre_capacity_kibibytes",
		Help:      "Total capacity of the Lustre filesystem in kibibytes.",
		Type:      parser.Gauge,
		LabelKeys: []string{"component", "target"},
	},
	"read_samples_total": {
		Name:      "lustre_read_samples_total",
		Help:      "Total number of read operations completed.",
		Type:      parser.Counter,
		LabelKeys: []string{"component", "target"},
	},
	"read_minimum_size_bytes": {
		Name:      "lustre_read_minimum_size_bytes",
		Help:      "Minimum read size in bytes.",
		Type:      parser.Gauge,
		LabelKeys: []string{"component", "target"},
	},
	"read_maximum_size_bytes": {
		Name:      "lustre_read_maximum_size_bytes",
		Help:      "Maximum read size in bytes.",
		Type:      parser.Gauge,
		LabelKeys: []string{"component", "target"},
	},
	"read_bytes_total": {
		Name:      "lustre_read_bytes_total",
		Help:      "Total bytes read from the Lustre filesystem.",
		Type:      parser.Counter,
		LabelKeys: []string{"component", "target"},
	},
	"write_samples_total": {
		Name:      "lustre_write_samples_total",
		Help:      "Total number of write operations completed.",
		Type:      parser.Counter,
		LabelKeys: []string{"component", "target"},
	},
	"write_minimum_size_bytes": {
		Name:      "lustre_write_minimum_size_bytes",
		Help:      "Minimum write size in bytes.",
		Type:      parser.Gauge,
		LabelKeys: []string{"component", "target"},
	},
	"write_maximum_size_bytes": {
		Name:      "lustre_write_maximum_size_bytes",
		Help:      "Maximum write size in bytes.",
		Type:      parser.Gauge,
		LabelKeys: []string{"component", "target"},
	},
	"write_bytes_total": {
		Name:      "lustre_write_bytes_total",
		Help:      "Total bytes written to the Lustre filesystem.",
		Type:      parser.Counter,
		LabelKeys: []string{"component", "target"},
	},
	"stats_total": {
		Name:      "lustre_stats_total",
		Help:      "Total count of Lustre operations by type.",
		Type:      parser.Counter,
		LabelKeys: []string{"component", "target", "operation"},
	},

	// Client tunable metrics
	"checksum_pages_enabled": {
		Name:      "lustre_checksum_pages_enabled",
		Help:      "Whether checksum verification of data pages is enabled.",
		Type:      parser.Gauge,
		LabelKeys: []string{"component", "target"},
	},
	"default_ea_size_bytes": {
		Name:      "lustre_default_ea_size_bytes",
		Help:      "Default extended attribute size in bytes.",
		Type:      parser.Gauge,
		LabelKeys: []string{"component", "target"},
	},
	"lazystatfs_enabled": {
		Name:      "lustre_lazystatfs_enabled",
		Help:      "Whether lazy statfs is enabled.",
		Type:      parser.Gauge,
		LabelKeys: []string{"component", "target"},
	},
	"maximum_ea_size_bytes": {
		Name:      "lustre_maximum_ea_size_bytes",
		Help:      "Maximum extended attribute size in bytes.",
		Type:      parser.Gauge,
		LabelKeys: []string{"component", "target"},
	},
	"maximum_read_ahead_megabytes": {
		Name:      "lustre_maximum_read_ahead_megabytes",
		Help:      "Maximum read-ahead size in megabytes.",
		Type:      parser.Gauge,
		LabelKeys: []string{"component", "target"},
	},
	"maximum_read_ahead_per_file_megabytes": {
		Name:      "lustre_maximum_read_ahead_per_file_megabytes",
		Help:      "Maximum read-ahead size per file in megabytes.",
		Type:      parser.Gauge,
		LabelKeys: []string{"component", "target"},
	},
	"maximum_read_ahead_whole_megabytes": {
		Name:      "lustre_maximum_read_ahead_whole_megabytes",
		Help:      "Maximum whole-file read-ahead size in megabytes.",
		Type:      parser.Gauge,
		LabelKeys: []string{"component", "target"},
	},
	"statahead_agl_enabled": {
		Name:      "lustre_statahead_agl_enabled",
		Help:      "Whether asynchronous glimpse lock statahead is enabled.",
		Type:      parser.Gauge,
		LabelKeys: []string{"component", "target"},
	},
	"statahead_maximum": {
		Name:      "lustre_statahead_maximum",
		Help:      "Maximum number of statahead requests.",
		Type:      parser.Gauge,
		LabelKeys: []string{"component", "target"},
	},
	"xattr_cache_enabled": {
		Name:      "lustre_xattr_cache_enabled",
		Help:      "Whether extended attribute caching is enabled.",
		Type:      parser.Gauge,
		LabelKeys: []string{"component", "target"},
	},

	// RPC stats (mdc/osc)
	"pages_per_rpc_total": {
		Name:      "lustre_pages_per_rpc_total",
		Help:      "Total pages per RPC by size bucket.",
		Type:      parser.Counter,
		LabelKeys: []string{"component", "target", "operation", "size"},
	},
	"rpcs_in_flight": {
		Name:      "lustre_rpcs_in_flight",
		Help:      "RPCs in flight by bucket.",
		Type:      parser.Counter,
		LabelKeys: []string{"component", "target", "operation", "size", "type"},
	},
	"rpcs_offset": {
		Name:      "lustre_rpcs_offset",
		Help:      "RPC offset distribution by bucket.",
		Type:      parser.Counter,
		LabelKeys: []string{"component", "target", "operation", "size"},
	},
}
