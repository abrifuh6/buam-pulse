# Terraform state backend.
#
# This directory is the one piece of infrastructure that cannot manage itself:
# the S3 bucket holding state has to exist before any configuration can use it
# as a backend. So this uses local state, is applied once, and then left alone.
#
# State is kept remotely rather than on a laptop because it is the record of
# what exists. A local state file means one lost machine equals infrastructure
# nobody can change or destroy.

terraform {
  required_version = ">= 1.9"
  required_providers {
    aws = {
      source  = "hashicorp/aws"
      version = "~> 5.70"
    }
  }
}

provider "aws" {
  region = var.region

  default_tags {
    tags = {
      Project   = "pulse"
      ManagedBy = "terraform"
      Owner     = "buam-technologies"
    }
  }
}

variable "region" {
  type        = string
  default     = "ca-central-1"
  description = "Data stays in Canada: Buam is a Canadian company and Pulse makes PIPEDA claims in its data-handling features."
}

variable "state_bucket" {
  type        = string
  default     = "buam-pulse-tfstate"
  description = "Bucket names are globally unique across all of AWS, so this may need a suffix if taken."
}

resource "aws_s3_bucket" "state" {
  bucket = var.state_bucket

  # State files are the map of everything that exists. Deleting this bucket by
  # accident would orphan every resource Terraform manages, leaving them
  # running and billing with no way to change them short of clicking through
  # the console.
  lifecycle {
    prevent_destroy = true
  }
}

# Versioning is the recovery path for a corrupted or truncated state file —
# the single most valuable setting on this bucket.
resource "aws_s3_bucket_versioning" "state" {
  bucket = aws_s3_bucket.state.id
  versioning_configuration {
    status = "Enabled"
  }
}

# State contains resource identifiers, and sometimes secrets that were passed
# as variables. Encrypting at rest costs nothing and closes that exposure.
resource "aws_s3_bucket_server_side_encryption_configuration" "state" {
  bucket = aws_s3_bucket.state.id
  rule {
    apply_server_side_encryption_by_default {
      sse_algorithm = "AES256"
    }
  }
}

resource "aws_s3_bucket_public_access_block" "state" {
  bucket                  = aws_s3_bucket.state.id
  block_public_acls       = true
  block_public_policy     = true
  ignore_public_acls      = true
  restrict_public_buckets = true
}

# Old versions accumulate on every apply. Ninety days is long enough to recover
# from a mistake and short enough that the bucket does not grow forever.
resource "aws_s3_bucket_lifecycle_configuration" "state" {
  bucket = aws_s3_bucket.state.id

  rule {
    id     = "expire-old-versions"
    status = "Enabled"
    filter {}

    noncurrent_version_expiration {
      noncurrent_days = 90
    }

    abort_incomplete_multipart_upload {
      days_after_initiation = 7
    }
  }
}

output "state_bucket" {
  value = aws_s3_bucket.state.id
}

output "region" {
  value = var.region
}
