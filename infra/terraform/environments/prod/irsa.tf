# IAM Roles for Service Accounts
# -----------------------------------------------------------------------------
# A pod that needs AWS permissions has three options: a long-lived access key in
# a Secret, the node's instance role, or IRSA.
#
# A key in a Secret has to be rotated by hand and leaks if the Secret does. The
# node role gives every pod on that node the same permissions, so a compromised
# frontend can read the database credentials. IRSA scopes permissions to one
# service account in one namespace, with tokens the cluster rotates automatically.

data "tls_certificate" "cluster" {
  url = aws_eks_cluster.main.identity[0].oidc[0].issuer
}

resource "aws_iam_openid_connect_provider" "cluster" {
  url             = aws_eks_cluster.main.identity[0].oidc[0].issuer
  client_id_list  = ["sts.amazonaws.com"]
  thumbprint_list = [data.tls_certificate.cluster.certificates[0].sha1_fingerprint]

  tags = { Name = "${var.name}-${var.environment}-oidc" }
}

locals {
  oidc_issuer = replace(aws_iam_openid_connect_provider.cluster.url, "https://", "")
}

# --- External Secrets: reads credentials from Secrets Manager ----------------
#
# The alternative is passing the database URL as a Helm value, which puts it in
# the release history, in any CI log that renders the chart, and in the shell
# history of whoever deployed it. External Secrets keeps it in AWS and
# synthesises a Kubernetes Secret at runtime.

resource "aws_iam_role" "external_secrets" {
  name = "${var.name}-${var.environment}-external-secrets"

  assume_role_policy = jsonencode({
    Version = "2012-10-17"
    Statement = [{
      Effect    = "Allow"
      Principal = { Federated = aws_iam_openid_connect_provider.cluster.arn }
      Action    = "sts:AssumeRoleWithWebIdentity"
      Condition = {
        StringEquals = {
          # Both conditions matter. Without the subject check, any service
          # account in the cluster could assume this role; without the audience
          # check, a token minted for another service would be accepted.
          "${local.oidc_issuer}:sub" = "system:serviceaccount:external-secrets:external-secrets"
          "${local.oidc_issuer}:aud" = "sts.amazonaws.com"
        }
      }
    }]
  })
}

resource "aws_iam_role_policy" "external_secrets" {
  name = "read-pulse-secrets"
  role = aws_iam_role.external_secrets.id

  policy = jsonencode({
    Version = "2012-10-17"
    Statement = [{
      Effect   = "Allow"
      Action   = ["secretsmanager:GetSecretValue", "secretsmanager:DescribeSecret"]
      # Scoped to this project's secrets by path, not "*". A wildcard here would
      # let a compromised controller read every secret in the account.
      Resource = "arn:aws:secretsmanager:${var.region}:${data.aws_caller_identity.current.account_id}:secret:${var.name}/${var.environment}/*"
    }]
  })
}

# --- Load balancer controller ------------------------------------------------

resource "aws_iam_role" "alb_controller" {
  name = "${var.name}-${var.environment}-alb-controller"

  assume_role_policy = jsonencode({
    Version = "2012-10-17"
    Statement = [{
      Effect    = "Allow"
      Principal = { Federated = aws_iam_openid_connect_provider.cluster.arn }
      Action    = "sts:AssumeRoleWithWebIdentity"
      Condition = {
        StringEquals = {
          "${local.oidc_issuer}:sub" = "system:serviceaccount:kube-system:aws-load-balancer-controller"
          "${local.oidc_issuer}:aud" = "sts.amazonaws.com"
        }
      }
    }]
  })
}

# The controller's policy is long and maintained upstream by AWS; vendoring it
# keeps the permissions pinned to a version we reviewed rather than whatever a
# remote URL returns today.
resource "aws_iam_policy" "alb_controller" {
  name   = "${var.name}-${var.environment}-alb-controller"
  policy = file("${path.module}/policies/alb-controller.json")
}

resource "aws_iam_role_policy_attachment" "alb_controller" {
  role       = aws_iam_role.alb_controller.name
  policy_arn = aws_iam_policy.alb_controller.arn
}

output "external_secrets_role_arn" {
  value = aws_iam_role.external_secrets.arn
}

output "alb_controller_role_arn" {
  value = aws_iam_role.alb_controller.arn
}
