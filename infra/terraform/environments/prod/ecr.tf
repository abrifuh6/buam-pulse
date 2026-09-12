# Container registry
# -----------------------------------------------------------------------------
# Images are already published to GHCR and Docker Hub by CI. ECR exists as well
# because pulls from inside the VPC stay on the AWS network — no NAT data
# charges, no dependency on a third party being up during a deploy, and IAM
# rather than a long-lived token for authentication.

locals {
  services = ["api", "scheduler", "worker", "notifier", "retention", "migrate", "web", "status", "site"]
}

resource "aws_ecr_repository" "service" {
  for_each = toset(local.services)

  name                 = "${var.name}/${each.value}"
  image_tag_mutability = "IMMUTABLE" # a tag can never be repointed, so a SHA always means one artifact

  image_scanning_configuration {
    scan_on_push = true
  }

  # Images are rebuilt from source; nothing here is irreplaceable.
  force_delete = true

  tags = { Name = "${var.name}-${each.value}" }
}

# Without this, every CI run adds images forever at $0.10/GB/month.
resource "aws_ecr_lifecycle_policy" "service" {
  for_each   = aws_ecr_repository.service
  repository = each.value.name

  policy = jsonencode({
    rules = [
      {
        rulePriority = 1
        description  = "Keep the last 20 tagged images"
        selection = {
          tagStatus     = "tagged"
          tagPrefixList = ["sha-", "v"]
          countType     = "imageCountMoreThan"
          countNumber   = 20
        }
        action = { type = "expire" }
      },
      {
        rulePriority = 2
        description  = "Expire untagged layers after a day"
        selection = {
          tagStatus   = "untagged"
          countType   = "sinceImagePushed"
          countUnit   = "days"
          countNumber = 1
        }
        action = { type = "expire" }
      },
    ]
  })
}
