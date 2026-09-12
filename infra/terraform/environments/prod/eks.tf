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

  # Kubernetes Secrets in etcd are base64, not encrypted, unless envelope
  # encryption is configured. This is the difference between "someone read our
  # etcd backup" being an incident and being a breach.
  encryption_config {
    provider {
      key_arn = aws_kms_key.cluster.arn
    }
    resources = ["secrets"]
  }

  # All five log types. Audit answers "who deleted that deployment";
  # controllerManager and scheduler answer "why did that pod never start",
  # which is the question during an incident at 2am.
  enabled_cluster_log_types = [
    "api", "audit", "authenticator", "controllerManager", "scheduler",
  ]

  depends_on = [aws_iam_role_policy_attachment.cluster_policy]

  tags = { Name = "${var.name}-${var.environment}" }
}

# Control-plane logs otherwise accumulate forever at $0.50/GB/month.
resource "aws_cloudwatch_log_group" "cluster" {
  name              = "/aws/eks/${var.name}-${var.environment}/cluster"
  retention_in_days = 14
  kms_key_id        = aws_kms_key.cluster.arn

  # checkov:skip=CKV_AWS_338: Fourteen days, not a year. Audit logs are billed
  # per GB ingested and per GB stored; a year of control-plane logs on a
  # project with no compliance obligation is a bill, not a control. A regulated
  # deployment would set 365 and accept the cost.

  depends_on = [aws_kms_key_policy.cluster]
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

# A managed node group cannot set metadata options directly; they belong to a
# launch template, which the group then references. This is the usual reason to
# introduce a launch template into an otherwise managed setup.
resource "aws_launch_template" "nodes" {
  name_prefix = "${var.name}-${var.environment}-"

  instance_type = var.node_instance_types[0]

  # IMDSv2 required, with a hop limit of 1 so a container cannot reach the
  # instance metadata service through the pod network. IMDSv1 lets any process
  # that can issue an HTTP GET read the node's IAM credentials — the mechanism
  # behind several well-known cloud breaches.
  metadata_options {
    http_endpoint               = "enabled"
    http_tokens                 = "required"
    http_put_response_hop_limit = 1
    instance_metadata_tags      = "enabled"
  }

  monitoring {
    enabled = true
  }

  # Tags must be set on the template to reach the instances it launches;
  # provider default_tags do not propagate through a launch template.
  tag_specifications {
    resource_type = "instance"
    tags = {
      Name = "${var.name}-${var.environment}-node"
    }
  }

  lifecycle { create_before_destroy = true }
}

resource "aws_eks_node_group" "main" {
  cluster_name    = aws_eks_cluster.main.name
  node_group_name = "${var.name}-${var.environment}-nodes"
  node_role_arn   = aws_iam_role.nodes.arn
  subnet_ids      = aws_subnet.private[*].id

  capacity_type = var.node_capacity_type

  # Instance type lives on the launch template rather than here: a node group
  # can set one or the other, not both.
  launch_template {
    id      = aws_launch_template.nodes.id
    version = aws_launch_template.nodes.latest_version
  }

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
