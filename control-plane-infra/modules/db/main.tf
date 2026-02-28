variable "postgres_secret_name" {
  type = string
}

locals {
  namespace = "db"
}

resource "helm_release" "postgresql" {
  namespace        = local.namespace
  create_namespace = false

  name       = "postgresql"
  repository = "oci://registry-1.docker.io/bitnamicharts"
  chart      = "postgresql"
  version    = "18.5.1"

  values = [templatefile("${path.module}/values.yaml", {
    existing_secret_name = var.postgres_secret_name
  })]
}

output "postgres_host" {
  value = "${helm_release.postgresql.name}.${local.namespace}.svc.cluster.local"
}