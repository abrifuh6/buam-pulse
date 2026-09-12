# VPC flow logs
# -----------------------------------------------------------------------------
# The only way to answer "what connected to what, when" after an incident.
# Without them, a question like "did anything reach the database from outside
# the node group" has no evidence either way.
#
# REJECT traffic only, rather than ALL: rejected connections are the
# interesting ones — probes, misconfigurations, and anything trying to reach
# something it should not — and they are a tiny fraction of the volume, which
# keeps ingestion cost near zero.

resource "aws_cloudwatch_log_group" "flow_logs" {
  name              = "/aws/vpc/${var.name}-${var.environment}"
  retention_in_days = 14
  kms_key_id        = aws_kms_key.cluster.arn

  # checkov:skip=CKV_AWS_338: See the EKS log group — retention is a cost
  # decision, not a security one, on a project with no audit obligation.

  depends_on = [aws_kms_key_policy.cluster]
}

resource "aws_iam_role" "flow_logs" {
  name = "${var.name}-${var.environment}-flow-logs"

  assume_role_policy = jsonencode({
    Version = "2012-10-17"
    Statement = [{
      Effect    = "Allow"
      Principal = { Service = "vpc-flow-logs.amazonaws.com" }
      Action    = "sts:AssumeRole"
    }]
  })
}

resource "aws_iam_role_policy" "flow_logs" {
  name = "write-logs"
  role = aws_iam_role.flow_logs.id

  policy = jsonencode({
    Version = "2012-10-17"
    Statement = [{
      Effect = "Allow"
      Action = [
        "logs:CreateLogStream",
        "logs:PutLogEvents",
        "logs:DescribeLogGroups",
        "logs:DescribeLogStreams",
      ]
      Resource = "${aws_cloudwatch_log_group.flow_logs.arn}:*"
    }]
  })
}

resource "aws_flow_log" "main" {
  vpc_id               = aws_vpc.main.id
  traffic_type         = "REJECT"
  log_destination_type = "cloud-watch-logs"
  log_destination      = aws_cloudwatch_log_group.flow_logs.arn
  iam_role_arn         = aws_iam_role.flow_logs.arn

  tags = { Name = "${var.name}-${var.environment}" }
}
