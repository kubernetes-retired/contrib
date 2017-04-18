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
	"bytes"
	"flag"
	"fmt"
	"math/rand"
	"net"
	"os"
	"path"
	"time"

	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/agent"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/golang/glog"
)

const tagKey = "k8s.io/role"
const tagValue = "k8s.io/contrib/imagebuilder"

var flagRegion = flag.String("region", "", "Cloud region to use")
var flagImage = flag.String("image", "", "Image to use as builder")
var flagSSHKey = flag.String("sshkey", "", "Name of SSH key to use")
var flagInstanceType = flag.String("instancetype", "m3.medium", "Instance type to launch")
var flagSubnet = flag.String("subnet", "", "Subnet in which to launch")
var flagSecurityGroup = flag.String("securitygroup", "", "Security group to use for launch")
var flagTemplatePath = flag.String("template", "", "Path to image template")

var flagUp = flag.Bool("up", true, "Set to create instance (if not found)")
var flagBuild = flag.Bool("build", true, "Set to build image")
var flagPublish = flag.Bool("publish", true, "Set to publish image")
var flagReplicate = flag.Bool("replicate", true, "Set to copy the image to all regions")
var flagDown = flag.Bool("down", true, "Set to shut down instance (if found)")

func main() {
	rand.Seed(time.Now().UTC().UnixNano())

	flag.Set("alsologtostderr", "true")
	flag.Parse()

	if *flagBuild && *flagTemplatePath == "" {
		glog.Fatalf("--template must be provided")
	}

	if *flagRegion == "" {
		*flagRegion = "us-east-1"
	}
	if *flagImage == "" {
		*flagImage = "ami-116d857a"
	}
	if *flagInstanceType == "" {
		glog.Exitf("--instance-type must be set")
	}

	var template *BootstrapVzTemplate
	var err error
	var imageName string
	if *flagTemplatePath != "" {
		template, err = NewBootstrapVzTemplate(*flagTemplatePath)
		if err != nil {
			glog.Fatalf("error parsing template: %v", err)
		}

		imageName, err = template.BuildImageName()
		if err != nil {
			glog.Fatalf("error inferring image name: %v", err)
		}

		glog.Infof("Parsed template %q; will build image with name %s", *flagTemplatePath, imageName)
	}

	cloud := &AWSCloud{
		Region:          *flagRegion,
		ec2:             ec2.New(session.New(), &aws.Config{Region: flagRegion}),
		ImageId:         *flagImage,
		SSHKeyName:      *flagSSHKey,
		InstanceType:    *flagInstanceType,
		SecurityGroupID: *flagSecurityGroup,
		SubnetID:        *flagSubnet,
	}

	instance, err := cloud.GetInstance()
	if err != nil {
		glog.Fatalf("error getting instance: %v", err)
	}

	if instance == nil && *flagUp {
		instance, err = cloud.CreateInstance()
		if err != nil {
			glog.Fatalf("error creating instance: %v", err)
		}
	}

	image, err := cloud.FindImage(imageName)
	if err != nil {
		glog.Fatalf("error finding image %q: %v", imageName, err)
	}

	if image != nil {
		glog.Infof("found existing image %q", image.ID())
	}

	if *flagBuild && image == nil {
		if instance == nil {
			glog.Fatalf("Instance was not found (specify --up?)")
		}

		sshAgent, err := net.Dial("unix", os.Getenv("SSH_AUTH_SOCK"))
		if err != nil {
			glog.Fatalf("error connecting to SSH agent: %v", err)
		}

		sshConfig := &ssh.ClientConfig{
			User: "admin",
			Auth: []ssh.AuthMethod{
				ssh.PublicKeysCallback(agent.NewClient(sshAgent).Signers),
			},
		}
		sshClient, err := instance.DialSSH(sshConfig)
		if err != nil {
			glog.Fatalf("error SSHing to instance: %v", err)
		}
		defer sshClient.Close()

		sshHelper := NewSSH(sshClient)

		err = setupInstance(sshHelper)
		if err != nil {
			glog.Fatalf("error setting up instance: %v", err)
		}

		credentials := cloud.ec2.Config.Credentials
		if credentials == nil {
			glog.Fatalf("unable to determine EC2 credentials")
		}

		err = buildImage(sshHelper, template.Bytes(), credentials)
		if err != nil {
			glog.Fatalf("error building image: %v", err)
		}

		image, err = cloud.FindImage(imageName)
		if err != nil {
			glog.Fatalf("error finding image %q: %v", imageName, err)
		}

		if image == nil {
			glog.Fatalf("image not found after build: %q", imageName)
		}
	}

	if *flagPublish {
		if image == nil {
			glog.Fatalf("image not found: %q", imageName)
		}

		glog.Infof("Making image public: %v", image.ID())

		err = image.EnsurePublic()
		if err != nil {
			glog.Fatalf("error making image public %q: %v", imageName, err)
		}

		glog.Infof("Made image public: %v", image.ID())
	}

	if *flagReplicate {
		if image == nil {
			glog.Fatalf("image not found: %q", imageName)
		}

		glog.Infof("Copying image to all regions: %v", image.ID())

		imageIDs, err := image.ReplicateImage(*flagPublish)
		if err != nil {
			glog.Fatalf("error replicating image %q: %v", imageName, err)
		}

		for region, imageID := range imageIDs {
			glog.Infof("Image in region %q: %q", region, imageID)
		}
	}

	if *flagDown {
		if instance == nil {
			glog.Infof("Instance not found / already shutdown")
		} else {
			err := instance.Shutdown()
			if err != nil {
				glog.Fatalf("error terminating instance: %v", err)
			}
		}
	}
}

func setupInstance(ssh *SSH) error {
	// TODO: Add continuation-style (name?) error chaining???
	s := ssh.Block()

	s.Exec("sudo apt-get update")
	s.Exec("sudo apt-get install --yes git python debootstrap python-pip kpartx parted")
	s.Exec("sudo pip install termcolor jsonschema fysom docopt pyyaml boto")
	if s.Err() != nil {
		return s.Err()
	}

	return nil
}

func buildImage(ssh *SSH, template []byte, credentials *credentials.Credentials) error {
	tmpdir := fmt.Sprintf("/tmp/imagebuilder-%d", rand.Int63())
	err := ssh.SCPMkdir(tmpdir, 0755)
	if err != nil {
		return err
	}
	defer ssh.Exec("rm -rf " + tmpdir)

	logdir := path.Join(tmpdir, "logs")
	err = ssh.SCPMkdir(logdir, 0755)
	if err != nil {
		return err
	}

	//err = ssh.Exec("git clone https://github.com/andsens/bootstrap-vz.git " + tmpdir + "/bootstrap-vz")
	err = ssh.Exec("git clone https://github.com/justinsb/bootstrap-vz.git -b k8s " + tmpdir + "/bootstrap-vz")
	if err != nil {
		return err
	}

	err = ssh.SCPPut(tmpdir+"/template.yml", len(template), bytes.NewReader(template), 0644)
	if err != nil {
		return err
	}

	// TODO: Create dir for logs, log to that dir using --log, collect logs from that dir
	cmd := ssh.Command(fmt.Sprintf("./bootstrap-vz/bootstrap-vz --debug --log %q ./template.yml", logdir))
	cmd.Cwd = tmpdir
	creds, err := credentials.Get()
	if err != nil {
		return fmt.Errorf("error fetching EC2 credentials: %v", err)
	}
	cmd.Env["AWS_ACCESS_KEY"] = creds.AccessKeyID
	cmd.Env["AWS_SECRET_KEY"] = creds.SecretAccessKey
	cmd.Sudo = true
	err = cmd.Run()
	if err != nil {
		return err
	}

	// TODO: Capture debug output file?
	// TODO: Capture the image id - bootstrap-vz doesn't really help us out here - maybe we should tag it
	return nil
}
