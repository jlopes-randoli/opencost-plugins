package main

import (
	"context"
	"fmt"
	"time"

	"github.com/opencost/opencost-plugins/pkg/plugins/network/networkplugin"
	"github.com/opencost/opencost/core/pkg/log"
	"github.com/opencost/opencost/core/pkg/model/pb"
	prometheusApiV1 "github.com/prometheus/client_golang/api/prometheus/v1"
	"github.com/prometheus/common/model"
)

func (s *NetworkCostSource) getSumOfInterZoneDataSinceBillingPeriodStart(req *pb.CustomCostRequest, query string) (sum int64, err error) {
	// create a range between the start of the billing period and the start of the request query range
	start := getBillingPeriodStartDate(req.Start.AsTime(), s.billingPeriodStartDate)
	end := req.Start.AsTime()
	step := req.Resolution.AsDuration()

	// if billing period starts on the same day as the query, there is no previous data to get
	if start.Equal(end) {
		return 0, nil
	}

	// query the Prometheus API for workload inter-zone bytes within the given range
	results, err := s.queryPrometheusData(query, start, end, step)
	if err != nil {
		return 0, err
	}

	return getInterZoneBytesSumFromPrometheusResults(results), nil
}

func (s *NetworkCostSource) queryPrometheusData(query string, start time.Time, end time.Time, step time.Duration) (model.Value, error) {
	queryRange := prometheusApiV1.Range{
		Start: start,
		End:   end,
		Step:  step,
	}

	formattedQuery := fmt.Sprintf(query, step.String())

	ctx, cancel := context.WithTimeout(context.TODO(), s.prometheusTimeout)
	defer cancel()

	results, warnings, err := s.prometheusClient.QueryRange(ctx, formattedQuery, queryRange)
	if err != nil {
		return nil, err
	}
	if len(warnings) > 0 {
		for _, warning := range warnings {
			log.Warnf("non-critical error while querying prometheus. query: %s, warning: %s", formattedQuery, warning)
		}
	}

	return results, nil
}

func getInterZoneBytesSumFromPrometheusResults(results model.Value) int64 {
	// loop through the given results and sum all bytes that are confirmed as inter-zone data transfer
	var sum int64

	matrix := results.(model.Matrix)
	for _, stream := range matrix {
		// if any labels are missing, we cannot evaluate this data and skip it
		srcZone, ok := stream.Metric[networkplugin.PROMETHEUS_LABEL_SRC_ZONE]
		if !ok {
			continue
		}

		dstZone, ok := stream.Metric[networkplugin.PROMETHEUS_LABEL_DST_ZONE]
		if !ok {
			continue
		}

		// if the data transferred between two different zones, then we need to add these bytes to the sum
		if srcZone != dstZone {
			for _, val := range stream.Values {
				sum += int64(val.Value)
			}
		}
	}

	return sum
}
