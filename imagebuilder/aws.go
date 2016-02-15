/*
Copyright 2016 The Kubernetes Authors All rights reserved.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package main

import (
	"fmt"
	"time"

	"golang.org/x/crypto/ssh"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/golang/glog"
)

// AWSInstance manages an AWS instance, used for building an image
type AWSInstance struct {
	instanceID string
	cloud      *AWSCloud
	instance   *ec2.Instance
}

// Shutdown terminates the running instance
func (i *AWSInstance) Shutdown() error {
	glog.Infof("Terminating instance %q", i.instanceID)
	return i.cloud.TerminateInstance(i.instanceID)
}

// DialSSH establishes an SSH client connection to the instance
func (i *AWSInstance) DialSSH(config *ssh.ClientConfig) (*ssh.Client, error) {
	publicIP, err := i.WaitPublicIP()
	if err != nil {
		return nil, err
	}

	for {
		// TODO: Timeout, check error code
		sshClient, err := ssh.Dial("tcp", publicIP+":22", config)
		if err != nil {
			glog.Warningf("error connecting to SSH on server %q: %v", publicIP, err)
			time.Sleep(5 * time.Second)
			continue
			//	return nil, fmt.Errorf("error connecting to SSH on server %q", publicIP)
		}

		return sshClient, nil
	}
}

// WaitPublicIP waits for the instance to get a public IP, returning it
func (i *AWSInstance) WaitPublicIP() (string, error) {
	// TODO: Timeout
	for {
		instance, err := i.cloud.describeInstance(i.instanceID)
		if err != nil {
			return "", err
		}
		publicIP := aws.StringValue(instance.PublicIpAddress)
		if publicIP != "" {
			glog.Infof("Instance public IP is %q", publicIP)
			return publicIP, nil
		}
		glog.V(2).Infof("Sleeping before requerying instance for public IP: %q", i.instanceID)
		time.Sleep(5 * time.Second)
	}
}

// AWSCloud is a helper type for talking to an AWS acccount
type AWSCloud struct {
	Region          string
	ec2             *ec2.EC2
	ImageId         string
	InstanceType    string
	SSHKeyName      string
	SubnetID        string
	SecurityGroupID string
}

func (a *AWSCloud) describeInstance(instanceID string) (*ec2.Instance, error) {
	request := &ec2.DescribeInstancesInput{}
	request.InstanceIds = []*string{&instanceID}

	glog.V(2).Infof("AWS DescribeInstances InstanceId=%q", instanceID)
	response, err := a.ec2.DescribeInstances(request)
	if err != nil {
		return nil, fmt.Errorf("error making AWS DescribeInstances call: %v", err)
	}

	for _, reservation := range response.Reservations {
		for _, instance := range reservation.Instances {
			if aws.StringValue(instance.InstanceId) != instanceID {
				panic("Unexpected InstanceId found")
			}

			return instance, err
		}
	}
	return nil, nil
}

// TerminateInstance terminates the specified instance
func (a *AWSCloud) TerminateInstance(instanceID string) error {
	request := &ec2.TerminateInstancesInput{}
	request.InstanceIds = []*string{&instanceID}

	glog.V(2).Infof("AWS TerminateInstances instanceID=%q", instanceID)
	_, err := a.ec2.TerminateInstances(request)
	return err
}

// GetInstance returns the AWS instance matching our tags, or nil if not found
func (a *AWSCloud) GetInstance() (*AWSInstance, error) {
	request := &ec2.DescribeInstancesInput{}
	request.Filters = []*ec2.Filter{
		{
			Name:   aws.String("tag:" + tagKey),
			Values: aws.StringSlice([]string{tagValue}),
		},
		{
			Name:   aws.String("instance-state-name"),
			Values: aws.StringSlice([]string{"pending", "running"}),
		},
	}

	glog.V(2).Infof("AWS DescribeInstances Filter:Tag:%q=%q", tagKey, tagValue)
	response, err := a.ec2.DescribeInstances(request)
	if err != nil {
		return nil, fmt.Errorf("error making AWS DescribeInstances call: %v", err)
	}

	for _, reservation := range response.Reservations {
		for _, instance := range reservation.Instances {
			instanceID := aws.StringValue(instance.InstanceId)
			if instanceID == "" {
				panic("Found instance with empty instance ID")
			}

			glog.Infof("Found existing instance: %q", instanceID)
			return &AWSInstance{
				cloud:      a,
				instance:   instance,
				instanceID: instanceID,
			}, nil
		}
	}

	return nil, nil
}

// TagResource adds AWS tags to the specified resource
func (a *AWSCloud) TagResource(resourceId string, tags ...*ec2.Tag) error {
	request := &ec2.CreateTagsInput{}
	request.Resources = aws.StringSlice([]string{resourceId})
	request.Tags = tags

	glog.V(2).Infof("AWS CreateTags Resource=%q", resourceId)
	_, err := a.ec2.CreateTags(request)
	if err != nil {
		return fmt.Errorf("error making AWS CreateTag call: %v", err)
	}

	return err
}

// CreateInstance creates an instance for building an image instance
func (a *AWSCloud) CreateInstance() (*AWSInstance, error) {
	request := &ec2.RunInstancesInput{}
	request.ImageId = aws.String(a.ImageId)
	request.KeyName = aws.String(a.SSHKeyName)
	request.InstanceType = aws.String(a.InstanceType)
	request.NetworkInterfaces = []*ec2.InstanceNetworkInterfaceSpecification{
		{
			DeviceIndex:              aws.Int64(0),
			AssociatePublicIpAddress: aws.Bool(true),
			SubnetId:                 aws.String(a.SubnetID),
			Groups:                   aws.StringSlice([]string{a.SecurityGroupID}),
		},
	}
	request.MaxCount = aws.Int64(1)
	request.MinCount = aws.Int64(1)

	glog.V(2).Infof("AWS RunInstances InstanceType=%q ImageId=%q KeyName=%q", a.InstanceType, a.ImageId, a.SSHKeyName)
	response, err := a.ec2.RunInstances(request)
	if err != nil {
		return nil, fmt.Errorf("error making AWS RunInstances call: %v", err)
	}

	for _, instance := range response.Instances {
		instanceID := aws.StringValue(instance.InstanceId)
		if instanceID == "" {
			return nil, fmt.Errorf("AWS RunInstances call returned empty InstanceId")
		}
		err := a.TagResource(instanceID, &ec2.Tag{
			Key: aws.String(tagKey), Value: aws.String(tagValue),
		})
		if err != nil {
			glog.Warningf("Tagging instance %q failed; will terminate to prevent leaking", instanceID)
			e2 := a.TerminateInstance(instanceID)
			if e2 != nil {
				glog.Warningf("error terminating instance %q, will leak instance", instanceID)
			}
			return nil, err
		}

		return &AWSInstance{
			cloud:      a,
			instance:   instance,
			instanceID: instanceID,
		}, nil
	}
	return nil, fmt.Errorf("instance was not returned by AWS RunInstances")
}

// FindImage finds a registered image, matching by the name tag
func (a *AWSCloud) FindImage(imageName string) (*AWSImage, error) {
	image, err := findImage(a.ec2, imageName)
	if err != nil {
		return nil, err
	}

	if image == nil {
		return nil, nil
	}

	imageID := aws.StringValue(image.ImageId)
	if imageID == "" {
		return nil, fmt.Errorf("found image with empty ImageId: %q", imageName)
	}

	return &AWSImage{
		cloud:   a,
		image:   image,
		imageID: imageID,
	}, nil
}

func findImage(client *ec2.EC2, imageName string) (*ec2.Image, error) {
	request := &ec2.DescribeImagesInput{}
	request.Filters = []*ec2.Filter{
		{
			Name:   aws.String("name"),
			Values: aws.StringSlice([]string{imageName}),
		},
	}
	request.Owners = aws.StringSlice([]string{"self"})

	glog.V(2).Infof("AWS DescribeImages Filter:Name=%q, Owner=self", imageName)
	response, err := client.DescribeImages(request)
	if err != nil {
		return nil, fmt.Errorf("error making AWS DescribeImages call: %v", err)
	}

	if len(response.Images) == 0 {
		return nil, nil
	}

	if len(response.Images) != 1 {
		// Image names are unique per user...
		return nil, fmt.Errorf("found multiple matching images for name: %q", imageName)
	}

	image := response.Images[0]
	return image, nil
}

// AWSImage represents an AMI on AWS
type AWSImage struct {
	cloud   *AWSCloud
	image   *ec2.Image
	imageID string
}

// ID returns the AWS identifier for the image
func (i *AWSImage) ID() string {
	return i.imageID
}

// String returns a string representation of the image
func (i *AWSImage) String() string {
	return "AWSImage[id=" + i.imageID + "]"
}

// EnsurePublic makes the image accessible outside the current account
func (i *AWSImage) EnsurePublic() error {
	return ensurePublic(i.cloud.ec2, i.imageID)
}

func waitImageAvailable(client *ec2.EC2, imageID string) error {
	for {
		// TODO: Timeout
		request := &ec2.DescribeImagesInput{}
		request.ImageIds = aws.StringSlice([]string{imageID})

		glog.V(2).Infof("AWS DescribeImages ImageId=%q", imageID)
		response, err := client.DescribeImages(request)
		if err != nil {
			return fmt.Errorf("error making AWS DescribeImages call: %v", err)
		}

		if len(response.Images) == 0 {
			return fmt.Errorf("image not found %q", imageID)
		}

		if len(response.Images) != 1 {
			return fmt.Errorf("multiple imags found with ID %q", imageID)
		}

		image := response.Images[0]

		state := aws.StringValue(image.State)
		glog.V(2).Infof("image state %q", state)
		if state == "available" {
			return nil
		}
		glog.Infof("Image not yet available (%s); waiting", imageID)
		time.Sleep(10 * time.Second)
	}
}

func ensurePublic(client *ec2.EC2, imageID string) error {
	err := waitImageAvailable(client, imageID)
	if err != nil {
		return err
	}

	// This is idempotent, so just always do it
	request := &ec2.ModifyImageAttributeInput{}
	request.ImageId = aws.String(imageID)
	request.LaunchPermission = &ec2.LaunchPermissionModifications{
		Add: []*ec2.LaunchPermission{
			{Group: aws.String("all")},
		},
	}

	glog.V(2).Infof("AWS ModifyImageAttribute Image=%q, LaunchPermission All", imageID)
	_, err = client.ModifyImageAttribute(request)
	if err != nil {
		return fmt.Errorf("error making image public %q: %v", imageID, err)
	}

	return err
}

// ReplicateImage copies the image to all accessable AWS regions
func (i *AWSImage) ReplicateImage(makePublic bool) (map[string]string, error) {
	imageIDs := make(map[string]string)

	glog.V(2).Infof("AWS DescribeRegions")
	request := &ec2.DescribeRegionsInput{}
	response, err := i.cloud.ec2.DescribeRegions(request)
	if err != nil {
		return nil, fmt.Errorf("error listing ec2 regions: %v", err)
	}

	imageIDs[i.cloud.Region] = i.imageID

	for _, region := range response.Regions {
		name := aws.StringValue(region.RegionName)
		if imageIDs[name] != "" {
			continue
		}

		imageID, err := i.copyImageToRegion(name)
		if err != nil {
			return nil, fmt.Errorf("error copying image to region %q: %v", err)
		}
		imageIDs[name] = imageID
	}

	if makePublic {
		for regionName, imageID := range imageIDs {
			targetEC2 := ec2.New(session.New(), &aws.Config{Region: &regionName})
			err := ensurePublic(targetEC2, imageID)
			if err != nil {
				return nil, fmt.Errorf("error making image public in region %q: %v", regionName, err)
			}
		}
	}

	return imageIDs, nil
}

func (i *AWSImage) copyImageToRegion(regionName string) (string, error) {
	targetEC2 := ec2.New(session.New(), &aws.Config{Region: &regionName})

	imageName := aws.StringValue(i.image.Name)
	description := aws.StringValue(i.image.Description)

	destImage, err := findImage(targetEC2, imageName)
	if err != nil {
		return "", err
	}

	var imageID string

	// We've already copied the image
	if destImage != nil {
		imageID = aws.StringValue(destImage.ImageId)
	} else {
		token := imageName + "-" + regionName

		request := &ec2.CopyImageInput{
			ClientToken:   aws.String(token),
			Description:   aws.String(description),
			Name:          aws.String(imageName),
			SourceImageId: aws.String(i.imageID),
			SourceRegion:  aws.String(i.cloud.Region),
		}
		glog.V(2).Infof("AWS CopyImage Image=%q, Region=%q", i.imageID, regionName)
		response, err := targetEC2.CopyImage(request)
		if err != nil {
			return "", fmt.Errorf("error copying image to region %q: %v", regionName, err)
		}

		imageID = aws.StringValue(response.ImageId)
	}

	return imageID, nil
}
