terraform {
  required_providers {
    openstack = {
      source  = "terraform-provider-openstack/openstack"
    }
  }
}

provider "openstack" {
  auth_url    = "{{ .openstack.authURL }}"
  domain_name = var.DOMAIN_NAME
  tenant_name = var.TENANT_NAME
  region      = "{{ .openstack.region }}"
  user_name   = var.USER_NAME
  password    = var.PASSWORD
  application_credential_id     = var.APPLICATION_CREDENTIAL_ID
  application_credential_name   = var.APPLICATION_CREDENTIAL_NAME
  application_credential_secret = var.APPLICATION_CREDENTIAL_SECRET
  insecure    = true
  max_retries = "{{ .openstack.maxApiCallRetries }}"
}

//=====================================================================
//= Networking: Router/Interfaces/Net/SubNet/SecGroup/SecRules
//=====================================================================

data "openstack_networking_network_v2" "fip" {
  name = "{{ .openstack.floatingPoolName }}"
}

{{ if .create.router -}}
{{ if .router.floatingPoolSubnet -}}
data "openstack_networking_subnet_ids_v2" "fip_subnets" {
  name_regex = {{ .router.floatingPoolSubnet | quote }}
  network_id = data.openstack_networking_network_v2.fip.id
}
{{- end }}

resource "openstack_networking_router_v2" "router" {
  name                = "{{ .clusterName }}"
  region              = "{{ .openstack.region }}"
  external_network_id = data.openstack_networking_network_v2.fip.id
  {{ if .router.enableSNAT -}}
  enable_snat         = true
  {{- end }}
  {{ if .router.floatingPoolSubnet -}}
  external_subnet_ids = data.openstack_networking_subnet_ids_v2.fip_subnets.ids
  {{- end }}
}

{{ if .networks.externalNetworkID }}
resource "openstack_networking_router_v2" "router-v6" {
  name                = "{{ .clusterName }}-v6"
  region              = "{{ .openstack.region }}"
  external_network_id = {{ .networks.externalNetworkID | quote }}
}
{{- end }}
{{- end }}

{{ if .create.network -}}
resource "openstack_networking_network_v2" "cluster" {
  name           = "{{ .clusterName }}"
  admin_state_up = "true"
}
{{ else -}}
data "openstack_networking_network_v2" "cluster" {
  network_id   = "{{ .networks.id }}"
}
{{- end }}

{{ if .networks.dualHomed }}
# IPv6 Network in dual homed mode
resource "openstack_networking_network_v2" "cluster-v6" {
name           = "{{ .clusterName }}"
admin_state_up = "true"
}
{{- end}}

resource "openstack_networking_subnet_v2" "cluster-v4" {
  name            = "{{ .clusterName }}"
  cidr            = "{{ .networks.workers }}"
  network_id      = {{ template "network-id" $ }}
  ip_version      = 4
  {{- if .dnsServers }}
  dns_nameservers = [{{- dnsServers .dnsServers | trimSuffix ", " }}]
  {{- else }}
  dns_nameservers = []
  {{- end }}

}

{{ if .networks.workersIPv6 }}
resource "openstack_networking_subnet_v2" "cluster-v6" {
  name            = "{{ .clusterName }}-v6"
  cidr            = "{{ .networks.workersIPv6 }}"
  network_id      = {{ template "network-id" $ }}

  ip_version      = 6
  ipv6_ra_mode      = "dhcpv6-stateful"
  ipv6_address_mode = "dhcpv6-stateful"

  dns_nameservers = []

  {{ if .networks.subnetPoolID }}
  subnetpool_id = {{ .networks.subnetPoolID | quote }}
  {{- end}}

  {{ if and .networks.allocationPool.start .networks.allocationPool.end }}
  allocation_pool {
    start = {{ .networks.allocationPool.start | quote  }}
    end = {{ .networks.allocationPool.end | quote  }}
  }
  {{- end}}
}
{{- end}}

{{- if .networks.workersIPv6 }}
resource "openstack_networking_router_interface_v2" "router_nodes_v6" {
router_id = "${openstack_networking_router_v2.router-v6.id}"
subnet_id = "${openstack_networking_subnet_v2.cluster-v6.id}"
}
{{- end }}

resource "openstack_networking_router_interface_v2" "router_nodes_v4" {
  router_id = {{ .router.id }}
  subnet_id = openstack_networking_subnet_v2.cluster-v4.id
}

resource "openstack_networking_secgroup_v2" "cluster" {
  name                 = "{{ .clusterName }}"
  description          = "Cluster Nodes"
  delete_default_rules = true
}

resource "openstack_networking_secgroup_rule_v2" "cluster_self_v4" {
  direction         = "ingress"
  ethertype         = "IPv4"
  security_group_id = openstack_networking_secgroup_v2.cluster.id
  remote_group_id   = openstack_networking_secgroup_v2.cluster.id
}

resource "openstack_networking_secgroup_rule_v2" "cluster_self_v6" {
  direction         = "ingress"
  ethertype         = "IPv6"
  security_group_id = "${openstack_networking_secgroup_v2.cluster.id}"
  remote_group_id   = "${openstack_networking_secgroup_v2.cluster.id}"
}

resource "openstack_networking_secgroup_rule_v2" "cluster_egress_v4" {
  direction         = "egress"
  ethertype         = "IPv4"
  security_group_id = openstack_networking_secgroup_v2.cluster.id
}

resource "openstack_networking_secgroup_rule_v2" "cluster_egress_v6" {
  direction         = "egress"
  ethertype         = "IPv6"
  security_group_id = "${openstack_networking_secgroup_v2.cluster.id}"
}

resource "openstack_networking_secgroup_rule_v2" "cluster_tcp_all_v4" {
  direction         = "ingress"
  ethertype         = "IPv4"
  protocol          = "tcp"
  remote_ip_prefix  = "0.0.0.0/0"
  security_group_id = openstack_networking_secgroup_v2.cluster.id
}

resource "openstack_networking_secgroup_rule_v2" "cluster_udp_all" {
  direction         = "ingress"
  ethertype         = "IPv4"
  protocol          = "udp"
  remote_ip_prefix  = "0.0.0.0/0"
  security_group_id = openstack_networking_secgroup_v2.cluster.id
}

resource "openstack_networking_secgroup_rule_v2" "cluster_tcp_all_v6" {
  direction         = "ingress"
  ethertype         = "IPv6"
  protocol          = "tcp"
  port_range_min    = 1
  port_range_max    = 65535
  remote_ip_prefix  = "::/0"
  security_group_id = "${openstack_networking_secgroup_v2.cluster.id}"
}


//=====================================================================
//= SSH Key for Nodes (Bastion and Worker)
//=====================================================================

resource "openstack_compute_keypair_v2" "ssh_key" {
  name       = "{{ .clusterName }}"
  public_key = "{{ .sshPublicKey }}"
}

// We have introduced new output variables. However, they are not applied for
// existing clusters as Terraform won't detect a diff when we run `terraform plan`.
// Workaround: Providing a null-resource for letting Terraform think that there are
// differences, enabling the Gardener to start an actual `terraform apply` job.
resource "null_resource" "outputs" {
  triggers = {
    recompute = "outputs"
  }
}

//=====================================================================
//= Output Variables
//=====================================================================

output "{{ .outputKeys.routerID }}" {
  value = {{ .router.id }}
}

output "{{ .outputKeys.networkID }}" {
  value = {{ template "network-id" $ }}
}

output "{{ .outputKeys.networkName }}" {
  value = {{ template "network-name" $ }}
}

output "{{ .outputKeys.keyName }}" {
  value = openstack_compute_keypair_v2.ssh_key.name
}

output "{{ .outputKeys.securityGroupID }}" {
  value = openstack_networking_secgroup_v2.cluster.id
}

output "{{ .outputKeys.securityGroupName }}" {
  value = openstack_networking_secgroup_v2.cluster.name
}

output "{{ .outputKeys.floatingNetworkID }}" {
  value = data.openstack_networking_network_v2.fip.id
}

output "{{ .outputKeys.subnetID }}" {
  value = openstack_networking_subnet_v2.cluster-v4.id
}


output "{{ .outputKeys.subnetIDv6 }}" {
{{- if .networks.nodeIPv6 }}
  value = openstack_networking_subnet_v2.cluster-v6.id
{{- else }} // use cluster-v4 to prevent crash will not be inserted into infrastructure object
  value = openstack_networking_subnet_v2.cluster-v4.id
{{- end }}
}


// Helpers

{{- /* Helper functions */ -}}
{{- define "network-id" -}}
{{ if .create.network -}}
openstack_networking_network_v2.cluster.id
{{ else -}}
data.openstack_networking_network_v2.cluster.id
{{ end -}}
{{- end -}}
{{- define "network-name" -}}
{{ if .create.network -}}
openstack_networking_network_v2.cluster.name
{{ else -}}
data.openstack_networking_network_v2.cluster.name
{{ end -}}
{{- end -}}