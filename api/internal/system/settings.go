package system

const (
	DefaultMonitoringIntervalSeconds = 5
	MinimumMonitoringIntervalSeconds = 5
	MaximumMonitoringIntervalSeconds = 86400
	DefaultMetricsRetentionDays      = 30
	MinimumMetricsRetentionDays      = 1
	MaximumMetricsRetentionDays      = 3650
)

// MonitoringSettings controls the optional background collection of host
// metrics. Disabling it prevents new metric rows from being stored.
type MonitoringSettings struct {
	Enabled         bool `json:"enabled"`
	IntervalSeconds int  `json:"intervalSeconds"`
	RetentionDays   int  `json:"retentionDays"`
}

func DefaultMonitoringSettings() MonitoringSettings {
	return MonitoringSettings{
		Enabled:         true,
		IntervalSeconds: DefaultMonitoringIntervalSeconds,
		RetentionDays:   DefaultMetricsRetentionDays,
	}
}

func ValidMonitoringSettings(settings MonitoringSettings) bool {
	return settings.IntervalSeconds >= MinimumMonitoringIntervalSeconds &&
		settings.IntervalSeconds <= MaximumMonitoringIntervalSeconds &&
		settings.RetentionDays >= MinimumMetricsRetentionDays &&
		settings.RetentionDays <= MaximumMetricsRetentionDays
}
