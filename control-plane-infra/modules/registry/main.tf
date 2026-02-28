variable "storage_class" {
  description = "Storage class for the registry PVC."
  type        = string
  default     = "local-path"
}

variable "storage_size" {
  description = "Size of the registry persistent volume."
  type        = string
  default     = "20Gi"
}

variable "tailscale_hostname" {
  description = "Hostname to use on the tailnet (e.g. 'registry' becomes registry.<tailnet>.ts.net)."
  type        = string
  default     = "registry"
}

locals {
  namespace = "registry"
}

resource "kubernetes_namespace" "registry" {
  metadata {
    name   = local.namespace
    labels = { name = local.namespace }
  }
}

resource "helm_release" "registry" {
  name             = "docker-registry"
  repository       = "https://twuni.github.io/docker-registry.helm"
  chart            = "docker-registry"
  namespace        = local.namespace
  create_namespace = false

  set {
    name  = "persistence.enabled"
    value = "true"
  }

  set {
    name  = "persistence.size"
    value = var.storage_size
  }

  set {
    name  = "persistence.storageClass"
    value = var.storage_class
  }

  depends_on = [kubernetes_namespace.registry]
}

resource "kubernetes_manifest" "registry_ingress" {
  manifest = {
    apiVersion = "networking.k8s.io/v1"
    kind       = "Ingress"
    metadata = {
      name      = "registry"
      namespace = local.namespace
      annotations = {
        "tailscale.com/hostname" = var.tailscale_hostname
      }
    }
    spec = {
      ingressClassName = "tailscale"
      rules = [{
        host = var.tailscale_hostname
        http = {
          paths = [{
            path     = "/"
            pathType = "Prefix"
            backend = {
              service = {
                name = "docker-registry"
                port = { number = 5000 }
              }
            }
          }]
        }
      }]
      tls = [{
        hosts = [var.tailscale_hostname]
      }]
    }
  }

  depends_on = [helm_release.registry]
}

output "registry_host" {
  value       = "${var.tailscale_hostname}.<tailnet>.ts.net"
  description = "Registry hostname on the tailnet. Replace <tailnet> with your actual tailnet name."
}
