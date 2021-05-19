// Copyright (c) 2021 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package infrastructure

import (
	"bytes"
	api "github.com/gardener/gardener-extension-provider-openstack/pkg/apis/openstack"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"text/template"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Template", func() {
	DescribeTable("dnsServer", func(values interface{}, out string) {
		testTpl := `[{{ dnsServers . }}]`
		parsedTpl, err := template.New("test").Funcs(
			map[string]interface{}{
				"dnsServers": dnsServers,
			}).
			Parse(testTpl)
		Expect(err).NotTo(HaveOccurred())

		var buffer bytes.Buffer
		err = parsedTpl.Execute(&buffer, values)

		Expect(err).NotTo(HaveOccurred())
		Expect(buffer.String()).To(Equal(out))
	},
		Entry("should print correctly", []string{"1"}, `["1"]`),
		Entry("should print correctly for 0 elements", []string{}, `[]`),
		Entry("should print correctly for multiple inputs", []string{"1", "2"}, `["1", "2"]`),
	)

	Describe("Actual Template", func() {
		var (
			infra  *extensionsv1alpha1.Infrastructure
			config *api.InfrastructureConfig

			keystoneURL = "foo-bar.com"
			values      map[string]interface{}
		)
		BeforeEach(func() {
			infra = &extensionsv1alpha1.Infrastructure{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "foo",
					Name:      "bar",
				},

				Spec: extensionsv1alpha1.InfrastructureSpec{
					Region: "de_1_1",
					SecretRef: corev1.SecretReference{
						Namespace: "foo",
						Name:      "openstack-credentials",
					},
					DefaultSpec: extensionsv1alpha1.DefaultSpec{
						ProviderConfig: &runtime.RawExtension{
							Object: config,
						},
					},
				},
			}
			config = &api.InfrastructureConfig{
				Networks: api.Networks{
					Workers: "10.1.0.0/16",
				},
				FloatingPoolName: "floating-pool-name",
			}
			values = map[string]interface{}{
				"dnsServers":  []string{"1.1.1.1", "2.2.2.2"},
				"clusterName": "mycoolcluster",
				"openstack": map[string]interface{}{
					"authURL":           keystoneURL,
					"region":            infra.Spec.Region,
					"floatingPoolName":  config.FloatingPoolName,
					"maxApiCallRetries": MaxApiCallRetries,
				},
				"create": map[string]interface{}{
					"router":  true,
					"network": true,
				},
				"router": map[string]interface{}{
					"id": DefaultRouterID,
				},
				"networks": map[string]interface{}{
					"workers":           config.Networks.Workers,
					"workersIPv6":       "",
					"dualHomed":         false,
					"subnetPoolID":      "",
					"externalNetworkID": "",
					"allocationPool":    map[string]interface{}{},
				},
				"outputKeys": map[string]interface{}{
					"routerID":          TerraformOutputKeyRouterID,
					"networkID":         TerraformOutputKeyNetworkID,
					"networkName":       TerraformOutputKeyNetworkName,
					"keyName":           TerraformOutputKeySSHKeyName,
					"securityGroupID":   TerraformOutputKeySecurityGroupID,
					"securityGroupName": TerraformOutputKeySecurityGroupName,
					"floatingNetworkID": TerraformOutputKeyFloatingNetworkID,
					"subnetID":          TerraformOutputKeySubnetID,
					"subnetIDv6":        TerraformOutputKeySubnetIDv6,
				},
			}
		})

		It("check network v4 only", func() {
			var mainTF bytes.Buffer
			err := mainTemplate.Execute(&mainTF, values)
			Expect(err).ShouldNot(HaveOccurred())
		})

		It("check network dual-stack", func() {
			values["networks"] = map[string]interface{}{
				"workers":           config.Networks.Workers,
				"workersIPv6":       "cafe::/64",
				"dualHomed":         false,
				"subnetPoolID":      "subnetpoolforv6",
				"externalNetworkID": "external-network-for-v6",
				"allocationPool": map[string]interface{}{
					"start": "cafe::5",
					"end":   "cafe::ffff",
				},
			}

			var mainTF bytes.Buffer
			err := mainTemplate.Execute(&mainTF, values)
			Expect(err).ShouldNot(HaveOccurred())

			terraformString := variablesTF + "\n" + mainTF.String()

			// Allocation pool
			Expect(terraformString).To(ContainSubstring("start = \"cafe::5\""))
			Expect(terraformString).To(ContainSubstring("end = \"cafe::ffff\""))

			// Subnet Pool
			Expect(terraformString).To(ContainSubstring("subnetpool_id = \"subnetpoolforv6\""))

			// Name consistency check.
			// !! IF THIS FAILS TERRAFORM WILL DO UNWANTED THINGS !!
			Expect(terraformString).To(ContainSubstring("resource \"openstack_networking_network_v2\" \"cluster\" {\n  name           = \"mycoolcluster\""))
			// v6
			Expect(terraformString).To(ContainSubstring("resource \"openstack_networking_subnet_v2\" \"cluster-v6\" {\n  name            = \"mycoolcluster-v6\"\n  cidr            = \"cafe::/64\""))
			Expect(terraformString).To(ContainSubstring("resource \"openstack_networking_router_v2\" \"router-v6\" {\n  name                = \"mycoolcluster-v6\""))
			// v4
			Expect(terraformString).To(ContainSubstring("resource \"openstack_networking_subnet_v2\" \"cluster-v4\" {\n  name            = \"mycoolcluster\"\n  cidr            = \"10.1.0.0/16\""))
			Expect(terraformString).To(ContainSubstring("resource \"openstack_networking_router_v2\" \"router\" {\n  name                = \"mycoolcluster\""))

		})
	})
})
