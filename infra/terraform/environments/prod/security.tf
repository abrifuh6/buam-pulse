# Security groups
# -----------------------------------------------------------------------------
# Rules reference other security groups rather than CIDR ranges wherever
# possible. "Postgres accepts connections from the node group" stays true when
# subnets are renumbered or nodes replaced; "Postgres accepts 10.40.64.0/20"
# quietly becomes wrong the moment the network changes.

resource "aws_security_group" "nodes" {
  name_prefix = "${var.name}-nodes-"
  description = "EKS worker nodes"
  vpc_id      = aws_vpc.main.id

  # Nodes need outbound to pull images, reach AWS APIs, and — since this is a
  # monitoring product — check customer endpoints anywhere on the internet.
  egress {
    description = "All outbound"
    from_port   = 0
    to_port     = 0
    protocol    = "-1"
    cidr_blocks = ["0.0.0.0/0"]
  }

  lifecycle { create_before_destroy = true }
  tags = { Name = "${var.name}-nodes" }
}

resource "aws_security_group" "database" {
  name_prefix = "${var.name}-db-"
  description = "RDS PostgreSQL"
  vpc_id      = aws_vpc.main.id

  lifecycle { create_before_destroy = true }
  tags = { Name = "${var.name}-db" }
}

# Separate rule resources rather than inline blocks: inline ingress inside a
# security group that is itself referenced by another creates a dependency
# cycle Terraform cannot resolve.
resource "aws_vpc_security_group_ingress_rule" "db_from_nodes" {
  security_group_id            = aws_security_group.database.id
  referenced_security_group_id = aws_security_group.nodes.id
  from_port                    = 5432
  to_port                      = 5432
  ip_protocol                  = "tcp"
  description                  = "PostgreSQL from EKS nodes"
}

resource "aws_security_group" "cache" {
  name_prefix = "${var.name}-cache-"
  description = "ElastiCache Redis"
  vpc_id      = aws_vpc.main.id

  lifecycle { create_before_destroy = true }
  tags = { Name = "${var.name}-cache" }
}

resource "aws_vpc_security_group_ingress_rule" "cache_from_nodes" {
  security_group_id            = aws_security_group.cache.id
  referenced_security_group_id = aws_security_group.nodes.id
  from_port                    = 6379
  to_port                      = 6379
  ip_protocol                  = "tcp"
  description                  = "Redis from EKS nodes"
}
