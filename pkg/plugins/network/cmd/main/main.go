package main

import (
	"context"
	"fmt"
	"time"

	"github.com/opencost/opencost-plugins/pkg/plugins/network/networkplugin"
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
