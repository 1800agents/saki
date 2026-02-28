output "stack_name" {
  description = "Computed stack name shared by infrastructure resources."
  value       = local.stack_name
}

output "region" {
  description = "Selected deployment region."
  value       = var.region
}

output "common_tags" {
  description = "Merged standard tags/labels for resources."
  value       = local.common_tags
}
