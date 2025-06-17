package main

import (
	"fmt"
	"math"
	"os"
	"path/filepath"

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

func convertBytesToGB(bytes int64) float64 {
	return float64(bytes) / math.Pow(1024, 3)
}

func convertGBToBytes(gb float64) int64 {
	return int64(math.Round(gb * math.Pow(1024, 3)))
}

func generateErrorResponse(msg string, err error, results []*pb.CustomCostResponse) []*pb.CustomCostResponse {
	errMsg := fmt.Sprintf("%s: %v", msg, err)

	log.Error(errMsg)
	errResponse := pb.CustomCostResponse{
		Errors: []string{errMsg},
	}

	results = append(results, &errResponse)
	return results
}
