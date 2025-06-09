package main

import (
	"testing"
	"time"

	"github.com/opencost/opencost/core/pkg/model/pb"
	"github.com/opencost/opencost/core/pkg/util/timeutil"
	"google.golang.org/protobuf/types/known/durationpb"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func TestGetCustomCosts(t *testing.T) {
	windowStart := time.Date(2024, 10, 9, 0, 0, 0, 0, time.UTC)
	windowEnd := time.Date(2024, 10, 10, 0, 0, 0, 0, time.UTC)

	req := &pb.CustomCostRequest{
		Start:      timestamppb.New(windowStart),
		End:        timestamppb.New(windowEnd),
		Resolution: durationpb.New(timeutil.Day),
	}

	networkCostSource := NetworkCostSource{}
	networkCostSource.GetCustomCosts(req)
}
