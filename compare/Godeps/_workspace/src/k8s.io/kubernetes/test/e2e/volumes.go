/*
Copyright 2015 The Kubernetes Authors All rights reserved.

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

/*
 * This test checks that various VolumeSources are working.
 *
 * There are two ways, how to test the volumes:
 * 1) With containerized server (NFS, Ceph, Gluster, iSCSI, ...)
 * The test creates a server pod, exporting simple 'index.html' file.
 * Then it uses appropriate VolumeSource to import this file into a client pod
 * and checks that the pod can see the file. It does so by importing the file
 * into web server root and loadind the index.html from it.
 *
 * These tests work only when privileged containers are allowed, exporting
 * various filesystems (NFS, GlusterFS, ...) usually needs some mounting or
 * other privileged magic in the server pod.
 *
 * Note that the server containers are for testing purposes only and should not
 * be used in production.
 *
 * 2) With server outside of Kubernetes (Cinder, ...)
 * Appropriate server (e.g. OpenStack Cinder) must exist somewhere outside
 * the tested Kubernetes cluster. The test itself creates a new volume,
 * and checks, that Kubernetes can use it as a volume.
 */

package e2e

import (
	"fmt"
	"os/exec"
	"strings"
	"time"

	"k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/api/unversioned"
	client "k8s.io/kubernetes/pkg/client/unversioned"

	"github.com/golang/glog"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

// Configuration of one tests. The test consist of:
// - server pod - runs serverImage, exports ports[]
// - client pod - does not need any special configuration
type VolumeTestConfig struct {
	namespace string
	// Prefix of all pods. Typically the test name.
	prefix string
	// Name of container image for the server pod.
	serverImage string
	// Ports to export from the server pod. TCP only.
	serverPorts []int
	// Volumes needed to be mounted to the server container from the host
	// map <host (source) path> -> <container (dst.) path>
	volumes map[string]string
}

// Starts a container specified by config.serverImage and exports all
// config.serverPorts from it. The returned pod should be used to get the server
// IP address and create appropriate VolumeSource.
func startVolumeServer(client *client.Client, config VolumeTestConfig) *api.Pod {
	podClient := client.Pods(config.namespace)

	portCount := len(config.serverPorts)
	serverPodPorts := make([]api.ContainerPort, portCount)

	for i := 0; i < portCount; i++ {
		portName := fmt.Sprintf("%s-%d", config.prefix, i)

		serverPodPorts[i] = api.ContainerPort{
			Name:          portName,
			ContainerPort: config.serverPorts[i],
			Protocol:      api.ProtocolTCP,
		}
	}

	volumeCount := len(config.volumes)
	volumes := make([]api.Volume, volumeCount)
	mounts := make([]api.VolumeMount, volumeCount)

	i := 0
	for src, dst := range config.volumes {
		mountName := fmt.Sprintf("path%d", i)
		volumes[i].Name = mountName
		volumes[i].VolumeSource.HostPath = &api.HostPathVolumeSource{
			Path: src,
		}

		mounts[i].Name = mountName
		mounts[i].ReadOnly = false
		mounts[i].MountPath = dst

		i++
	}

	By(fmt.Sprint("creating ", config.prefix, " server pod"))
	privileged := new(bool)
	*privileged = true
	serverPod := &api.Pod{
		TypeMeta: unversioned.TypeMeta{
			Kind:       "Pod",
			APIVersion: "v1",
		},
		ObjectMeta: api.ObjectMeta{
			Name: config.prefix + "-server",
			Labels: map[string]string{
				"role": config.prefix + "-server",
			},
		},

		Spec: api.PodSpec{
			Containers: []api.Container{
				{
					Name:  config.prefix + "-server",
					Image: config.serverImage,
					SecurityContext: &api.SecurityContext{
						Privileged: privileged,
					},
					Ports:        serverPodPorts,
					VolumeMounts: mounts,
				},
			},
			Volumes: volumes,
		},
	}
	_, err := podClient.Create(serverPod)
	expectNoError(err, "Failed to create %s pod: %v", serverPod.Name, err)

	expectNoError(waitForPodRunningInNamespace(client, serverPod.Name, config.namespace))

	By("locating the server pod")
	pod, err := podClient.Get(serverPod.Name)
	expectNoError(err, "Cannot locate the server pod %v: %v", serverPod.Name, err)

	By("sleeping a bit to give the server time to start")
	time.Sleep(20 * time.Second)
	return pod
}

// Clean both server and client pods.
func volumeTestCleanup(client *client.Client, config VolumeTestConfig) {
	By(fmt.Sprint("cleaning the environment after ", config.prefix))

	defer GinkgoRecover()

	podClient := client.Pods(config.namespace)

	err := podClient.Delete(config.prefix+"-client", nil)
	if err != nil {
		// Log the error before failing test: if the test has already failed,
		// expectNoError() won't print anything to logs!
		glog.Warningf("Failed to delete client pod: %v", err)
		expectNoError(err, "Failed to delete client pod: %v", err)
	}
	err = podClient.Delete(config.prefix+"-server", nil)
	if err != nil {
		glog.Warningf("Failed to delete server pod: %v", err)
		expectNoError(err, "Failed to delete server pod: %v", err)
	}
}

// Start a client pod using given VolumeSource (exported by startVolumeServer())
// and check that the pod sees the data from the server pod.
func testVolumeClient(client *client.Client, config VolumeTestConfig, volume api.VolumeSource, expectedContent string) {
	By(fmt.Sprint("starting ", config.prefix, " client"))
	podClient := client.Pods(config.namespace)

	clientPod := &api.Pod{
		TypeMeta: unversioned.TypeMeta{
			Kind:       "Pod",
			APIVersion: "v1",
		},
		ObjectMeta: api.ObjectMeta{
			Name: config.prefix + "-client",
			Labels: map[string]string{
				"role": config.prefix + "-client",
			},
		},
		Spec: api.PodSpec{
			Containers: []api.Container{
				{
					Name:  config.prefix + "-client",
					Image: "gcr.io/google_containers/nginx:1.7.9",
					Ports: []api.ContainerPort{
						{
							Name:          "web",
							ContainerPort: 80,
							Protocol:      api.ProtocolTCP,
						},
					},
					VolumeMounts: []api.VolumeMount{
						{
							Name:      config.prefix + "-volume",
							MountPath: "/usr/share/nginx/html",
						},
					},
				},
			},
			Volumes: []api.Volume{
				{
					Name:         config.prefix + "-volume",
					VolumeSource: volume,
				},
			},
		},
	}
	if _, err := podClient.Create(clientPod); err != nil {
		Failf("Failed to create %s pod: %v", clientPod.Name, err)
	}
	expectNoError(waitForPodRunningInNamespace(client, clientPod.Name, config.namespace))

	By("reading a web page from the client")
	subResourceProxyAvailable, err := serverVersionGTE(subResourceProxyVersion, client)
	if err != nil {
		Failf("Failed to get server version: %v", err)
	}
	var body []byte
	if subResourceProxyAvailable {
		body, err = client.Get().
			Namespace(config.namespace).
			Resource("pods").
			SubResource("proxy").
			Name(clientPod.Name).
			DoRaw()
	} else {
		body, err = client.Get().
			Prefix("proxy").
			Namespace(config.namespace).
			Resource("pods").
			Name(clientPod.Name).
			DoRaw()
	}
	expectNoError(err, "Cannot read web page: %v", err)
	Logf("body: %v", string(body))

	By("checking the page content")
	Expect(body).To(ContainSubstring(expectedContent))
}

// Insert index.html with given content into given volume. It does so by
// starting and auxiliary pod which writes the file there.
// The volume must be writable.
func injectHtml(client *client.Client, config VolumeTestConfig, volume api.VolumeSource, content string) {
	By(fmt.Sprint("starting ", config.prefix, " injector"))
	podClient := client.Pods(config.namespace)

	injectPod := &api.Pod{
		TypeMeta: unversioned.TypeMeta{
			Kind:       "Pod",
			APIVersion: "v1",
		},
		ObjectMeta: api.ObjectMeta{
			Name: config.prefix + "-injector",
			Labels: map[string]string{
				"role": config.prefix + "-injector",
			},
		},
		Spec: api.PodSpec{
			Containers: []api.Container{
				{
					Name:    config.prefix + "-injector",
					Image:   "gcr.io/google_containers/busybox",
					Command: []string{"/bin/sh"},
					Args:    []string{"-c", "echo '" + content + "' > /mnt/index.html && chmod o+rX /mnt /mnt/index.html"},
					VolumeMounts: []api.VolumeMount{
						{
							Name:      config.prefix + "-volume",
							MountPath: "/mnt",
						},
					},
				},
			},
			RestartPolicy: api.RestartPolicyNever,
			Volumes: []api.Volume{
				{
					Name:         config.prefix + "-volume",
					VolumeSource: volume,
				},
			},
		},
	}

	defer func() {
		podClient.Delete(config.prefix+"-injector", nil)
	}()

	injectPod, err := podClient.Create(injectPod)
	expectNoError(err, "Failed to create injector pod: %v", err)
	err = waitForPodSuccessInNamespace(client, injectPod.Name, injectPod.Spec.Containers[0].Name, injectPod.Namespace)
	Expect(err).NotTo(HaveOccurred())
}

func deleteCinderVolume(name string) error {
	// Try to delete the volume for several seconds - it takes
	// a while for the plugin to detach it.
	var output []byte
	var err error
	timeout := time.Second * 120

	Logf("Waiting up to %v for removal of cinder volume %s", timeout, name)
	for start := time.Now(); time.Since(start) < timeout; time.Sleep(5 * time.Second) {
		output, err = exec.Command("cinder", "delete", name).CombinedOutput()
		if err == nil {
			Logf("Cinder volume %s deleted", name)
			return nil
		} else {
			Logf("Failed to delete volume %s: %v", name, err)
		}
	}
	Logf("Giving up deleting volume %s: %v\n%s", name, err, string(output[:]))
	return err
}

// Marked with [Skipped] to skip the test by default (see driver.go),
// these tests needs privileged containers, which are disabled by default.
// Run the test with "go run hack/e2e.go ... --ginkgo.focus=Volume"
var _ = Describe("Volumes [Skipped]", func() {
	framework := NewFramework("volume")

	// If 'false', the test won't clear its volumes upon completion. Useful for debugging,
	// note that namespace deletion is handled by delete-namespace flag
	clean := true
	// filled in BeforeEach
	var c *client.Client
	var namespace *api.Namespace

	BeforeEach(func() {
		c = framework.Client
		namespace = framework.Namespace
	})

	////////////////////////////////////////////////////////////////////////
	// NFS
	////////////////////////////////////////////////////////////////////////

	Describe("NFS", func() {
		It("should be mountable", func() {
			config := VolumeTestConfig{
				namespace:   namespace.Name,
				prefix:      "nfs",
				serverImage: "gcr.io/google_containers/volume-nfs",
				serverPorts: []int{2049},
			}

			defer func() {
				if clean {
					volumeTestCleanup(c, config)
				}
			}()
			pod := startVolumeServer(c, config)
			serverIP := pod.Status.PodIP
			Logf("NFS server IP address: %v", serverIP)

			volume := api.VolumeSource{
				NFS: &api.NFSVolumeSource{
					Server:   serverIP,
					Path:     "/",
					ReadOnly: true,
				},
			}
			// Must match content of test/images/volumes-tester/nfs/index.html
			testVolumeClient(c, config, volume, "Hello from NFS!")
		})
	})

	////////////////////////////////////////////////////////////////////////
	// Gluster
	////////////////////////////////////////////////////////////////////////

	Describe("GlusterFS", func() {
		It("should be mountable", func() {
			config := VolumeTestConfig{
				namespace:   namespace.Name,
				prefix:      "gluster",
				serverImage: "gcr.io/google_containers/volume-gluster",
				serverPorts: []int{24007, 24008, 49152},
			}

			defer func() {
				if clean {
					volumeTestCleanup(c, config)
				}
			}()
			pod := startVolumeServer(c, config)
			serverIP := pod.Status.PodIP
			Logf("Gluster server IP address: %v", serverIP)

			// create Endpoints for the server
			endpoints := api.Endpoints{
				TypeMeta: unversioned.TypeMeta{
					Kind:       "Endpoints",
					APIVersion: "v1",
				},
				ObjectMeta: api.ObjectMeta{
					Name: config.prefix + "-server",
				},
				Subsets: []api.EndpointSubset{
					{
						Addresses: []api.EndpointAddress{
							{
								IP: serverIP,
							},
						},
						Ports: []api.EndpointPort{
							{
								Name:     "gluster",
								Port:     24007,
								Protocol: api.ProtocolTCP,
							},
						},
					},
				},
			}

			endClient := c.Endpoints(config.namespace)

			defer func() {
				if clean {
					endClient.Delete(config.prefix + "-server")
				}
			}()

			if _, err := endClient.Create(&endpoints); err != nil {
				Failf("Failed to create endpoints for Gluster server: %v", err)
			}

			volume := api.VolumeSource{
				Glusterfs: &api.GlusterfsVolumeSource{
					EndpointsName: config.prefix + "-server",
					// 'test_vol' comes from test/images/volumes-tester/gluster/run_gluster.sh
					Path:     "test_vol",
					ReadOnly: true,
				},
			}
			// Must match content of test/images/volumes-tester/gluster/index.html
			testVolumeClient(c, config, volume, "Hello from GlusterFS!")
		})
	})

	////////////////////////////////////////////////////////////////////////
	// iSCSI
	////////////////////////////////////////////////////////////////////////

	// The test needs privileged containers, which are disabled by default.
	// Also, make sure that iscsiadm utility and iscsi target kernel modules
	// are installed on all nodes!
	// Run the test with "go run hack/e2e.go ... --ginkgo.focus=iSCSI"

	Describe("iSCSI", func() {
		It("should be mountable", func() {
			config := VolumeTestConfig{
				namespace:   namespace.Name,
				prefix:      "iscsi",
				serverImage: "gcr.io/google_containers/volume-iscsi",
				serverPorts: []int{3260},
				volumes: map[string]string{
					// iSCSI container needs to insert modules from the host
					"/lib/modules": "/lib/modules",
				},
			}

			defer func() {
				if clean {
					volumeTestCleanup(c, config)
				}
			}()
			pod := startVolumeServer(c, config)
			serverIP := pod.Status.PodIP
			Logf("iSCSI server IP address: %v", serverIP)

			volume := api.VolumeSource{
				ISCSI: &api.ISCSIVolumeSource{
					TargetPortal: serverIP + ":3260",
					// from test/images/volumes-tester/iscsi/initiatorname.iscsi
					IQN:      "iqn.2003-01.org.linux-iscsi.f21.x8664:sn.4b0aae584f7c",
					Lun:      0,
					FSType:   "ext2",
					ReadOnly: true,
				},
			}
			// Must match content of test/images/volumes-tester/iscsi/block.tar.gz
			testVolumeClient(c, config, volume, "Hello from iSCSI")
		})
	})

	////////////////////////////////////////////////////////////////////////
	// Ceph RBD
	////////////////////////////////////////////////////////////////////////

	Describe("Ceph RBD", func() {
		It("should be mountable", func() {
			config := VolumeTestConfig{
				namespace:   namespace.Name,
				prefix:      "rbd",
				serverImage: "gcr.io/google_containers/volume-rbd",
				serverPorts: []int{6789},
				volumes: map[string]string{
					// iSCSI container needs to insert modules from the host
					"/lib/modules": "/lib/modules",
					"/sys":         "/sys",
				},
			}

			defer func() {
				if clean {
					volumeTestCleanup(c, config)
				}
			}()
			pod := startVolumeServer(c, config)
			serverIP := pod.Status.PodIP
			Logf("Ceph server IP address: %v", serverIP)

			// create secrets for the server
			secret := api.Secret{
				TypeMeta: unversioned.TypeMeta{
					Kind:       "Secret",
					APIVersion: "v1",
				},
				ObjectMeta: api.ObjectMeta{
					Name: config.prefix + "-secret",
				},
				Data: map[string][]byte{
					// from test/images/volumes-tester/rbd/keyring
					"key": []byte("AQDRrKNVbEevChAAEmRC+pW/KBVHxa0w/POILA=="),
				},
			}

			secClient := c.Secrets(config.namespace)

			defer func() {
				if clean {
					secClient.Delete(config.prefix + "-secret")
				}
			}()

			if _, err := secClient.Create(&secret); err != nil {
				Failf("Failed to create secrets for Ceph RBD: %v", err)
			}

			volume := api.VolumeSource{
				RBD: &api.RBDVolumeSource{
					CephMonitors: []string{serverIP},
					RBDPool:      "rbd",
					RBDImage:     "foo",
					RadosUser:    "admin",
					SecretRef: &api.LocalObjectReference{
						Name: config.prefix + "-secret",
					},
					FSType:   "ext2",
					ReadOnly: true,
				},
			}
			// Must match content of test/images/volumes-tester/gluster/index.html
			testVolumeClient(c, config, volume, "Hello from RBD")

		})
	})
	////////////////////////////////////////////////////////////////////////
	// Ceph
	////////////////////////////////////////////////////////////////////////

	Describe("CephFS", func() {
		It("should be mountable", func() {
			config := VolumeTestConfig{
				namespace:   namespace.Name,
				prefix:      "cephfs",
				serverImage: "gcr.io/google_containers/volume-ceph",
				serverPorts: []int{6789},
			}

			defer func() {
				if clean {
					volumeTestCleanup(c, config)
				}
			}()
			pod := startVolumeServer(c, config)
			serverIP := pod.Status.PodIP
			Logf("Ceph server IP address: %v", serverIP)
			By("sleeping a bit to give ceph server time to initialize")
			time.Sleep(20 * time.Second)

			// create ceph secret
			secret := &api.Secret{
				TypeMeta: unversioned.TypeMeta{
					Kind:       "Secret",
					APIVersion: "v1",
				},
				ObjectMeta: api.ObjectMeta{
					Name: config.prefix + "-secret",
				},
				// Must use the ceph keyring at contrib/for-tests/volumes-ceph/ceph/init.sh
				// and encode in base64
				Data: map[string][]byte{
					"key": []byte("AQAMgXhVwBCeDhAA9nlPaFyfUSatGD4drFWDvQ=="),
				},
			}

			defer func() {
				if clean {
					if err := c.Secrets(namespace.Name).Delete(secret.Name); err != nil {
						Failf("unable to delete secret %v: %v", secret.Name, err)
					}
				}
			}()

			var err error
			if secret, err = c.Secrets(namespace.Name).Create(secret); err != nil {
				Failf("unable to create test secret %s: %v", secret.Name, err)
			}

			volume := api.VolumeSource{
				CephFS: &api.CephFSVolumeSource{
					Monitors:  []string{serverIP + ":6789"},
					User:      "kube",
					SecretRef: &api.LocalObjectReference{Name: config.prefix + "-secret"},
					ReadOnly:  true,
				},
			}
			// Must match content of contrib/for-tests/volumes-ceph/ceph/index.html
			testVolumeClient(c, config, volume, "Hello Ceph!")
		})
	})

	////////////////////////////////////////////////////////////////////////
	// OpenStack Cinder
	////////////////////////////////////////////////////////////////////////

	// This test assumes that OpenStack client tools are installed
	// (/usr/bin/nova, /usr/bin/cinder and /usr/bin/keystone)
	// and that the usual OpenStack authentication env. variables are set
	// (OS_USERNAME, OS_PASSWORD, OS_TENANT_NAME at least).

	Describe("Cinder", func() {
		It("should be mountable", func() {
			config := VolumeTestConfig{
				namespace: namespace.Name,
				prefix:    "cinder",
			}

			// We assume that namespace.Name is a random string
			volumeName := namespace.Name
			By("creating a test Cinder volume")
			output, err := exec.Command("cinder", "create", "--display-name="+volumeName, "1").CombinedOutput()
			outputString := string(output[:])
			Logf("cinder output:\n%s", outputString)
			Expect(err).NotTo(HaveOccurred())

			defer func() {
				// Ignore any cleanup errors, there is not much we can do about
				// them. They were already logged.
				deleteCinderVolume(volumeName)
			}()

			// Parse 'id'' from stdout. Expected format:
			// |     attachments     |                  []                  |
			// |  availability_zone  |                 nova                 |
			// ...
			// |          id         | 1d6ff08f-5d1c-41a4-ad72-4ef872cae685 |
			volumeID := ""
			for _, line := range strings.Split(outputString, "\n") {
				fields := strings.Fields(line)
				if len(fields) != 5 {
					continue
				}
				if fields[1] != "id" {
					continue
				}
				volumeID = fields[3]
				break
			}
			Logf("Volume ID: %s", volumeID)
			Expect(volumeID).NotTo(Equal(""))

			defer func() {
				if clean {
					Logf("Running volumeTestCleanup")
					volumeTestCleanup(c, config)
				}
			}()
			volume := api.VolumeSource{
				Cinder: &api.CinderVolumeSource{
					VolumeID: volumeID,
					FSType:   "ext3",
					ReadOnly: false,
				},
			}

			// Insert index.html into the test volume with some random content
			// to make sure we don't see the content from previous test runs.
			content := "Hello from Cinder from namespace " + volumeName
			injectHtml(c, config, volume, content)

			testVolumeClient(c, config, volume, content)
		})
	})
})
