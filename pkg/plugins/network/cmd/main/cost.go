package main

import (
	"time"

	"github.com/opencost/opencost/core/pkg/model/pb"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func getCustomCostResponseWithMetadata(start time.Time, end time.Time) pb.CustomCostResponse {
	return pb.CustomCostResponse{
		Metadata:   map[string]string{"api_client_version": "v1"},
		CostSource: "network",
		Domain:     "randoli",
		Version:    "v1",
		Currency:   "USD",
		Start:      timestamppb.New(start),
		End:        timestamppb.New(end),
		Errors:     []string{},
		Costs:      []*pb.CustomCost{},
	}
}

func getBillingPeriodStartDate(queryStartDate time.Time, billingPeriodStartDate int) time.Time {
	date := queryStartDate
	dayDifference := billingPeriodStartDate - queryStartDate.Day()

	// if the billing start date is smaller than the current one, the billing period starts in the current month
	if dayDifference < 0 {
		date = date.AddDate(0, 0, dayDifference)
	}

	// if the billing start date is larger than the current one, the billing period starts in the previous month
	if dayDifference > 0 {
		date = date.AddDate(0, -1, dayDifference)
	}

	return date
}
