package networkplugin

import "time"

type NetworkConfig struct {
	PrometheusURL          string        `json:"prometheus_url"`
	PrometheusTimeout      time.Duration `json:"prometheus_timeout"`
	BillingPeriodStartDate int           `json:"billing_period_start_date"`
}
