output "cluster_name" {
  value = aws_eks_cluster.main.name
}

output "cluster_endpoint" {
  value = aws_eks_cluster.main.endpoint
}

output "region" {
  value = var.region
}

output "database_secret_arn" {
  value       = aws_secretsmanager_secret.db.arn
  description = "Pods read the connection string from here via IRSA rather than receiving it as a Helm value."
}

output "redis_endpoint" {
  value = aws_elasticache_replication_group.main.primary_endpoint_address
}

output "ecr_registry" {
  value = split("/", aws_ecr_repository.service["api"].repository_url)[0]
}

output "kubeconfig_command" {
  value = "aws eks update-kubeconfig --region ${var.region} --name ${aws_eks_cluster.main.name}"
}

# Everything above is safe to print. The database password is deliberately not
# an output: it would land in state and in any CI log that runs `terraform
# output`. Read it from Secrets Manager instead.
