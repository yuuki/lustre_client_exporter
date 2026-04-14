package parser

import (
	"encoding/json"
	"fmt"
)

type lpccStatusOutput map[string]lpccMountEntry

type lpccMountEntry struct {
	PCC     []lpccCacheEntry `json:"pcc"`
	FSStats lpccFSStats      `json:"fs_stats"`
}

type lpccCacheEntry struct {
	Mount      string         `json:"mount"`
	Cache      string         `json:"cache"`
	Status     string         `json:"status"`
	ROID       float64        `json:"roid"`
	Autocache  string         `json:"autocache"`
	Purge      string         `json:"purge"`
	PurgeStats lpccPurgeStats `json:"purge_stats"`
}

type lpccPurgeStats struct {
	Version       string              `json:"version"`
	FSName        string              `json:"fsname"`
	CacheUsagePct float64             `json:"cache_usage_pct"`
	Config        lpccPurgeConfig     `json:"config"`
	Stats         lpccPurgeStatsInner `json:"stats"`
	CacheStats    lpccCacheStats      `json:"cache_stats"`
}

type lpccPurgeConfig struct {
	HighUsage    float64 `json:"high_usage"`
	LowUsage     float64 `json:"low_usage"`
	Interval     float64 `json:"interval"`
	ScanThreads  float64 `json:"scan_threads"`
	CandidateNum float64 `json:"candidate_num"`
}

type lpccPurgeStatsInner struct {
	ScanTimes       float64 `json:"scan_times"`
	TotalPurgedObjs float64 `json:"total_purged_objs"`
	TotalFailedObjs float64 `json:"total_failed_objs"`
	StartUsagePct   float64 `json:"start_usage_pct"`
	ScannedObjs     float64 `json:"scanned_objs"`
	PurgedObjs      float64 `json:"purged_objs"`
}

type lpccCacheStats struct {
	CachedFiles       float64 `json:"cached_files"`
	CachedBytes       float64 `json:"cached_bytes"`
	MinCachedFileSize float64 `json:"min_cached_file_size"`
	MaxCachedFileSize float64 `json:"max_cached_file_size"`
	AverageAgeSecs    float64 `json:"average_age_secs"`
}

type lpccFSStats struct {
	OpenCount          float64 `json:"open_count"`
	PCCRealHit         float64 `json:"pcc_real_hit"`
	PCCOpenHitPct      float64 `json:"pcc_open_hit_pct"`
	PCCRealHitBytes    float64 `json:"pcc_real_hit_bytes"`
	TotalReadBytes     float64 `json:"total_read_bytes"`
	PCCReadHitBytesPct float64 `json:"pcc_read_hit_bytes_pct"`
}

func runningToBool(s string) float64 {
	if s == "running" {
		return 1.0
	}
	return 0.0
}

func ParseLpccStatus(data []byte, source string) ([]Observation, error) {
	var output lpccStatusOutput
	if err := json.Unmarshal(data, &output); err != nil {
		return nil, fmt.Errorf("lpcc status: %w", err)
	}

	const metricsPerCache = 17
	const metricsPerMount = 6
	obs := make([]Observation, 0, len(output)*(metricsPerCache+metricsPerMount))

	for mountPath, entry := range output {
		for _, pcc := range entry.PCC {
			labels := map[string]string{
				"mount": mountPath,
				"cache": pcc.Cache,
			}

			cacheObs := []struct {
				id  string
				typ MetricType
				val float64
			}{
				{"pcc_status", Gauge, runningToBool(pcc.Status)},
				{"pcc_purge_status", Gauge, runningToBool(pcc.Purge)},
				{"pcc_cache_usage_ratio", Gauge, pcc.PurgeStats.CacheUsagePct / 100},
				{"pcc_purge_high_usage_ratio", Gauge, pcc.PurgeStats.Config.HighUsage / 100},
				{"pcc_purge_low_usage_ratio", Gauge, pcc.PurgeStats.Config.LowUsage / 100},
				{"pcc_purge_interval_seconds", Gauge, pcc.PurgeStats.Config.Interval},
				{"pcc_purge_scan_threads", Gauge, pcc.PurgeStats.Config.ScanThreads},
				{"pcc_purge_scan_times_total", Counter, pcc.PurgeStats.Stats.ScanTimes},
				{"pcc_purge_total_purged_objs_total", Counter, pcc.PurgeStats.Stats.TotalPurgedObjs},
				{"pcc_purge_total_failed_objs_total", Counter, pcc.PurgeStats.Stats.TotalFailedObjs},
				{"pcc_purge_scanned_objs", Gauge, pcc.PurgeStats.Stats.ScannedObjs},
				{"pcc_purge_purged_objs", Gauge, pcc.PurgeStats.Stats.PurgedObjs},
				{"pcc_cached_files", Gauge, pcc.PurgeStats.CacheStats.CachedFiles},
				{"pcc_cached_bytes", Gauge, pcc.PurgeStats.CacheStats.CachedBytes},
				{"pcc_min_cached_file_size_bytes", Gauge, pcc.PurgeStats.CacheStats.MinCachedFileSize},
				{"pcc_max_cached_file_size_bytes", Gauge, pcc.PurgeStats.CacheStats.MaxCachedFileSize},
				{"pcc_average_age_seconds", Gauge, pcc.PurgeStats.CacheStats.AverageAgeSecs},
			}

			for _, o := range cacheObs {
				obs = append(obs, Observation{
					Collector:  "lpcc",
					Source:     source,
					MetricID:   o.id,
					MetricType: o.typ,
					Labels:     labels,
					Value:      o.val,
				})
			}
		}

		fsLabels := map[string]string{
			"mount": mountPath,
		}

		fsObs := []struct {
			id  string
			typ MetricType
			val float64
		}{
			{"pcc_fs_open_count_total", Counter, entry.FSStats.OpenCount},
			{"pcc_fs_real_hit_total", Counter, entry.FSStats.PCCRealHit},
			{"pcc_fs_open_hit_ratio", Gauge, entry.FSStats.PCCOpenHitPct / 100},
			{"pcc_fs_real_hit_bytes_total", Counter, entry.FSStats.PCCRealHitBytes},
			{"pcc_fs_total_read_bytes_total", Counter, entry.FSStats.TotalReadBytes},
			{"pcc_fs_read_hit_bytes_ratio", Gauge, entry.FSStats.PCCReadHitBytesPct / 100},
		}

		for _, o := range fsObs {
			obs = append(obs, Observation{
				Collector:  "lpcc",
				Source:     source,
				MetricID:   o.id,
				MetricType: o.typ,
				Labels:     fsLabels,
				Value:      o.val,
			})
		}
	}

	return obs, nil
}
