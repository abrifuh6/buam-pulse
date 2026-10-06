# DNS and certificates.
#
# Deliberately separate state from the cluster. The hosted zone and the
# certificate survive `terraform destroy` on the application infrastructure,
# because re-delegating nameservers and re-validating a certificate takes hours
# and neither costs anything meaningful to leave running.

terraform {
  required_version = ">= 1.10"

  backend "s3" {
    bucket       = "buam-pulse-tfstate"
    key          = "dns/terraform.tfstate"
    region       = "ca-central-1"
    encrypt      = true
    use_lockfile = true
  }

  required_providers {
    aws = {
      source  = "hashicorp/aws"
      version = "~> 5.70"
    }
  }
}

provider "aws" {
  region = "ca-central-1"

  default_tags {
    tags = {
      Project   = "pulse"
      ManagedBy = "terraform"
      Owner     = "buam-technologies"
    }
  }
}

variable "domain" {
  type    = string
  default = "buamtech.live"
}

resource "aws_route53_zone" "main" {
  name = var.domain

  # Losing this zone means every record has to be recreated and the registrar
  # re-pointed, with DNS propagation delay on top.
  lifecycle {
    prevent_destroy = true
  }
}

# One certificate covering the apex and every subdomain. A wildcard means
# adding a new subdomain later needs no certificate work at all.
resource "aws_acm_certificate" "main" {
  domain_name               = var.domain
  subject_alternative_names = ["*.${var.domain}"]
  validation_method         = "DNS"

  lifecycle {
    create_before_destroy = true
  }
}

# ACM proves domain ownership by asking for specific CNAME records. Because
# Route 53 holds the zone, Terraform can write them itself — with DNS at the
# registrar this would be a manual copy-paste every renewal.
resource "aws_route53_record" "validation" {
  for_each = {
    for d in aws_acm_certificate.main.domain_validation_options :
    d.domain_name => d
  }

  zone_id         = aws_route53_zone.main.zone_id
  name            = each.value.resource_record_name
  type            = each.value.resource_record_type
  records         = [each.value.resource_record_value]
  ttl             = 60
  allow_overwrite = true
}

# Blocks until AWS confirms the records, so anything depending on this
# certificate waits rather than failing.
resource "aws_acm_certificate_validation" "main" {
  certificate_arn         = aws_acm_certificate.main.arn
  validation_record_fqdns = [for r in aws_route53_record.validation : r.fqdn]
}

output "zone_id" {
  value = aws_route53_zone.main.zone_id
}

output "certificate_arn" {
  value = aws_acm_certificate_validation.main.certificate_arn
}

output "nameservers" {
  value       = aws_route53_zone.main.name_servers
  description = "Set these as the nameservers at GoDaddy."
}

# The three hostnames, all pointing at the same load balancer. The ALB routes
# to the right frontend by hostname, which is what removes the /app prefix and
# its whole class of problems.
#
# The ALB is created by the controller in the cluster, not here, so its address
# is read at apply time rather than managed. These records are added in a
# second apply once the ALB exists.
variable "alb_dns_name" {
  type        = string
  default     = ""
  description = "The ALB hostname from: kubectl get ingress pulse -n pulse. Left empty until the cluster is up."
}

# The ALB's canonical hosted zone, looked up from the load balancer rather than
# hardcoded. These per-region IDs are published by AWS but easy to get wrong.
data "aws_lb" "pulse" {
  count = local.create_records ? 1 : 0
  name  = split("-", var.alb_dns_name)[0] == "k8s" ? null : null
  tags = {
    "elbv2.k8s.aws/cluster" = "pulse-prod"
  }
}

locals {
  create_records = var.alb_dns_name != ""
  hosts          = ["", "app", "status"] # "" is the apex
}

# Alias records rather than CNAMEs: the apex cannot be a CNAME (DNS forbids it),
# and aliases are free where CNAME lookups are billed.
resource "aws_route53_record" "app" {
  for_each = local.create_records ? toset(local.hosts) : []

  zone_id = aws_route53_zone.main.zone_id
  name    = each.value == "" ? var.domain : "${each.value}.${var.domain}"
  type    = "A"

  alias {
    name                   = var.alb_dns_name
    zone_id                = data.aws_lb.pulse[0].zone_id
    evaluate_target_health = true
  }
}
