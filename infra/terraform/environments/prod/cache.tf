# Redis
# -----------------------------------------------------------------------------
# Redis holds the check queue. Losing it means losing whatever checks were
# waiting, which is recoverable: the scheduler re-enqueues on its next tick and
# at most one interval of checks is missed. That tolerance is why this runs as a
# single node with no replica — the cheapest option that is honest about what it
# is protecting.

resource "aws_elasticache_subnet_group" "main" {
  name       = "${var.name}-${var.environment}"
  subnet_ids = aws_subnet.data[*].id
}

resource "aws_elasticache_replication_group" "main" {
  replication_group_id = "${var.name}-${var.environment}"
  description          = "Pulse check queue"

  engine         = "valkey"
  engine_version = "7.2"
  node_type      = var.cache_node_type
  port           = 6379

  num_cache_clusters         = 1
  automatic_failover_enabled = false # requires two nodes; see the note above
  multi_az_enabled           = false

  subnet_group_name  = aws_elasticache_subnet_group.main.name
  security_group_ids = [aws_security_group.cache.id]

  # Encryption in transit is off deliberately: the queue carries monitor IDs,
  # nothing sensitive, the subnet has no internet route, and TLS on the client
  # adds a handshake to every dequeue in a loop that runs continuously. Turn it
  # on the moment anything identifying goes through this queue.
  at_rest_encryption_enabled = true
  transit_encryption_enabled = false

  # A queue is not a cache. If Redis fills, dropping the oldest entries would
  # silently lose scheduled checks, so writes fail loudly instead and the
  # scheduler retries on its next tick.
  parameter_group_name = aws_elasticache_parameter_group.main.name

  maintenance_window       = "sun:10:00-sun:11:00"
  snapshot_retention_limit = 0 # a rebuildable queue does not need snapshots

  tags = { Name = "${var.name}-${var.environment}" }
}

resource "aws_elasticache_parameter_group" "main" {
  # ElastiCache parameter groups require an explicit name — unlike RDS ones,
  # they do not support name_prefix. Changing a parameter therefore replaces
  # the group by name, which is why create_before_destroy matters here.
  name   = "${var.name}-${var.environment}-valkey"
  family = "valkey7"

  parameter {
    name  = "maxmemory-policy"
    value = "noeviction"
  }

  lifecycle { create_before_destroy = true }
}
