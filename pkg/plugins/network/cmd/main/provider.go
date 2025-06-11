package main

import (
	"github.com/opencost/opencost/core/pkg/model/pb"
)

type Provider interface {
	Init(src *NetworkCostSource) error
	GetNetworkCost(src *NetworkCostSource, req *pb.CustomCostRequest, region string) []*pb.CustomCostResponse
}

// TODO: implement way to get correct provider
func getProvider() Provider {
	return &AwsProvider{}
}
