bacalhau_version       = "v0.3.11"
bacalhau_port          = "1235"
bacalhau_node_id_0     = "QmNXczFhX8oLEeuGThGowkcJDJUnX4HqoYQ2uaYhuCNSxD"
bacalhau_node_id_1     = "QmfRDVYnEcPassyJFGQw8Wt4t9QuA843uuKPVNEVNm4Smo"
ipfs_version           = "v0.12.2"
gcp_project            = "bacalhau-development"
instance_count         = 2
region                 = "europe-north1"
zone                   = "europe-north1-c"
volume_size_gb         = 10
machine_type           = "e2-standard-4"
protect_resources      = true
auto_subnets           = false
ingress_cidrs          = ["0.0.0.0/0"]
ssh_access_cidrs       = ["0.0.0.0/0"]
internal_ip_addresses  = ["192.168.0.5", "192.168.0.6"]
num_gpu_machines       = 0
