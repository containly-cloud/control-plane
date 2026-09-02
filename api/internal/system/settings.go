package system

const (
	CollectionIntervalSeconds   = 1
	DefaultMetricsRetentionDays = 30
	MinimumMetricsRetentionDays = 1
	MaximumMetricsRetentionDays = 3650
)

// MonitoringSettings controls the optional background collection of host
// metrics. Disabling it prevents new metric rows from being stored.
type MonitoringSettings struct {
	Enabled       bool `json:"enabled"`
	RetentionDays int  `json:"retentionDays"`
}

func DefaultMonitoringSettings() MonitoringSettings {
	return MonitoringSettings{
		Enabled:       true,
		RetentionDays: DefaultMetricsRetentionDays,
	}
}

func ValidMonitoringSettings(settings MonitoringSettings) bool {
	return settings.RetentionDays >= MinimumMetricsRetentionDays &&
		settings.RetentionDays <= MaximumMetricsRetentionDays
}

type MetricGranularity string

const (
	MetricGranularitySecond MetricGranularity = "second"
	MetricGranularityMinute MetricGranularity = "minute"
	MetricGranularityHour   MetricGranularity = "hour"
)

func ValidMetricGranularity(granularity MetricGranularity) bool {
	return granularity == MetricGranularitySecond ||
		granularity == MetricGranularityMinute ||
		granularity == MetricGranularityHour
}
