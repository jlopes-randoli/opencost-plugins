package main

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strconv"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/pricing"
	"github.com/aws/aws-sdk-go-v2/service/pricing/types"
	"github.com/opencost/opencost-plugins/pkg/plugins/network/networkplugin"
	"github.com/opencost/opencost/core/pkg/log"
	"github.com/opencost/opencost/core/pkg/model/pb"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

type NetworkCostSource struct {
	// TODO: add values
}

func (d *NetworkCostSource) GetCustomCosts(req *pb.CustomCostRequest) []*pb.CustomCostResponse {
	provider := getProvider()

	return provider.GetNetworkCost(req)
}

// HELPER FUNCTIONS

func generateErrorResponse(msg string, err error, results []*pb.CustomCostResponse) []*pb.CustomCostResponse {
	errMsg := fmt.Sprintf("%s:%v", msg, err)

	log.Error(errMsg)
	errResponse := pb.CustomCostResponse{
		Errors: []string{errMsg},
	}

	results = append(results, &errResponse)
	return results
}

func getRegionFromNodeLabels() (string, error) {
	config, err := rest.InClusterConfig()
	if err != nil {
		return "", err
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return "", err
	}

	nodes, err := clientset.CoreV1().Nodes().List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		return "", err
	}

	if region, ok := nodes.Items[0].Labels[networkplugin.K8S_REGION_LABEL]; ok {
		return region, nil
	} else {
		return "", fmt.Errorf("label '%s' does not exist on node", networkplugin.K8S_REGION_LABEL)
	}
}

// PROVIDER IMPLEMENTATIONS
type Provider interface {
	GetNetworkCost(req *pb.CustomCostRequest) []*pb.CustomCostResponse
}

// TODO: implement way to get correct provider
func getProvider() Provider {
	return &AwsProvider{}
}

// AWS

type AwsProvider struct{}

func (p *AwsProvider) GetNetworkCost(req *pb.CustomCostRequest) []*pb.CustomCostResponse {
	results := []*pb.CustomCostResponse{}

	// load AWS config
	cfg, err := config.LoadDefaultConfig(
		context.TODO(),
		config.WithRegion(networkplugin.REGION_US_EAST_1),  // Pricing API is only available in us-east-1
		config.WithSharedConfigProfile("network-cost-dev"), // TODO: this credential needs to be added and configured automatically
	)
	if err != nil {
		return generateErrorResponse("failed to load AWS config", err, results)
	}

	// create pricing client
	client := pricing.NewFromConfig(cfg)

	// get region from node labels
	region, err := getRegionFromNodeLabels()
	if err != nil {
		return generateErrorResponse("failed to fetch region from node labels", err, results)
	}

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
	interZoneProducts, err := client.GetProducts(context.TODO(), interZoneInput)
	if err != nil {
		return generateErrorResponse("failed to get AWS products related to inter-zone network cost", err, results)
	}

	// loop through products that matched given filter
	var billedIntraRegionCost float32
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
	case networkplugin.REGION_US_EAST_1:
		return networkplugin.AWS_USAGE_TYPE_REGIONAL_US_EAST_1
	case networkplugin.REGION_US_WEST_1:
		return networkplugin.AWS_USAGE_TYPE_REGIONAL_US_WEST_1
	case networkplugin.REGION_US_WEST_2:
		return networkplugin.AWS_USAGE_TYPE_REGIONAL_US_WEST_2
	case networkplugin.REGION_US_GOV_WEST_1:
		return networkplugin.AWS_USAGE_TYPE_REGIONAL_US_GOV_WEST_1
	case networkplugin.REGION_SOUTH_AMERICA_EAST_1:
		return networkplugin.AWS_USAGE_TYPE_REGIONAL_SOUTH_AMERICA_EAST_1
	case networkplugin.REGION_EUROPE_WEST_1:
		return networkplugin.AWS_USAGE_TYPE_REGIONAL_EUROPE_WEST_1
	case networkplugin.REGION_EUROPE_CENTRAL_1:
		return networkplugin.AWS_USAGE_TYPE_REGIONAL_EUROPE_CENTRAL_1
	case networkplugin.REGION_ASIA_PACIFIC_NORTH_EAST_1:
		return networkplugin.AWS_USAGE_TYPE_REGIONAL_ASIA_PACIFIC_NORTH_EAST_1
	case networkplugin.REGION_ASIA_PACIFIC_NORTH_EAST_2:
		return networkplugin.AWS_USAGE_TYPE_REGIONAL_ASIA_PACIFIC_NORTH_EAST_2
	case networkplugin.REGION_ASIA_PACIFIC_SOUTH_EAST_1:
		return networkplugin.AWS_USAGE_TYPE_REGIONAL_ASIA_PACIFIC_SOUTH_EAST_1
	case networkplugin.REGION_ASIA_PACIFIC_SOUTH_EAST_2:
		return networkplugin.AWS_USAGE_TYPE_REGIONAL_ASIA_PACIFIC_SOUTH_EAST_2
	default:
		return networkplugin.AWS_USAGE_TYPE_REGIONAL_US_EAST_1
	}
}
