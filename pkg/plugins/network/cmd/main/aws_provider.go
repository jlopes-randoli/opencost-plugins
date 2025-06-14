package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strconv"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsConfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/pricing"
	"github.com/aws/aws-sdk-go-v2/service/pricing/types"
	"github.com/google/uuid"
	"github.com/opencost/opencost-plugins/pkg/plugins/network/networkplugin"
	"github.com/opencost/opencost/core/pkg/log"
	"github.com/opencost/opencost/core/pkg/model/pb"
	"github.com/opencost/opencost/core/pkg/opencost"
	"github.com/prometheus/common/model"
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

	// create pricing client from configuration
	p.client = pricing.NewFromConfig(cfg)

	return nil
}

func (p *AwsProvider) GetNetworkCost(src *NetworkCostSource, req *pb.CustomCostRequest, region string) []*pb.CustomCostResponse {
	results := []*pb.CustomCostResponse{}

	windows, err := opencost.GetWindows(req.Start.AsTime(), req.End.AsTime(), req.Resolution.AsDuration())
	if err != nil {
		return generateErrorResponse("failed to create windows from request parameters", err, results)
	}

	for _, window := range windows {
		// skip any windows after the current time
		if window.Start().After(time.Now().UTC()) {
			log.Debugf("skipping future window %v", window)
			continue
		}

		// create a basic response and generate metadata
		response := getCustomCostResponseWithMetadata(*window.Start(), *window.End())

		// calculate and append inter-zone costs to response
		interZoneCosts, err := p.getInterZoneCostsForWindow(window, src, req, region)
		if err != nil {
			log.Errorf("error calculating AWS inter-zone costs: %v", err)
			response.Errors = append(response.Errors, err.Error())
		}
		if interZoneCosts != nil {
			response.Costs = append(response.Costs, interZoneCosts...)
		}

		// calculate and append internet costs to response
		internetCosts, err := p.getInternetCostsForWindow(window, src, req)
		if err != nil {
			log.Errorf("error calculating AWS internet costs: %v", err)
			response.Errors = append(response.Errors, err.Error())
		}
		if interZoneCosts != nil {
			response.Costs = append(response.Costs, internetCosts...)
		}

		results = append(results, &response)
	}

	return results
}

func (p *AwsProvider) getInterZoneCostsForWindow(window opencost.Window, src *NetworkCostSource, req *pb.CustomCostRequest, region string) ([]*pb.CustomCost, error) {
	// get correct usage type to filter for the correct product from the AWS API
	usagetype := getAwsRegionalUsageTypeFromRegion(region)

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
		return nil, fmt.Errorf("failed to get AWS products related to inter-zone network cost: %v", err)
	}

	// loop through products that matched given filter and retrieve pricing data
	var priceDimensions []networkplugin.PriceDimension
	for _, interzonePricingJson := range interZoneProducts.PriceList {

		// marshal pricing data JSON into a structure
		var interzonePricingData networkplugin.ProductPrice
		if err := json.Unmarshal([]byte(interzonePricingJson), &interzonePricingData); err != nil {
			return nil, fmt.Errorf("failed to unmarshal inter-zone network pricing entry: %v", err)
		}

		// navigate through pricing data structure to get price dimensions
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
	}

	if len(priceDimensions) > 0 {
		// get sums of billed data since start of billing period
		ingressBytesSum, err := src.getSumOfInterZoneDataSinceBillingPeriodStart(req, networkplugin.QUERY_WORKLOAD_INGRESS_BYTES_TOTAL)
		if err != nil {
			return nil, fmt.Errorf("failed to calculate sum of inter-zone ingress data transfer since start of billing period: %v", err)
		}

		egressBytesSum, err := src.getSumOfInterZoneDataSinceBillingPeriodStart(req, networkplugin.QUERY_WORKLOAD_EGRESS_BYTES_TOTAL)
		if err != nil {
			return nil, fmt.Errorf("failed to calculate sum of inter-zone egress data transfer since start of billing period: %v", err)
		}

		// create total sum of billed data to check against the pricing tier range
		totalBilledSum := ingressBytesSum + egressBytesSum

		// query inter-zone data transfer to calculate cost for window
		ingressQueryResults, err := src.queryPrometheusData(networkplugin.QUERY_WORKLOAD_INGRESS_BYTES_TOTAL, *window.Start(), *window.End(), window.Duration())
		if err != nil {
			return nil, fmt.Errorf("failed to query inter-zone ingress data transfer for given window: %v", err)
		}

		egressQueryResults, err := src.queryPrometheusData(networkplugin.QUERY_WORKLOAD_INGRESS_BYTES_TOTAL, *window.Start(), *window.End(), window.Duration())
		if err != nil {
			return nil, fmt.Errorf("failed to query inter-zone egress data transfer for given window: %v", err)
		}

		// we are manually tracking the current pricing tier index
		priceDimensionIndex := 0

		// calculate and return ingress & egress inter-zone costs
		ingressCosts, totalBilledSum, priceDimensionIndex, err := calculateAwsInterZoneCosts("Ingress Inter Zone", ingressQueryResults.(model.Matrix), priceDimensions, totalBilledSum, priceDimensionIndex)
		if err != nil {
			return nil, fmt.Errorf("failed to calculate inter-zone ingress data cost for given window: %v", err)
		}

		egressCosts, totalBilledSum, priceDimensionIndex, err := calculateAwsInterZoneCosts("Ingress Inter Zone", egressQueryResults.(model.Matrix), priceDimensions, totalBilledSum, priceDimensionIndex)
		if err != nil {
			return nil, fmt.Errorf("failed to calculate inter-zone egress data cost for given window: %v", err)
		}

		return append(ingressCosts, egressCosts...), nil
	} else {
		return nil, errors.New("received no pricing information from AWS API for the given region")
	}
}

func (p *AwsProvider) getInternetCostsForWindow(window opencost.Window, src *NetworkCostSource, req *pb.CustomCostRequest) ([]*pb.CustomCost, error) {
	return nil, nil
}

func calculateAwsInterZoneCosts(
	resourceName string,
	resultsMatrix model.Matrix,
	priceDimensions []networkplugin.PriceDimension,
	totalBilledSum int64,
	priceDimensionIndex int,
) (costs []*pb.CustomCost, updatedTotalBilledSum int64, updatedPriceDimensionIndex int, err error) {
	// create a map to store costs by workload
	workloadCostMap := make(map[networkplugin.Workload]*pb.CustomCost)

	// loop through data matrices and calculate the cost at the correct pricing tier
	for _, stream := range resultsMatrix {
		// if any labels are missing, we cannot evaluate this data and skip it
		srcZone, ok := stream.Metric[networkplugin.PROMETHEUS_LABEL_SRC_ZONE]
		if !ok {
			continue
		}

		dstZone, ok := stream.Metric[networkplugin.PROMETHEUS_LABEL_DST_ZONE]
		if !ok {
			continue
		}

		// check if we are at the correct pricing tier before calculating cost
		currentPriceDimension := priceDimensions[priceDimensionIndex]
		isCorrectDimension := false

		for isCorrectDimension == false {
			// if pricing dimension end range is 'INF', then this is the final pricing tier
			// if it is not 'INF', we need to check the current sum to make sure we are in the correct pricing tier
			if currentPriceDimension.EndRange == "Inf" {
				isCorrectDimension = true
			} else {
				endRange, err := strconv.ParseFloat(currentPriceDimension.EndRange, 64)
				if err != nil {
					return nil, 0, 0, fmt.Errorf("failed to parse float from price dimension end range: %v", err)
				}

				// if previously billed data for this billing period exceeds the current pricing tier, move to the next one
				// otherwise, this is the correct pricing tier
				if convertBytesToGB(totalBilledSum) >= endRange {
					if isIndexValidForPriceDimensionArray(priceDimensionIndex+1, priceDimensions) {
						priceDimensionIndex++
						currentPriceDimension = priceDimensions[priceDimensionIndex]
					} else {
						// if the index can not be incremented any further, we need to exit the loop
						isCorrectDimension = true
					}
				} else {
					isCorrectDimension = true
				}
			}
		}

		// if the data transferred between two different zones, we need to bill this data
		if srcZone != dstZone {
			for i, val := range stream.Values {
				if i > 0 {
					log.Warnf("prometheus query returned data for additional step when only 1 was expected: %v", val)
					continue
				}

				// TODO: check for edge-case where value goes over to next pricing dimension

				// calculate inter-zone cost
				billedQuantity := convertBytesToGB(int64(val.Value))
				pricePerUnit, err := strconv.ParseFloat(currentPriceDimension.PricePerUnit.USD, 64)
				if err != nil {
					log.Errorf("failed to parse float from price dimension price per unit: %v", err)
					break
				}

				billedCost := pricePerUnit * billedQuantity

				// convert cost and quantity to float32 to match response format
				convertedBilledCost := float32(billedCost)
				convertedBilledQuantity := float32(billedQuantity)

				// get source owner name and type to map cost to workload
				srcOwnerName, ok := stream.Metric[networkplugin.PROMETHEUS_LABEL_SRC_OWNER_NAME]
				if !ok {
					log.Warnf("prometheus data is missing label: %s", networkplugin.PROMETHEUS_LABEL_SRC_OWNER_NAME)
				}

				srcOwnerType, ok := stream.Metric[networkplugin.PROMETHEUS_LABEL_SRC_OWNER_TYPE]
				if !ok {
					log.Warnf("prometheus data is missing label: %s", networkplugin.PROMETHEUS_LABEL_SRC_OWNER_NAME)
				}

				// if workload already exists in map, add to existing workload cost
				// otherwise, create new cost associated with workload
				workload := networkplugin.Workload{
					Name: string(srcOwnerName),
					Type: string(srcOwnerType),
				}

				workloadCost, costExists := workloadCostMap[workload]
				if !costExists {
					workloadCostMap[workload] = createAwsCustomCost(convertedBilledCost, convertedBilledQuantity, resourceName)
				} else {
					workloadCost.BilledCost += convertedBilledCost
					workloadCost.UsageQuantity += convertedBilledQuantity

					workloadCostMap[workload] = workloadCost
				}

				// add billed quantity to total billed sum
				totalBilledSum += int64(val.Value)
			}
		}
	}

	// convert workload cost map to array and return
	var workloadCostArray []*pb.CustomCost
	for _, cost := range workloadCostMap {
		workloadCostArray = append(workloadCostArray, cost)
	}

	return workloadCostArray, totalBilledSum, priceDimensionIndex, nil
}

func createAwsCustomCost(billedCost float32, billedQuantity float32, resourceName string) *pb.CustomCost {
	return &pb.CustomCost{
		BilledCost:     billedCost,
		ChargeCategory: "Usage",
		Description:    fmt.Sprintf("%s Network Data Transfer", resourceName),
		Id:             uuid.New().String(),
		ResourceName:   resourceName,
		ResourceType:   "Network",
		UsageQuantity:  billedQuantity,

		// TODO: figure out values
		// AccountName:    billingEntry.OrganizationName,
		// ProviderId:    fmt.Sprintf("%s/%s/%s", billingEntry.OrganizationID, billingEntry.ProjectID, billingEntry.Name)
		// UsageUnit:      "tokens - All snapshots, all projects",
	}
}

func getAwsRegionalUsageTypeFromRegion(region string) string {
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

func isIndexValidForPriceDimensionArray(index int, slice []networkplugin.PriceDimension) bool {
	return (index >= 0 && index < len(slice))
}
