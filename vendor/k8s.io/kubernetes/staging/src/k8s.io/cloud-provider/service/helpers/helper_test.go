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

package helpers

import (
	"strings"
	"testing"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	utilnet "k8s.io/utils/net"
)

/*
This file is duplicated from "k8s.io/kubernetes/pkg/api/v1/service/util_test.go"
in order for in-tree cloud providers to not depend on internal packages.
*/

func TestGetLoadBalancerSourceRanges(t *testing.T) {
	checkError := func(v string) {
		annotations := make(map[string]string)
		annotations[v1.AnnotationLoadBalancerSourceRangesKey] = v
		svc := v1.Service{}
		svc.Annotations = annotations
		_, err := GetLoadBalancerSourceRanges(&svc)
		if err == nil {
			t.Errorf("Expected error parsing: %q", v)
		}
		svc = v1.Service{}
		svc.Spec.LoadBalancerSourceRanges = strings.Split(v, ",")
		_, err = GetLoadBalancerSourceRanges(&svc)
		if err == nil {
			t.Errorf("Expected error parsing: %q", v)
		}
	}
	checkError("10.0.0.1/33")
	checkError("foo.bar")
	checkError("10.0.0.1/32,*")
	checkError("10.0.0.1/32,")
	checkError("10.0.0.1/32, ")
	checkError("10.0.0.1")

	checkOK := func(v string) utilnet.IPNetSet {
		annotations := make(map[string]string)
		annotations[v1.AnnotationLoadBalancerSourceRangesKey] = v
		svc := v1.Service{}
		svc.Annotations = annotations
		_, err := GetLoadBalancerSourceRanges(&svc)
		if err != nil {
			t.Errorf("Unexpected error parsing: %q", v)
		}
		svc = v1.Service{}
		svc.Spec.LoadBalancerSourceRanges = strings.Split(v, ",")
		cidrs, err := GetLoadBalancerSourceRanges(&svc)
		if err != nil {
			t.Errorf("Unexpected error parsing: %q", v)
		}
		return cidrs
	}
	cidrs := checkOK("192.168.0.1/32")
	if len(cidrs) != 1 {
		t.Errorf("Expected exactly one CIDR: %v", cidrs.StringSlice())
	}
	cidrs = checkOK("192.168.0.1/32,192.168.0.1/32")
	if len(cidrs) != 1 {
		t.Errorf("Expected exactly one CIDR (after de-dup): %v", cidrs.StringSlice())
	}
	cidrs = checkOK("192.168.0.1/32,192.168.0.2/32")
	if len(cidrs) != 2 {
		t.Errorf("Expected two CIDRs: %v", cidrs.StringSlice())
	}
	cidrs = checkOK("  192.168.0.1/32 , 192.168.0.2/32   ")
	if len(cidrs) != 2 {
		t.Errorf("Expected two CIDRs: %v", cidrs.StringSlice())
	}
	// check LoadBalancerSourceRanges not specified
	svc := v1.Service{}
	cidrs, err := GetLoadBalancerSourceRanges(&svc)
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	if len(cidrs) != 1 {
		t.Errorf("Expected exactly one CIDR: %v", cidrs.StringSlice())
	}
	if !IsAllowAll(cidrs) {
		t.Errorf("Expected default to be allow-all: %v", cidrs.StringSlice())
	}
	// check SourceRanges annotation is empty
	annotations := make(map[string]string)
	annotations[v1.AnnotationLoadBalancerSourceRangesKey] = ""
	svc = v1.Service{}
	svc.Annotations = annotations
	cidrs, err = GetLoadBalancerSourceRanges(&svc)
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	if len(cidrs) != 1 {
		t.Errorf("Expected exactly one CIDR: %v", cidrs.StringSlice())
	}
	if !IsAllowAll(cidrs) {
		t.Errorf("Expected default to be allow-all: %v", cidrs.StringSlice())
	}
}

func TestAllowAll(t *testing.T) {
	checkAllowAll := func(allowAll bool, cidrs ...string) {
		ipnets, err := utilnet.ParseIPNets(cidrs...)
		if err != nil {
			t.Errorf("Unexpected error parsing cidrs: %v", cidrs)
		}
		if allowAll != IsAllowAll(ipnets) {
			t.Errorf("IsAllowAll did not return expected value for %v", cidrs)
		}
	}
	checkAllowAll(false, "10.0.0.1/32")
	checkAllowAll(false, "10.0.0.1/32", "10.0.0.2/32")
	checkAllowAll(false, "10.0.0.1/32", "10.0.0.1/32")

	checkAllowAll(true, "0.0.0.0/0")
	checkAllowAll(true, "192.168.0.0/0")
	checkAllowAll(true, "192.168.0.1/32", "0.0.0.0/0")
}

func TestRequestsOnlyLocalTraffic(t *testing.T) {
	checkRequestsOnlyLocalTraffic := func(requestsOnlyLocalTraffic bool, service *v1.Service) {
		res := RequestsOnlyLocalTraffic(service)
		if res != requestsOnlyLocalTraffic {
			t.Errorf("Expected requests OnlyLocal traffic = %v, got %v",
				requestsOnlyLocalTraffic, res)
		}
	}

	checkRequestsOnlyLocalTraffic(false, &v1.Service{})
	checkRequestsOnlyLocalTraffic(false, &v1.Service{
		Spec: v1.ServiceSpec{
			Type: v1.ServiceTypeClusterIP,
		},
	})
	checkRequestsOnlyLocalTraffic(false, &v1.Service{
		Spec: v1.ServiceSpec{
			Type: v1.ServiceTypeNodePort,
		},
	})
	checkRequestsOnlyLocalTraffic(false, &v1.Service{
		Spec: v1.ServiceSpec{
			Type:                  v1.ServiceTypeNodePort,
			ExternalTrafficPolicy: v1.ServiceExternalTrafficPolicyTypeCluster,
		},
	})
	checkRequestsOnlyLocalTraffic(true, &v1.Service{
		Spec: v1.ServiceSpec{
			Type:                  v1.ServiceTypeNodePort,
			ExternalTrafficPolicy: v1.ServiceExternalTrafficPolicyTypeLocal,
		},
	})
	checkRequestsOnlyLocalTraffic(false, &v1.Service{
		Spec: v1.ServiceSpec{
			Type:                  v1.ServiceTypeLoadBalancer,
			ExternalTrafficPolicy: v1.ServiceExternalTrafficPolicyTypeCluster,
		},
	})
	checkRequestsOnlyLocalTraffic(true, &v1.Service{
		Spec: v1.ServiceSpec{
			Type:                  v1.ServiceTypeLoadBalancer,
			ExternalTrafficPolicy: v1.ServiceExternalTrafficPolicyTypeLocal,
		},
	})
}

func TestNeedsHealthCheck(t *testing.T) {
	checkNeedsHealthCheck := func(needsHealthCheck bool, service *v1.Service) {
		res := NeedsHealthCheck(service)
		if res != needsHealthCheck {
			t.Errorf("Expected needs health check = %v, got %v",
				needsHealthCheck, res)
		}
	}

	checkNeedsHealthCheck(false, &v1.Service{
		Spec: v1.ServiceSpec{
			Type: v1.ServiceTypeClusterIP,
		},
	})
	checkNeedsHealthCheck(false, &v1.Service{
		Spec: v1.ServiceSpec{
			Type:                  v1.ServiceTypeNodePort,
			ExternalTrafficPolicy: v1.ServiceExternalTrafficPolicyTypeCluster,
		},
	})
	checkNeedsHealthCheck(false, &v1.Service{
		Spec: v1.ServiceSpec{
			Type:                  v1.ServiceTypeNodePort,
			ExternalTrafficPolicy: v1.ServiceExternalTrafficPolicyTypeLocal,
		},
	})
	checkNeedsHealthCheck(false, &v1.Service{
		Spec: v1.ServiceSpec{
			Type:                  v1.ServiceTypeLoadBalancer,
			ExternalTrafficPolicy: v1.ServiceExternalTrafficPolicyTypeCluster,
		},
	})
	checkNeedsHealthCheck(true, &v1.Service{
		Spec: v1.ServiceSpec{
			Type:                  v1.ServiceTypeLoadBalancer,
			ExternalTrafficPolicy: v1.ServiceExternalTrafficPolicyTypeLocal,
		},
	})
}

func TestHasLBFinalizer(t *testing.T) {
	testCases := []struct {
		desc         string
		svc          *v1.Service
		hasFinalizer bool
	}{
		{
			desc:         "service without finalizer",
			svc:          &v1.Service{},
			hasFinalizer: false,
		},
		{
			desc: "service with unrelated finalizer",
			svc: &v1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Finalizers: []string{"unrelated"},
				},
			},
			hasFinalizer: false,
		},
		{
			desc: "service with one finalizer",
			svc: &v1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Finalizers: []string{LoadBalancerCleanupFinalizer},
				},
			},
			hasFinalizer: true,
		},
		{
			desc: "service with multiple finalizers",
			svc: &v1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Finalizers: []string{LoadBalancerCleanupFinalizer, "unrelated"},
				},
			},
			hasFinalizer: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			if hasFinalizer := HasLBFinalizer(tc.svc); hasFinalizer != tc.hasFinalizer {
				t.Errorf("HasLBFinalizer() = %t, want %t", hasFinalizer, tc.hasFinalizer)
			}
		})
	}
}