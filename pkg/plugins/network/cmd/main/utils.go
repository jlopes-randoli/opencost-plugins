package main

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/opencost/opencost-plugins/pkg/plugins/network/networkplugin"
	"github.com/opencost/opencost/core/pkg/log"
	"github.com/opencost/opencost/core/pkg/model/pb"
	prometheusApi "github.com/prometheus/client_golang/api"
	prometheusApiV1 "github.com/prometheus/client_golang/api/prometheus/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

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
		// if plugin is being run out of cluster, use out-of-cluster configuration instead
		k8sConfigPath := filepath.Join(os.Getenv("HOME"), ".kube", "config")

		k8sConfig, err = clientcmd.BuildConfigFromFlags("", k8sConfigPath)
		if err != nil {
			return nil, nil, err
		}
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
		date = date.AddDate(0, 0, dayDifference)
	}

	// if the billing start date is larger than the current one, the billing period starts in the previous month
	if dayDifference > 0 {
		date = date.AddDate(0, -1, dayDifference)
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
