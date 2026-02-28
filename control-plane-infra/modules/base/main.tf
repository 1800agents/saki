variable "kubeconfig_path" {
  description = "Path to the kubeconfig file to pass to kubectl in local-exec provisioners."
  type        = string
  default     = "~/.kube/config"
}

variable "kubeconfig_context" {
  description = "Kubernetes context to pass to kubectl in local-exec provisioners."
  type        = string
  default     = "default"
}

locals {
  namespaces = [
    "tailscale",
    "db",
  ]

  secrets = {
    tailscale-secret = { namespace = "tailscale", name = "operator-oauth", keys = ["client_id", "client_secret"] }
    postgres-secret  = { namespace = "db", name = "postgres-secret", keys = ["postgres-password"] }
  }

  # flatten secrets x keys into a map keyed by "secret-key__field"
  secret_key_pairs = {
    for pair in flatten([
      for sk, sv in local.secrets : [
        for key in sv.keys : {
          id         = "${sk}__${key}"
          secret_key = sk
          key        = key
        }
      ]
    ]) : pair.id => pair
  }
}

resource "kubernetes_namespace" "namespaces" {
  count = length(local.namespaces)

  metadata {
    name   = local.namespaces[count.index]
    labels = { name = local.namespaces[count.index] }
  }
}

resource "random_password" "secret_values" {
  for_each = local.secret_key_pairs

  length  = 40
  special = false
}

resource "kubernetes_secret" "secrets" {
  for_each = local.secrets

  metadata {
    name      = each.value.name
    namespace = each.value.namespace
  }

  data = {
    for key in each.value.keys :
    key => random_password.secret_values["${each.key}__${key}"].result
  }

  depends_on = [kubernetes_namespace.namespaces]
}

output "namespace_names" {
  value = [for ns in kubernetes_namespace.namespaces : ns.metadata[0].name]
}

output "secret_names" {
  value = {
    for key, secret in kubernetes_secret.secrets :
    key => secret.metadata[0].name
  }
}
