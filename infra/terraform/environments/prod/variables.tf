variable "region" {
  type        = string
  default     = "ca-central-1"
  description = "Data stays in Canada: Buam is Canadian and Pulse makes PIPEDA claims in its data-handling features."
}

variable "environment" {
  type    = string
  default = "prod"
}

variable "name" {
  type        = string
  default     = "pulse"
  description = "Prefix for every resource name, so anything in the console is traceable to this project."
}

variable "vpc_cidr" {
  type    = string
  default = "10.40.0.0/16"
  # Deliberately not 10.0.0.0/16: that is the default everywhere, and a VPC
  # that must one day peer with another network is far easier when its range is
  # unusual. Renumbering a live VPC is not possible.
}

variable "az_count" {
  type        = number
  default     = 2
  description = "Two availability zones. Three is the textbook answer and costs a third more in NAT and node capacity for a durability gain this workload does not need."
}

variable "single_nat_gateway" {
  type        = bool
  default     = true
  description = <<-EOT
    One NAT gateway shared across AZs rather than one per AZ.

    Saves roughly $35/month per AZ avoided. The trade is real: if that AZ fails,
    private subnets in every other AZ lose outbound internet until it returns.
    For Pulse that means checks stop, which is bad but recoverable, and the
    saving is a third of the monthly bill. A production system with revenue
    riding on it should set this false.
  EOT
}

variable "cluster_version" {
  type    = string
  default = "1.31"
}

variable "node_instance_types" {
  type    = list(string)
  default = ["t3.medium"]
}

variable "node_capacity_type" {
  type        = string
  default     = "SPOT"
  description = <<-EOT
    Spot instances, roughly 70% cheaper than on-demand, in exchange for a
    two-minute termination notice.

    Pulse tolerates this well: every service is stateless, the scheduler
    restarts in seconds, and queued checks survive a worker dying because they
    sit in Redis until something consumes them. The database is managed and
    unaffected. Set to ON_DEMAND if a node disappearing mid-check is not
    acceptable.
  EOT
}

variable "db_instance_class" {
  type        = string
  default     = "db.t4g.micro"
  description = "Graviton: roughly 20% cheaper than the equivalent Intel instance for identical performance on this workload."
}

variable "cache_node_type" {
  type    = string
  default = "cache.t4g.micro"
}
