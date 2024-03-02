package config

import "time"

// Poller describes configuration for the poller to automatically update the database with new articles
type Poller struct {
	// Enabled whether to automatically update database with new articles
	Enabled bool `json:"enabled" yaml:"enabled" mapstructure:"enabled"`
	// Interval time interval in minutes for polling for feed updates
	Interval time.Duration `json:"interval" yaml:"interval" mapstructure:"interval"`
}
