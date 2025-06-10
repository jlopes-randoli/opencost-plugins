package main

import (
	"testing"
	"time"

	"github.com/opencost/opencost-plugins/pkg/plugins/network/networkplugin"
	"github.com/opencost/opencost/core/pkg/model/pb"
	"github.com/opencost/opencost/core/pkg/util/timeutil"
	"google.golang.org/protobuf/types/known/durationpb"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func TestGetCustomCosts(t *testing.T) {
	windowStart := time.Date(2025, 6, 9, 0, 0, 0, 0, time.UTC)
	windowEnd := time.Date(2025, 6, 10, 0, 0, 0, 0, time.UTC)

	req := &pb.CustomCostRequest{
		Start:      timestamppb.New(windowStart),
		End:        timestamppb.New(windowEnd),
		Resolution: durationpb.New(timeutil.Day),
	}

	config := networkplugin.NetworkConfig{
		PrometheusURL:          "http://localhost:9090",
		PrometheusTimeout:      30 * time.Second,
		BillingPeriodStartDate: 1,
	}

	prometheusClient, k8sClient, err := getNetworkCostClients(config)
	if err != nil {
		t.Errorf("error initializing clients: %v", err)
	}

	networkCostSource := NetworkCostSource{
		prometheusClient:       prometheusClient,
		prometheusTimeout:      config.PrometheusTimeout,
		k8sClient:              k8sClient,
		billingPeriodStartDate: config.BillingPeriodStartDate,
	}

	networkCostSource.GetCustomCosts(req)
}
