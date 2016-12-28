package builder

import (
	"github.com/golang/glog"
	"k8s.io/contrib/cluster-autoscaler/cloudprovider"
	"k8s.io/contrib/cluster-autoscaler/cloudprovider/aws"
	"k8s.io/contrib/cluster-autoscaler/cloudprovider/gce"
	"os"
)

// CloudProviderBuilder builds a cloud provider from all the necessary parameters including the name of a cloud provider e.g. aws, gce
// and the path to a config file
type CloudProviderBuilder struct {
	cloudProviderFlag string
	cloudConfig       string
}

// NewCloudProviderBuilder builds a new builder from static settings
func NewCloudProviderBuilder(cloudProviderFlag string, cloudConfig string) CloudProviderBuilder {
	return CloudProviderBuilder{
		cloudProviderFlag: cloudProviderFlag,
		cloudConfig:       cloudConfig,
	}
}

// Build a cloud provider from static settings contained in the builder and dynamic settings passed via args
func (b CloudProviderBuilder) Build(nodeGroupsFlag []string) cloudprovider.CloudProvider {
	var err error
	var cloudProvider cloudprovider.CloudProvider

	if b.cloudProviderFlag == "gce" {
		// GCE Manager
		var gceManager *gce.GceManager
		var gceError error
		if b.cloudConfig != "" {
			config, fileErr := os.Open(b.cloudConfig)
			if fileErr != nil {
				glog.Fatalf("Couldn't open cloud provider configuration %s: %#v", b.cloudConfig, err)
			}
			defer config.Close()
			gceManager, gceError = gce.CreateGceManager(config)
		} else {
			gceManager, gceError = gce.CreateGceManager(nil)
		}
		if gceError != nil {
			glog.Fatalf("Failed to create GCE Manager: %v", err)
		}
		cloudProvider, err = gce.BuildGceCloudProvider(gceManager, nodeGroupsFlag)
		if err != nil {
			glog.Fatalf("Failed to create GCE cloud provider: %v", err)
		}
	}

	if b.cloudProviderFlag == "aws" {
		var awsManager *aws.AwsManager
		var awsError error
		if b.cloudConfig != "" {
			config, fileErr := os.Open(b.cloudConfig)
			if fileErr != nil {
				glog.Fatalf("Couldn't open cloud provider configuration %s: %#v", b.cloudConfig, err)
			}
			defer config.Close()
			awsManager, awsError = aws.CreateAwsManager(config)
		} else {
			awsManager, awsError = aws.CreateAwsManager(nil)
		}
		if awsError != nil {
			glog.Fatalf("Failed to create AWS Manager: %v", err)
		}
		cloudProvider, err = aws.BuildAwsCloudProvider(awsManager, nodeGroupsFlag)
		if err != nil {
			glog.Fatalf("Failed to create AWS cloud provider: %v", err)
		}
	}

	return cloudProvider
}
