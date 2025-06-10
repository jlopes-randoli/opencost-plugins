package main

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/pricing"
	"github.com/aws/aws-sdk-go-v2/service/pricing/types"
	"github.com/opencost/opencost-plugins/pkg/plugins/network/networkplugin"
	"github.com/opencost/opencost/core/pkg/log"
	"github.com/opencost/opencost/core/pkg/model/pb"

	prometheusApi "github.com/prometheus/client_golang/api"
	prometheusApiV1 "github.com/prometheus/client_golang/api/prometheus/v1"
	"github.com/prometheus/common/model"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

type NetworkCostSource struct {
	prometheusClient       prometheusApiV1.API
	prometheusTimeout      time.Duration
	k8sClient              *kubernetes.Clientset
	billingPeriodStartDate int
}

func (s *NetworkCostSource) GetCustomCosts(req *pb.CustomCostRequest) []*pb.CustomCostResponse {
	// get provider based on cluster vendor
	provider := getProvider()

	// get region from node labels
	region, err := s.getRegionFromNodeLabels()
	if err != nil {
		results := []*pb.CustomCostResponse{}
		return generateErrorResponse("failed to fetch region from node labels", err, results)
	}

	return provider.GetNetworkCost(req, s, region)
}

func (s *NetworkCostSource) getRegionFromNodeLabels() (string, error) {
	nodes, err := s.k8sClient.CoreV1().Nodes().List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		return "", err
	}

	if region, ok := nodes.Items[0].Labels[networkplugin.K8S_REGION_LABEL]; ok {
		return region, nil
	} else {
		return "", fmt.Errorf("label '%s' does not exist on node", networkplugin.K8S_REGION_LABEL)
	}
}

func (s *NetworkCostSource) getSumOfInterZoneDataSinceBillingPeriodStart(req *pb.CustomCostRequest) (ingressSum int64, egressSum int64, err error) {
	// create a range between the start of the billing period and the start of the request query range
	start := getBillingPeriodStartDate(req.Start.AsTime(), s.billingPeriodStartDate)
	end := req.Start.AsTime()
	step := req.Resolution.AsDuration()

	// if billing period starts on the same day as the query, there is no previous data to get
	if start.Equal(end) {
		return 0, 0, nil
	}

	queryRange := prometheusApiV1.Range{
		Start: start,
		End:   end,
		Step:  step,
	}

	// query the Prometheus API for workload inter-zone ingress & egress bytes within the given range
	ingressQuery := fmt.Sprintf(networkplugin.QUERY_WORKLOAD_INGRESS_BYTES_TOTAL, step.String())
	egressQuery := fmt.Sprintf(networkplugin.QUERY_WORKLOAD_EGRESS_BYTES_TOTAL, step.String())

	ctx, cancel := context.WithTimeout(context.TODO(), s.prometheusTimeout)
	defer cancel()

	ingressResults, warnings, err := s.prometheusClient.QueryRange(ctx, ingressQuery, queryRange)
	if err != nil {
		return 0, 0, err
	}
	if len(warnings) > 0 {
		for _, warning := range warnings {
			log.Warnf("non-critical error while querying prometheus. query: %s, warning: %s", ingressQuery, warning)
		}
	}

	egressResults, warnings, err := s.prometheusClient.QueryRange(ctx, egressQuery, queryRange)
	if err != nil {
		return 0, 0, err
	}
	if len(warnings) > 0 {
		for _, warning := range warnings {
			log.Warnf("non-critical error while querying prometheus. query: %s, warning: %s", egressQuery, warning)
		}
	}

	// calculate the sums of ingress and egress bytes that are identified as inter-zone data transfer
	ingressBytesSum := getInterZoneBytesSumFromPrometheusResults(ingressResults)
	egressBytesSum := getInterZoneBytesSumFromPrometheusResults(egressResults)

	return ingressBytesSum, egressBytesSum, nil
}

// PROMETHEUS HELPERS
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

// GENERIC HELPERS
func getNetworkCostClients(config networkplugin.NetworkConfig) (prometheusApiV1.API, *kubernetes.Clientset, error) {
	// init prometheus client
	client, err := prometheusApi.NewClient(prometheusApi.Config{
		Address: config.PrometheusURL,
	})
	if err != nil {
		return nil, nil, err
	}

	prometheusApiV1Client := prometheusApiV1.NewAPI(client)

	// init k8s client
	k8sConfig, err := rest.InClusterConfig()
	if err != nil {
		return nil, nil, err
	}

	clientset, err := kubernetes.NewForConfig(k8sConfig)
	if err != nil {
		return nil, nil, err
	}

	return prometheusApiV1Client, clientset, nil
}

func getBillingPeriodStartDate(queryStartDate time.Time, billingPeriodStartDate int) time.Time {
	date := queryStartDate
	dayDifference := billingPeriodStartDate - queryStartDate.Day()

	// if the billing start date is smaller than the current one, the billing period starts in the current month
	if dayDifference < 0 {
		date.AddDate(0, 0, dayDifference)
	}

	// if the billing start date is larger than the current one, the billing period starts in the previous month
	if dayDifference > 0 {
		date.AddDate(0, -1, dayDifference)
	}

	return date
}

func generateErrorResponse(msg string, err error, results []*pb.CustomCostResponse) []*pb.CustomCostResponse {
	errMsg := fmt.Sprintf("%s:%v", msg, err)

	log.Error(errMsg)
	errResponse := pb.CustomCostResponse{
		Errors: []string{errMsg},
	}

	results = append(results, &errResponse)
	return results
}

// PROVIDER IMPLEMENTATIONS
type Provider interface {
	GetNetworkCost(req *pb.CustomCostRequest, src *NetworkCostSource, region string) []*pb.CustomCostResponse
}

// TODO: implement way to get correct provider
func getProvider() Provider {
	return &AwsProvider{}
}

// AWS

type AwsProvider struct{}

func (p *AwsProvider) GetNetworkCost(req *pb.CustomCostRequest, src *NetworkCostSource, region string) []*pb.CustomCostResponse {
	results := []*pb.CustomCostResponse{}

	// TODO: move client creation code to start-up?

	// load AWS config
	cfg, err := config.LoadDefaultConfig(
		context.TODO(),
		config.WithRegion(networkplugin.AWS_REGION_US_EAST_1), // Pricing API is only available in us-east-1
		config.WithSharedConfigProfile("network-cost-dev"),    // TODO: this credential needs to be added and configured automatically
	)
	if err != nil {
		return generateErrorResponse("failed to load AWS config", err, results)
	}

	// create pricing client
	client := pricing.NewFromConfig(cfg)

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
