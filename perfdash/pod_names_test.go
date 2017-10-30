package main

import "testing"

func TestRemoveDisambiguationInfixes(t *testing.T) {
	testCases := []struct{ Input, Output string }{
		{"v1", "v1"},
		{"v1-blah", "v1-blah"},
		{"etcd-empty-dir-cleanup-e2e-scalability-master/etcd-empty-dir-cleanup", "etcd-empty-dir-cleanup-e2e-scalability-master/etcd-empty-dir-cleanup"},
		{"etcd-server-e2e-scalability-master/etcd-container", "etcd-server-e2e-scalability-master/etcd-container"},
		{"etcd-server-events-e2e-scalability-master/etcd-container", "etcd-server-events-e2e-scalability-master/etcd-container"},
		{"event-exporter-v0.1.7-9d4dbb69c-sff9w/event-exporter", "event-exporter/event-exporter"},
		{"fluentd-gcp-v2.0.9-2dxjh/fluentd-gcp", "fluentd-gcp/fluentd-gcp"},
		{"heapster-v1.5.0-beta.0-64d4f4bdd8-ljnfd/eventer", "heapster/eventer"},
		{"kube-addon-manager-e2e-scalability-master/kube-addon-manager", "kube-addon-manager-e2e-scalability-master/kube-addon-manager"},
		{"kube-apiserver-e2e-scalability-master/kube-apiserver", "kube-apiserver-e2e-scalability-master/kube-apiserver"},
		{"kube-controller-manager-e2e-scalability-master/kube-controller-manager", "kube-controller-manager-e2e-scalability-master/kube-controller-manager"},
		{"kube-dns-74dbf45884-7gkmp/dnsmasq", "kube-dns/dnsmasq"},
		{"kubernetes-dashboard-765c6f47bd-sbrxj/kubernetes-dashboard", "kubernetes-dashboard/kubernetes-dashboard"},
		{"l7-default-backend-6d477bf555-jfmgt/default-http-backend", "l7-default-backend/default-http-backend"},
		{"l7-lb-controller-v0.9.7-e2e-scalability-master/l7-lb-controller", "l7-lb-controller/l7-lb-controller"},
		{"kube-proxy-e2e-scalability-minion-group-2mh1/kube-proxy", "kube-proxy-e2e-scalability-minion-group/kube-proxy"},
	}
	for _, testCase := range testCases {
		v := RemoveDisambiguationInfixes(testCase.Input)
		if v != testCase.Output {
			t.Error("For", testCase.Input, "expected", testCase.Output, "but got", v)
		}
	}
}
