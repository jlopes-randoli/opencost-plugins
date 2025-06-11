package main

import (
	"github.com/opencost/opencost-plugins/pkg/plugins/network/networkplugin"
	"github.com/prometheus/common/model"
)

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
