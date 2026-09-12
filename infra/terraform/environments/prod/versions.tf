terraform {
  required_version = ">= 1.10"

  # State lives in S3 with native locking (Terraform 1.10+). The older pattern
  # used a DynamoDB table for locks; S3 conditional writes replaced it, which
  # removes a resource, a cost, and a class of "the lock table drifted" problem.
  backend "s3" {
    bucket       = "buam-pulse-tfstate"
    key          = "prod/terraform.tfstate"
    region       = "ca-central-1"
    encrypt      = true
    use_lockfile = true
  }

  required_providers {
    aws = {
      source  = "hashicorp/aws"
      version = "~> 5.70"
    }
    random = {
      source  = "hashicorp/random"
      version = "~> 3.6"
    }
  }
}

provider "aws" {
  region = var.region

  # Every resource is tagged at the provider level rather than individually.
  # Without this, the first question after a surprise bill — "what is this?" —
  # has no answer.
  default_tags {
    tags = {
      Project     = "pulse"
      Environment = var.environment
      ManagedBy   = "terraform"
      Owner       = "buam-technologies"
      Repo        = "github.com/abrifuh6/buam-pulse"
    }
  }
}
