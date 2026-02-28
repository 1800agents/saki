variable "tailscale_oauth_client_id" {
  description = "Tailscale OAuth client ID"
  type        = string
  sensitive   = true
}

variable "tailscale_oauth_client_secret" {
  description = "Tailscale OAuth client secret"
  type        = string
  sensitive   = true
}

# resource "kubernetes_namespace" "tailscale" {
#   metadata {
#     name = "tailscale"
#     labels = {
#       name = "tailscale"
#     }
#   }
# }

resource "helm_release" "tailscale-operator" {
  name       = "tailscale-operator"
  repository = "https://pkgs.tailscale.com/helmcharts"
  chart      = "tailscale-operator"
  version    = "v1.86.2"

  create_namespace = false
  namespace        = "tailscale"

  # set {
  #   name  = "oauth.clientId"
  #   value = var.tailscale_oauth_client_id
  #   type  = "string"
  # }

  # set {
  #   name  = "oauth.clientSecret"
  #   value = var.tailscale_oauth_client_secret
  #   type  = "string"
  # }

  lifecycle {
    ignore_changes = [set]
  }
}
