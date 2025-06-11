package main

import (
	"context"
	"fmt"
	"time"

	"github.com/opencost/opencost-plugins/pkg/plugins/network/networkplugin"
	"github.com/opencost/opencost/core/pkg/log"
	"github.com/opencost/opencost/core/pkg/model/pb"

	prometheusApiV1 "github.com/prometheus/client_golang/api/prometheus/v1"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
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

	return provider.GetNetworkCost(s, req, region)
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
