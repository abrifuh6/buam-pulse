# PostgreSQL
# -----------------------------------------------------------------------------
# Managed rather than self-hosted in the cluster. Running Postgres on Kubernetes
# is possible and occasionally correct, but it means owning backups, failover,
# patching and storage expansion — four jobs RDS does for roughly $15/month at
# this size. The database is the one thing here that cannot be recreated from a
# git repository.

resource "random_password" "db" {
  length = 32
  # Excludes characters that break connection strings when they appear
  # unescaped: @ separates credentials from host, / separates the database name,
  # and quotes cause trouble in shell-expanded environments.
  override_special = "!#$%&*()-_=+[]{}<>:?"
}

# Credentials live in Secrets Manager, not in state or a tfvars file. State is
# encrypted but still contains the value; Secrets Manager gives rotation,
# per-secret IAM, and an audit trail of every read.
resource "aws_secretsmanager_secret" "db" {
  name        = "${var.name}/${var.environment}/database"
  description = "Pulse PostgreSQL credentials"
  # Zero means a deleted secret is gone immediately rather than lingering for a
  # week and blocking a redeploy under the same name. Appropriate here; a real
  # production secret should keep the recovery window.
  recovery_window_in_days = 0
}

resource "aws_secretsmanager_secret_version" "db" {
  secret_id = aws_secretsmanager_secret.db.id
  secret_string = jsonencode({
    username = aws_db_instance.main.username
    password = random_password.db.result
    host     = aws_db_instance.main.address
    port     = aws_db_instance.main.port
    dbname   = aws_db_instance.main.db_name
    url = format(
      "postgres://%s:%s@%s:%d/%s?sslmode=require",
      aws_db_instance.main.username,
      urlencode(random_password.db.result),
      aws_db_instance.main.address,
      aws_db_instance.main.port,
      aws_db_instance.main.db_name,
    )
  })
}

resource "aws_db_subnet_group" "main" {
  name       = "${var.name}-${var.environment}"
  subnet_ids = aws_subnet.data[*].id
  tags       = { Name = "${var.name}-${var.environment}" }
}

# Parameter group rather than the default, so settings are versioned here.
resource "aws_db_parameter_group" "main" {
  name_prefix = "${var.name}-${var.environment}-"
  family      = "postgres16"

  # Refuse unencrypted connections. The database is in a subnet with no
  # internet route, but defence in depth means the transport is encrypted too.
  parameter {
    name  = "rds.force_ssl"
    value = "1"
  }

  # Log anything slower than a second. Pulse writes constantly, so this is the
  # first place a query-shaped performance problem will show up.
  parameter {
    name  = "log_min_duration_statement"
    value = "1000"
  }

  lifecycle { create_before_destroy = true }
}

resource "aws_db_instance" "main" {
  identifier     = "${var.name}-${var.environment}"
  engine         = "postgres"
  engine_version = "16.4"
  instance_class = var.db_instance_class

  db_name  = "pulse"
  username = "pulse"
  password = random_password.db.result

  # gp3 rather than gp2: same price, better baseline throughput, and IOPS can
  # be raised without growing the volume.
  allocated_storage     = 20
  max_allocated_storage = 100 # autoscaling cap, so a runaway table cannot cost unbounded money
  storage_type          = "gp3"
  storage_encrypted     = true

  db_subnet_group_name   = aws_db_subnet_group.main.name
  vpc_security_group_ids = [aws_security_group.database.id]
  parameter_group_name   = aws_db_parameter_group.main.name
  publicly_accessible    = false

  backup_retention_period = 7
  backup_window           = "08:00-09:00" # 01:00 Pacific, the quietest hour
  maintenance_window      = "sun:09:00-sun:10:00"

  # Single-AZ. Multi-AZ doubles the cost for automatic failover; with daily
  # backups and a seven-day window, the exposure is measured in minutes of
  # recovery rather than data loss. A revenue-carrying deployment should turn
  # this on.
  multi_az = false

  # Minor versions apply themselves in the maintenance window; majors never do,
  # because a major upgrade can break queries and must be deliberate.
  auto_minor_version_upgrade  = true
  allow_major_version_upgrade = false

  # A final snapshot on destroy would block `terraform destroy` between working
  # sessions and accumulate storage cost. Correct for this project, wrong for
  # anything holding customer data.
  skip_final_snapshot = true
  deletion_protection = false

  # Query-level insight at no cost on the free tier.
  performance_insights_enabled          = true
  performance_insights_retention_period = 7

  # OS-level metrics at 60s. Performance Insights shows which queries are slow;
  # enhanced monitoring shows whether the instance is starved of CPU, memory or
  # IO underneath them. The two answer different halves of "why is it slow".
  monitoring_interval = 60
  monitoring_role_arn = aws_iam_role.rds_monitoring.arn

  # Postgres logs to CloudWatch, so a crash is diagnosable after the instance
  # has been replaced.
  enabled_cloudwatch_logs_exports = ["postgresql"]

  tags = { Name = "${var.name}-${var.environment}" }
}


resource "aws_iam_role" "rds_monitoring" {
  name = "${var.name}-${var.environment}-rds-monitoring"

  assume_role_policy = jsonencode({
    Version = "2012-10-17"
    Statement = [{
      Effect    = "Allow"
      Principal = { Service = "monitoring.rds.amazonaws.com" }
      Action    = "sts:AssumeRole"
    }]
  })
}

resource "aws_iam_role_policy_attachment" "rds_monitoring" {
  role       = aws_iam_role.rds_monitoring.name
  policy_arn = "arn:aws:iam::aws:policy/service-role/AmazonRDSEnhancedMonitoringRole"
}
