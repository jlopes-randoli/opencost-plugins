package networkplugin

const K8S_REGION_LABEL = "topology.kubernetes.io/region"
const DAY_OF_MONTH_31st = 31

// PROMETHEUS
const PROMETHEUS_LABEL_SRC_ZONE = "SrcK8S_Zone"
const PROMETHEUS_LABEL_DST_ZONE = "DstK8S_Zone"
const PROMETHEUS_LABEL_SRC_OWNER_NAME = "SrcK8S_OwnerName"
const PROMETHEUS_LABEL_SRC_OWNER_TYPE = "SrcK8S_OwnerType"

const QUERY_WORKLOAD_INGRESS_BYTES_TOTAL = "increase(netobserv_workload_ingress_bytes_total[%s])"
const QUERY_WORKLOAD_EGRESS_BYTES_TOTAL = "increase(netobserv_workload_egress_bytes_total[%s])"

// AWS
const AWS_CONFIG_REGION = "us-east-1"
const AWS_SERVICE_CODE = "AmazonEC2" // Service type code for pricing API
const AWS_FORMAT_VERSION = "aws_v1"  // Format version of pricing API response

const AWS_REGION_US_EAST_1 = "us-east-1"
const AWS_REGION_US_EAST_2 = "us-east-2"
const AWS_REGION_US_WEST_1 = "us-west-1"
const AWS_REGION_US_WEST_2 = "us-west-2"
const AWS_REGION_US_GOV_EAST_1 = "us-gov-east-1"
const AWS_REGION_US_GOV_WEST_1 = "us-gov-west-1"

const AWS_REGION_CANADA_CENTRAL_1 = "ca-central-1"
const AWS_REGION_CANADA_WEST_1 = "ca-west-1"

const AWS_REGION_MEXICO_CENTRAL_1 = "mx-central-1"

const AWS_REGION_SOUTH_AMERICA_EAST_1 = "sa-east-1"

const AWS_REGION_EUROPE_CENTRAL_1 = "eu-central-1"
const AWS_REGION_EUROPE_CENTRAL_2 = "eu-central-2"
const AWS_REGION_EUROPE_WEST_1 = "eu-west-1"
const AWS_REGION_EUROPE_WEST_2 = "eu-west-2"
const AWS_REGION_EUROPE_WEST_3 = "eu-west-3"
const AWS_REGION_EUROPE_SOUTH_1 = "eu-south-1"
const AWS_REGION_EUROPE_SOUTH_2 = "eu-south-2"
const AWS_REGION_EUROPE_NORTH_1 = "eu-north-1"

const AWS_REGION_ISRAEL_CENTRAL_1 = "il-central-1"
const AWS_REGION_MIDDLE_EAST_CENTRAL_1 = "me-central-1"
const AWS_REGION_MIDDLE_EAST_SOUTH_1 = "me-south-1"

const AWS_REGION_AFRICA_SOUTH_1 = "af-south-1"

const AWS_REGION_ASIA_PACIFIC_SOUTH_1 = "ap-south-1"
const AWS_REGION_ASIA_PACIFIC_SOUTH_2 = "ap-south-2"
const AWS_REGION_ASIA_PACIFIC_SOUTH_EAST_1 = "ap-southeast-1"
const AWS_REGION_ASIA_PACIFIC_SOUTH_EAST_2 = "ap-southeast-2"
const AWS_REGION_ASIA_PACIFIC_SOUTH_EAST_3 = "ap-southeast-3"
const AWS_REGION_ASIA_PACIFIC_SOUTH_EAST_4 = "ap-southeast-4"
const AWS_REGION_ASIA_PACIFIC_SOUTH_EAST_5 = "ap-southeast-5"
const AWS_REGION_ASIA_PACIFIC_SOUTH_EAST_7 = "ap-southeast-7"

const AWS_REGION_ASIA_PACIFIC_EAST_1 = "ap-east-1"
const AWS_REGION_ASIA_PACIFIC_NORTH_EAST_1 = "ap-northeast-1"
const AWS_REGION_ASIA_PACIFIC_NORTH_EAST_2 = "ap-northeast-2"
const AWS_REGION_ASIA_PACIFIC_NORTH_EAST_3 = "ap-northeast-3"

const AWS_USAGE_TYPE_REGIONAL_US_EAST_1 = "DataTransfer-Regional-Bytes"
const AWS_USAGE_TYPE_REGIONAL_US_WEST_1 = "USW1-DataTransfer-Regional-Bytes"
const AWS_USAGE_TYPE_REGIONAL_US_WEST_2 = "USW2-DataTransfer-Regional-Bytes"
const AWS_USAGE_TYPE_REGIONAL_US_GOV_WEST_1 = "UGW1-DataTransfer-Regional-Bytes"
const AWS_USAGE_TYPE_REGIONAL_SOUTH_AMERICA_EAST_1 = "SAE1-DataTransfer-Regional-Bytes"
const AWS_USAGE_TYPE_REGIONAL_EUROPE_WEST_1 = "EU-DataTransfer-Regional-Bytes"
const AWS_USAGE_TYPE_REGIONAL_EUROPE_CENTRAL_1 = "EUC1-DataTransfer-Regional-Bytes"
const AWS_USAGE_TYPE_REGIONAL_ASIA_PACIFIC_NORTH_EAST_1 = "APN1-DataTransfer-Regional-Bytes"
const AWS_USAGE_TYPE_REGIONAL_ASIA_PACIFIC_NORTH_EAST_2 = "APN2-DataTransfer-Regional-Bytes"
const AWS_USAGE_TYPE_REGIONAL_ASIA_PACIFIC_SOUTH_EAST_1 = "APS1-DataTransfer-Regional-Bytes"
const AWS_USAGE_TYPE_REGIONAL_ASIA_PACIFIC_SOUTH_EAST_2 = "APS2-DataTransfer-Regional-Bytes"
