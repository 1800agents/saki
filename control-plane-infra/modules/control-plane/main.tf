variable "image" {
  description = "Docker image for the control plane (e.g. registry.corgi-teeth.ts.net/saki/control-plane:latest)."
  type        = string
}

variable "frontend_image" {
  description = "Docker image for the control plane frontend (e.g. registry.corgi-teeth.ts.net/saki/control-plane-frontend:latest)."
  type        = string
}

variable "postgres_host" {
  description = "PostgreSQL host (e.g. from db module output)."
  type        = string
}

variable "registry_host" {
  description = "Docker registry host for app images."
  type        = string
  default     = "registry.corgi-teeth.ts.net"
}

variable "app_base_domain" {
  description = "Base domain used when generating ingress hostnames for deployed apps."
  type        = string
  default     = "corgi-teeth.ts.net"
}

variable "k8s_apps_namespace" {
  description = "Namespace where user apps are deployed."
  type        = string
  default     = "saki-apps"
}

variable "tailscale_hostname" {
  description = "Tailscale hostname for the control plane (becomes <hostname>.<tailnet>.ts.net)."
  type        = string
  default     = "saki-control-plane"
}

variable "frontend_tailscale_hostname" {
  description = "Tailscale hostname for the control plane frontend (becomes <hostname>.<tailnet>.ts.net)."
  type        = string
  default     = "saki"
}

variable "replicas" {
  description = "Number of control plane replicas."
  type        = number
  default     = 1
}

variable "frontend_replicas" {
  description = "Number of control plane frontend replicas."
  type        = number
  default     = 1
}

variable "default_app_ttl_hours" {
  description = "Default TTL in hours for deployed apps."
  type        = number
  default     = 168
}

locals {
  namespace = "saki"
}

# ── Namespaces ────────────────────────────────────────────────────────────────

resource "kubernetes_namespace" "control_plane" {
  metadata {
    name   = local.namespace
    labels = { name = local.namespace }
  }
}

resource "kubernetes_namespace" "saki_apps" {
  metadata {
    name   = var.k8s_apps_namespace
    labels = { name = var.k8s_apps_namespace }
  }
}

# ── Secret ────────────────────────────────────────────────────────────────────

resource "kubernetes_secret" "control_plane" {
  metadata {
    name      = "control-plane"
    namespace = local.namespace
  }

  data = {
    ADMIN_TOKENS      = ""
    POSTGRES_PASSWORD = ""
  }

  lifecycle {
    ignore_changes = [data]
  }

  depends_on = [kubernetes_namespace.control_plane]
}

# ── RBAC ─────────────────────────────────────────────────────────────────────

resource "kubernetes_manifest" "service_account" {
  manifest = yamldecode(templatefile("${path.module}/manifests/serviceaccount.yaml", {
    namespace = local.namespace
  }))

  depends_on = [kubernetes_namespace.control_plane]
}

resource "kubernetes_manifest" "apps_role" {
  manifest = yamldecode(templatefile("${path.module}/manifests/role.yaml", {
    apps_namespace = var.k8s_apps_namespace
  }))

  depends_on = [kubernetes_namespace.saki_apps]
}

resource "kubernetes_manifest" "apps_role_binding" {
  manifest = yamldecode(templatefile("${path.module}/manifests/rolebinding.yaml", {
    namespace      = local.namespace
    apps_namespace = var.k8s_apps_namespace
  }))

  depends_on = [kubernetes_manifest.apps_role, kubernetes_manifest.service_account]
}

# ── Deployment ────────────────────────────────────────────────────────────────

resource "kubernetes_manifest" "deployment" {
  manifest = yamldecode(templatefile("${path.module}/manifests/deployment.yaml", {
    namespace             = local.namespace
    image                 = var.image
    replicas              = var.replicas
    registry_host         = var.registry_host
    app_base_domain       = var.app_base_domain
    apps_namespace        = var.k8s_apps_namespace
    default_app_ttl_hours = var.default_app_ttl_hours
    postgres_host         = var.postgres_host
    secret_name           = kubernetes_secret.control_plane.metadata[0].name
  }))

  depends_on = [
    kubernetes_namespace.control_plane,
    kubernetes_manifest.service_account,
    kubernetes_manifest.apps_role_binding,
    kubernetes_secret.control_plane,
  ]
}

# ── Service ───────────────────────────────────────────────────────────────────

resource "kubernetes_manifest" "service" {
  manifest = yamldecode(templatefile("${path.module}/manifests/service.yaml", {
    namespace = local.namespace
  }))

  depends_on = [kubernetes_namespace.control_plane]
}

# ── Ingress ───────────────────────────────────────────────────────────────────

resource "kubernetes_manifest" "ingress" {
  manifest = yamldecode(templatefile("${path.module}/manifests/ingress.yaml", {
    namespace          = local.namespace
    tailscale_hostname = var.tailscale_hostname
  }))

  depends_on = [kubernetes_manifest.service]
}

# ── Frontend Deployment ───────────────────────────────────────────────────────

resource "kubernetes_manifest" "frontend_deployment" {
  manifest = yamldecode(templatefile("${path.module}/manifests/frontend-deployment.yaml", {
    namespace = local.namespace
    image     = var.frontend_image
    replicas  = var.frontend_replicas
  }))

  depends_on = [kubernetes_namespace.control_plane]
}

# ── Frontend Service ──────────────────────────────────────────────────────────

resource "kubernetes_manifest" "frontend_service" {
  manifest = yamldecode(templatefile("${path.module}/manifests/frontend-service.yaml", {
    namespace = local.namespace
  }))

  depends_on = [kubernetes_manifest.frontend_deployment]
}

# ── Frontend Ingress ──────────────────────────────────────────────────────────

resource "kubernetes_manifest" "frontend_ingress" {
  manifest = yamldecode(templatefile("${path.module}/manifests/frontend-ingress.yaml", {
    namespace          = local.namespace
    tailscale_hostname = var.frontend_tailscale_hostname
  }))

  depends_on = [kubernetes_manifest.frontend_service]
}

# ── Outputs ───────────────────────────────────────────────────────────────────

output "url" {
  value       = "https://${var.tailscale_hostname}.<tailnet>.ts.net"
  description = "Control plane backend URL on the tailnet. Replace <tailnet> with your actual tailnet name."
}

output "frontend_url" {
  value       = "https://${var.frontend_tailscale_hostname}.<tailnet>.ts.net"
  description = "Control plane frontend URL on the tailnet. Replace <tailnet> with your actual tailnet name."
}
