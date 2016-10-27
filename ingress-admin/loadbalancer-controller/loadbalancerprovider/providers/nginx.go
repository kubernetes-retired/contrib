/*
Copyright 2016 The Kubernetes Authors.

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
package nginx

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/golang/glog"
	tprapi "k8s.io/contrib/ingress-admin/loadbalancer-controller/api"
	"k8s.io/contrib/ingress-admin/loadbalancer-controller/controller"
	"k8s.io/contrib/ingress-admin/loadbalancer-controller/loadbalancerprovider"
	"k8s.io/kubernetes/pkg/api/errors"

	"k8s.io/client-go/1.5/dynamic"
	"k8s.io/client-go/1.5/kubernetes"
	"k8s.io/client-go/1.5/pkg/api"
	"k8s.io/client-go/1.5/pkg/api/resource"
	"k8s.io/client-go/1.5/pkg/api/unversioned"
	"k8s.io/client-go/1.5/pkg/api/v1"
	"k8s.io/client-go/1.5/pkg/util/intstr"
)

var keepalibedImage, nginxIngressImage string

func init() {
	keepalibedImage = os.Getenv("INGRESS-KEEPALIVED-IMAGE")
	if keepalibedImage == "" {
		keepalibedImage = "index.caicloud.io/caicloud/ingress-keepalived-vip:v0.0.1"
	}
	nginxIngressImage = os.Getenv("INGRESS-NGINX-IMAGE")
	if nginxIngressImage == "" {
		nginxIngressImage = "index.caicloud.io/caicloud/nginx-ingress-controller:v0.0.1"
	}
}

func ProbeLoadBalancerPlugin() loadbalancerprovider.LoadBalancerPlugin {
	return &nginxLoadBalancerPlugin{}
}

const (
	nginxLoadBalancerPluginName = "ingress.alpha.k8s.io/ingress-nginx"
	ingressRoleLabelKey         = "ingress.alpha.k8s.io/role"
)

var (
	lbresource = &unversioned.APIResource{Name: "loadbalancers", Kind: "loadbalancer", Namespaced: true}
)

var _ loadbalancerprovider.LoadBalancerPlugin = &nginxLoadBalancerPlugin{}

type nginxLoadBalancerPlugin struct{}

func (plugin *nginxLoadBalancerPlugin) GetPluginName() string {
	return nginxLoadBalancerPluginName
}

func (plugin *nginxLoadBalancerPlugin) CanSupport(claim *tprapi.LoadBalancerClaim) bool {
	if claim == nil || claim.Annotations == nil {
		return false
	}
	return claim.Annotations[controller.IngressProvisioningClassKey] == nginxLoadBalancerPluginName
}

func (plugin *nginxLoadBalancerPlugin) NewProvisioner(options loadbalancerprovider.LoadBalancerOptions) loadbalancerprovider.Provisioner {
	return &nginxLoadbalancerProvisioner{
		options: options,
	}
}

type nginxLoadbalancerProvisioner struct {
	options loadbalancerprovider.LoadBalancerOptions
}

var _ loadbalancerprovider.Provisioner = &nginxLoadbalancerProvisioner{}

func (p *nginxLoadbalancerProvisioner) Provision(clientset *kubernetes.Clientset, dynamicClient *dynamic.Client) (string, error) {
	service, rc, loadbalancer := p.getService(), p.getReplicationController(), p.getLoadBalancer()

	lbUnstructed, err := loadbalancer.ToUnstructured()
	if err != nil {
		return "", err
	}

	err = func() error {
		if _, err := clientset.Core().Services("kube-system").Create(service); err != nil {
			return err
		}
		if _, err := clientset.Core().ReplicationControllers("kube-system").Create(rc); err != nil {
			return err
		}
		if _, err := dynamicClient.Resource(lbresource, "kube-system").Create(lbUnstructed); err != nil {
			return err
		}
		return nil
	}()

	if err != nil {
		if err := clientset.Core().Services("kube-system").Delete(service.Name, &api.DeleteOptions{}); err != nil && !errors.IsNotFound(err) {
			glog.Errorf("Faile do delete service due to: %v", err)
		}
		if err := clientset.Core().ReplicationControllers("kube-system").Delete(rc.Name, &api.DeleteOptions{}); err != nil && !errors.IsNotFound(err) {
			glog.Errorf("Faile do delete rc due to: %v", err)
		}
		if err := dynamicClient.Resource(lbresource, "kube-system").Delete(lbUnstructed.GetName(), &v1.DeleteOptions{}); err != nil && !errors.IsNotFound(err) {
			glog.Errorf("Faile do delete lb due to: %v", err)
		}

		return "", fmt.Errorf("Failed to provision loadbalancer due to: %v", err)
	}

	return p.options.LoadBalancerName, nil
}

func (p *nginxLoadbalancerProvisioner) getLoadBalancer() *tprapi.LoadBalancer {
	return &tprapi.LoadBalancer{
		TypeMeta: unversioned.TypeMeta{
			Kind: "Loadbalancer",
		},
		ObjectMeta: v1.ObjectMeta{
			Name: p.options.LoadBalancerName,
			Annotations: map[string]string{
				controller.IngressParameterVIPKey: p.options.LoadBalancerVIP,
				"kubernetes.io/createdby":         "loadbalancer-nginx-dynamic-provisioner",
			},
		},
		Spec: tprapi.LoadBalancerSpec{
			NginxLoadBalancer: &tprapi.NginxLoadBalancer{
				Service: v1.ObjectReference{
					Kind:      "Service",
					Namespace: "kube-system",
					Name:      p.options.LoadBalancerName,
				},
			},
		},
	}
}

func (p *nginxLoadbalancerProvisioner) getService() *v1.Service {
	return &v1.Service{
		ObjectMeta: v1.ObjectMeta{
			Name: p.options.LoadBalancerName,
			Labels: map[string]string{
				"kubernetes.io/cluster-service": "true",
			},
			Annotations: map[string]string{
				controller.IngressParameterVIPKey: p.options.LoadBalancerVIP,
				"kubernetes.io/createdby":         "loadbalancer-nginx-dynamic-provisioner",
			},
		},
		Spec: v1.ServiceSpec{
			Selector: map[string]string{
				"k8s-app":                       p.options.LoadBalancerName,
				"kubernetes.io/cluster-service": "true",
			},
			Ports: []v1.ServicePort{
				{
					Port:       80,
					TargetPort: intstr.FromInt(80),
				},
			},
		},
	}
}

func (p *nginxLoadbalancerProvisioner) getReplicationController() *v1.ReplicationController {
	nginxlbReplicas, terminationGracePeriodSeconds, nginxlbPrivileged := int32(2), int64(60), true

	lbTolerations, _ := json.Marshal([]api.Toleration{{Key: "dedicated", Value: "loadbalancer", Effect: api.TaintEffectNoSchedule}})

	nodeAffinity := &v1.NodeAffinity{
		RequiredDuringSchedulingIgnoredDuringExecution: &v1.NodeSelector{
			NodeSelectorTerms: []v1.NodeSelectorTerm{{
				MatchExpressions: []v1.NodeSelectorRequirement{{
					Key: ingressRoleLabelKey, Operator: v1.NodeSelectorOpIn, Values: []string{"loadbalancer"},
				}},
			}},
		},
	}

	podAntiAffinity := &v1.PodAntiAffinity{
		RequiredDuringSchedulingIgnoredDuringExecution: []v1.PodAffinityTerm{
			{
				LabelSelector: &unversioned.LabelSelector{
					MatchLabels: map[string]string{
						"k8s-app":                       p.options.LoadBalancerName,
						"kubernetes.io/cluster-service": "true",
					},
				},
				TopologyKey: unversioned.LabelHostname,
			},
		},
	}
	affinityAnnotation, _ := json.Marshal(v1.Affinity{NodeAffinity: nodeAffinity, PodAntiAffinity: podAntiAffinity})

	return &v1.ReplicationController{
		ObjectMeta: v1.ObjectMeta{
			Name: p.options.LoadBalancerName,
			Labels: map[string]string{
				"k8s-app":                       p.options.LoadBalancerName,
				"kubernetes.io/cluster-service": "true",
			},
			Annotations: map[string]string{
				"kubernetes.io/createdby": "loadbalancer-nginx-dynamic-provisioner",
			},
		},
		Spec: v1.ReplicationControllerSpec{
			Replicas: &nginxlbReplicas,
			Template: &v1.PodTemplateSpec{
				ObjectMeta: v1.ObjectMeta{
					Labels: map[string]string{
						"k8s-app":                       p.options.LoadBalancerName,
						"kubernetes.io/cluster-service": "true",
					},
					Annotations: map[string]string{
						api.TolerationsAnnotationKey: string(lbTolerations),
						api.AffinityAnnotationKey:    string(affinityAnnotation),
					},
				},
				Spec: v1.PodSpec{
					HostNetwork:                   true,
					TerminationGracePeriodSeconds: &terminationGracePeriodSeconds,
					Containers: []v1.Container{
						{
							Name:            "keepalived",
							Image:           keepalibedImage,
							ImagePullPolicy: v1.PullAlways,
							Resources: v1.ResourceRequirements{
								Requests: v1.ResourceList{
									v1.ResourceCPU:    resource.MustParse("50m"),
									v1.ResourceMemory: resource.MustParse("50Mi"),
								},
								Limits: v1.ResourceList{
									v1.ResourceCPU:    resource.MustParse("50m"),
									v1.ResourceMemory: resource.MustParse("50Mi"),
								},
							},
							SecurityContext: &v1.SecurityContext{
								Privileged: &nginxlbPrivileged,
							},
							Env: []v1.EnvVar{
								{
									Name: "POD_NAME",
									ValueFrom: &v1.EnvVarSource{
										FieldRef: &v1.ObjectFieldSelector{
											FieldPath: "metadata.name",
										},
									},
								},
								{
									Name: "POD_NAMESPACE",
									ValueFrom: &v1.EnvVarSource{
										FieldRef: &v1.ObjectFieldSelector{
											FieldPath: "metadata.namespace",
										},
									},
								},
								{
									Name:  "SERVICE_NAME",
									Value: p.options.LoadBalancerName,
								},
							},
						},
						{
							Name:            "nginx-ingress-lb",
							Image:           nginxIngressImage,
							ImagePullPolicy: v1.PullAlways,
							Resources:       p.options.Resources,
							ReadinessProbe: &v1.Probe{
								Handler: v1.Handler{
									HTTPGet: &v1.HTTPGetAction{
										Path:   "/ingress-controller-healthz",
										Port:   intstr.FromInt(80),
										Scheme: v1.URISchemeHTTP,
									},
								},
							},
							LivenessProbe: &v1.Probe{
								Handler: v1.Handler{
									HTTPGet: &v1.HTTPGetAction{
										Path:   "/ingress-controller-healthz",
										Port:   intstr.FromInt(80),
										Scheme: v1.URISchemeHTTP,
									},
								},
							},
							Env: []v1.EnvVar{
								{
									Name: "POD_NAME",
									ValueFrom: &v1.EnvVarSource{
										FieldRef: &v1.ObjectFieldSelector{
											FieldPath: "metadata.name",
										},
									},
								},
								{
									Name: "POD_NAMESPACE",
									ValueFrom: &v1.EnvVarSource{
										FieldRef: &v1.ObjectFieldSelector{
											FieldPath: "metadata.namespace",
										},
									},
								},
							},
							Ports: []v1.ContainerPort{
								{
									ContainerPort: 80,
									HostPort:      80,
								},
								{
									ContainerPort: 443,
									HostPort:      443,
								},
							},
							Args: []string{
								"/nginx-ingress-controller",
								"--default-backend-service=default/default-http-backend",
							},
						},
					},
				},
			},
		},
	}
}
