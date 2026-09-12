# Envelope encryption for Kubernetes Secrets.
#
# Without this, a Secret in etcd is base64-encoded, not encrypted — anyone with
# etcd access or a control-plane backup can read every credential in the
# cluster. With it, the data key is itself encrypted by KMS, so reading etcd is
# not enough.
#
# One key, roughly $1/month, used for the cluster and the log group.
resource "aws_kms_key" "cluster" {
  description             = "Pulse EKS secret envelope encryption"
  deletion_window_in_days = 7
  enable_key_rotation     = true

  tags = { Name = "${var.name}-${var.environment}-eks" }
}

resource "aws_kms_alias" "cluster" {
  name          = "alias/${var.name}-${var.environment}-eks"
  target_key_id = aws_kms_key.cluster.key_id
}

# CloudWatch needs explicit permission to use the key, or log delivery fails
# silently and the log group simply stays empty.
data "aws_caller_identity" "current" {}

resource "aws_kms_key_policy" "cluster" {
  key_id = aws_kms_key.cluster.id

  policy = jsonencode({
    Version = "2012-10-17"
    Statement = [
      {
        Sid       = "EnableRootAccess"
        Effect    = "Allow"
        Principal = { AWS = "arn:aws:iam::${data.aws_caller_identity.current.account_id}:root" }
        Action    = "kms:*"
        Resource  = "*"
      },
      {
        Sid       = "AllowCloudWatchLogs"
        Effect    = "Allow"
        Principal = { Service = "logs.${var.region}.amazonaws.com" }
        Action = [
          "kms:Encrypt*", "kms:Decrypt*", "kms:ReEncrypt*",
          "kms:GenerateDataKey*", "kms:Describe*",
        ]
        Resource = "*"
        Condition = {
          ArnLike = {
            "kms:EncryptionContext:aws:logs:arn" = "arn:aws:logs:${var.region}:${data.aws_caller_identity.current.account_id}:log-group:*"
          }
        }
      },
    ]
  })
}
