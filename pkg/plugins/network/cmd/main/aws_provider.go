package main

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strconv"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsConfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/pricing"
	"github.com/aws/aws-sdk-go-v2/service/pricing/types"
	"github.com/opencost/opencost-plugins/pkg/plugins/network/networkplugin"
	"github.com/opencost/opencost/core/pkg/log"
	"github.com/opencost/opencost/core/pkg/model/pb"
)

type AwsProvider struct {
	client *pricing.Client
}

func (p *AwsProvider) Init(src *NetworkCostSource) error {
	// load AWS config
	cfg, err := awsConfig.LoadDefaultConfig(
		context.TODO(),
		awsConfig.WithRegion(networkplugin.AWS_REGION_US_EAST_1), // Pricing API is only available in us-east-1
		awsConfig.WithSharedConfigProfile("network-cost-dev"),    // TODO: this credential needs to be added and configured automatically
	)
	if err != nil {
		return fmt.Errorf("failed to load AWS config: %v", err)
	}

	// create pricing client
	p.client = pricing.NewFromConfig(cfg)

	return nil
}

func (p *AwsProvider) GetNetworkCost(src *NetworkCostSource, req *pb.CustomCostRequest, region string) []*pb.CustomCostResponse {
	results := []*pb.CustomCostResponse{}

	// TODO: move client creation code to start-up?

	// get correct usage type to filter for the correct product from the AWS API
	usagetype := getAwsUsageTypeFromRegion(region)

	// create filter to get inter-zone network pricing for this cluster
	interZoneInput := &pricing.GetProductsInput{
		ServiceCode: aws.String(networkplugin.AWS_SERVICE_CODE),
		Filters: []types.Filter{
			{
				Field: aws.String("usagetype"),
				Type:  types.FilterTypeTermMatch,
				Value: aws.String(usagetype),
			},
		},
		FormatVersion: aws.String(networkplugin.AWS_FORMAT_VERSION),
		MaxResults:    aws.Int32(1),
	}

	// get list of filtered products for inter-zone network pricing
	interZoneProducts, err := p.client.GetProducts(context.TODO(), interZoneInput)
	if err != nil {
		return generateErrorResponse("failed to get AWS products related to inter-zone network cost", err, results)
	}

	// TODO: add comment
	ingressBytesSum, egressBytesSum, err := src.getSumOfInterZoneDataSinceBillingPeriodStart(req)
	if err != nil {
		return generateErrorResponse("failed to calculate sums of inter-zone data transfer since start of billing period", err, results)
	}

	// TODO: remove log
	log.Infof("ingress bytes sum: %v, egress bytes sum: %v", ingressBytesSum, egressBytesSum)

	// loop through products that matched given filter
	// var billedIntraRegionCost float32
	for _, interzonePricingJson := range interZoneProducts.PriceList {

		// marshal pricing data JSON into a structure
		var interzonePricingData networkplugin.ProductPrice
		if err := json.Unmarshal([]byte(interzonePricingJson), &interzonePricingData); err != nil {
			return generateErrorResponse("failed to unmarshal inter-zone network pricing entry", err, results)
		}

		// navigate through pricing data structure to get price dimensions
		var priceDimensions []networkplugin.PriceDimension
		for _, term := range interzonePricingData.Terms.OnDemand {
			if len(priceDimensions) != 0 {
				log.Warnf("unexpected inter-zone network pricing term found. skipping... term: %v", term)
				continue
			}

			for _, dimension := range term.PriceDimensions {
				priceDimensions = append(priceDimensions, dimension)
			}
		}

		// sort price dimensions by range
		sort.Slice(priceDimensions, func(i, j int) bool {
			beginI, _ := strconv.Atoi(priceDimensions[i].BeginRange)
			beginJ, _ := strconv.Atoi(priceDimensions[j].BeginRange)
			return beginI < beginJ
		})

		// TODO: get Prometheus data and calculate billed cost
		// for _, dimension := range priceDimensions {

		// }
	}

	// TODO: finish internet cost

	return results
}

func getAwsUsageTypeFromRegion(region string) string {
	switch region {
	case networkplugin.AWS_REGION_US_EAST_1:
		return networkplugin.AWS_USAGE_TYPE_REGIONAL_US_EAST_1
	case networkplugin.AWS_REGION_US_WEST_1:
		return networkplugin.AWS_USAGE_TYPE_REGIONAL_US_WEST_1
	case networkplugin.AWS_REGION_US_WEST_2:
		return networkplugin.AWS_USAGE_TYPE_REGIONAL_US_WEST_2
	case networkplugin.AWS_REGION_US_GOV_WEST_1:
		return networkplugin.AWS_USAGE_TYPE_REGIONAL_US_GOV_WEST_1
	case networkplugin.AWS_REGION_SOUTH_AMERICA_EAST_1:
		return networkplugin.AWS_USAGE_TYPE_REGIONAL_SOUTH_AMERICA_EAST_1
	case networkplugin.AWS_REGION_EUROPE_WEST_1:
		return networkplugin.AWS_USAGE_TYPE_REGIONAL_EUROPE_WEST_1
	case networkplugin.AWS_REGION_EUROPE_CENTRAL_1:
		return networkplugin.AWS_USAGE_TYPE_REGIONAL_EUROPE_CENTRAL_1
	case networkplugin.AWS_REGION_ASIA_PACIFIC_NORTH_EAST_1:
		return networkplugin.AWS_USAGE_TYPE_REGIONAL_ASIA_PACIFIC_NORTH_EAST_1
	case networkplugin.AWS_REGION_ASIA_PACIFIC_NORTH_EAST_2:
		return networkplugin.AWS_USAGE_TYPE_REGIONAL_ASIA_PACIFIC_NORTH_EAST_2
	case networkplugin.AWS_REGION_ASIA_PACIFIC_SOUTH_EAST_1:
		return networkplugin.AWS_USAGE_TYPE_REGIONAL_ASIA_PACIFIC_SOUTH_EAST_1
	case networkplugin.AWS_REGION_ASIA_PACIFIC_SOUTH_EAST_2:
		return networkplugin.AWS_USAGE_TYPE_REGIONAL_ASIA_PACIFIC_SOUTH_EAST_2
	default:
		return networkplugin.AWS_USAGE_TYPE_REGIONAL_US_EAST_1
	}
}
