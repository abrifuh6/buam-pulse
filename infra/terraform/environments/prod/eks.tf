# EKS
# -----------------------------------------------------------------------------
# One cluster, environments separated by namespace. Two clusters would be
# stronger isolation but doubles the $73/month control plane, and namespace
# separation with network policies is what most teams under fifty engineers
# actually run. See ADR 0008.

resource "aws_iam_role" "cluster" {
  name = "${var.name}-${var.environment}-cluster"

  assume_role_policy = jsonencode({
    Version = "2012-10-17"
    Statement = [{
      Effect    = "Allow"
      Principal = { Service = "eks.amazonaws.com" }
      Action    = "sts:AssumeRole"
    }]
  })
}

resource "aws_iam_role_policy_attachment" "cluster_policy" {
  role       = aws_iam_role.cluster.name
  policy_arn = "arn:aws:iam::aws:policy/AmazonEKSClusterPolicy"
}

resource "aws_eks_cluster" "main" {
  name     = "${var.name}-${var.environment}"
  role_arn = aws_iam_role.cluster.arn
  version  = var.cluster_version

  vpc_config {
    subnet_ids = concat(aws_subnet.private[*].id, aws_subnet.public[*].id)

    # The API server is reachable from the internet because there is no bastion
    # or VPN here and kubectl has to come from somewhere. It is authenticated
    # and authorised; public does not mean open. A production deployment with a
    # VPN would set this false and keep only private access.
    endpoint_public_access  = true
    endpoint_private_access = true
  }

  # API_AND_CONFIG_MAP rather than the older aws-auth ConfigMap alone: access
  # entries are real IAM resources Terraform can manage, instead of a YAML blob
  # that, if corrupted, locks everyone out of the cluster irrecoverably.
  access_config {
    authentication_mode                         = "API_AND_CONFIG_MAP"
    bootstrap_cluster_creator_admin_permissions = true
  }

  # Audit logs answer "who deleted that deployment". Control-plane logging is
  # not free, but it is the difference between an incident review and a shrug.
  enabled_cluster_log_types = ["api", "audit", "authenticator"]

  depends_on = [aws_iam_role_policy_attachment.cluster_policy]

  tags = { Name = "${var.name}-${var.environment}" }
}

# Control-plane logs otherwise accumulate forever at $0.50/GB/month.
resource "aws_cloudwatch_log_group" "cluster" {
  name              = "/aws/eks/${var.name}-${var.environment}/cluster"
  retention_in_days = 14
}

resource "aws_iam_role" "nodes" {
  name = "${var.name}-${var.environment}-nodes"

  assume_role_policy = jsonencode({
    Version = "2012-10-17"
    Statement = [{
      Effect    = "Allow"
      Principal = { Service = "ec2.amazonaws.com" }
      Action    = "sts:AssumeRole"
    }]
  })
}

resource "aws_iam_role_policy_attachment" "nodes" {
  for_each = toset([
    "arn:aws:iam::aws:policy/AmazonEKSWorkerNodePolicy",
    "arn:aws:iam::aws:policy/AmazonEKS_CNI_Policy",
    # Pull access only. Nodes never push images; CI does that with its own
    # credentials.
    "arn:aws:iam::aws:policy/AmazonEC2ContainerRegistryReadOnly",
  ])

  role       = aws_iam_role.nodes.name
  policy_arn = each.value
}

resource "aws_eks_node_group" "main" {
  cluster_name    = aws_eks_cluster.main.name
  node_group_name = "${var.name}-${var.environment}-nodes"
  node_role_arn   = aws_iam_role.nodes.arn
  subnet_ids      = aws_subnet.private[*].id

  instance_types = var.node_instance_types
  capacity_type  = var.node_capacity_type

  scaling_config {
    desired_size = 2
    min_size     = 2
    max_size     = 4
  }

  # One node at a time during upgrades, so capacity never halves mid-roll.
  update_config {
    max_unavailable = 1
  }

  depends_on = [aws_iam_role_policy_attachment.nodes]

  # Terraform should not fight the cluster autoscaler over replica count.
  lifecycle {
    ignore_changes = [scaling_config[0].desired_size]
  }

  tags = { Name = "${var.name}-${var.environment}-nodes" }
}

# Managed add-ons rather than hand-applied manifests: AWS keeps them compatible
# with the control-plane version, which is one fewer thing to remember on a
# cluster upgrade.
resource "aws_eks_addon" "core" {
  for_each = toset(["vpc-cni", "coredns", "kube-proxy", "eks-pod-identity-agent"])

  cluster_name                = aws_eks_cluster.main.name
  addon_name                  = each.value
  resolve_conflicts_on_update = "OVERWRITE"

  depends_on = [aws_eks_node_group.main]
}
