variable "kubeconfig_path" {
  description = "Path to the kubeconfig file."
  type        = string
  default     = "~/.kube/saki.yaml"
}

variable "kubeconfig_context" {
  description = "Kubernetes context to use from the kubeconfig file."
  type        = string
  default     = "saki"
}

variable "tailscale_oauth_client_id" {
  description = "Tailscale OAuth client ID. Only required when creating tailscale operator, else can be left blank."
  type        = string
  sensitive   = true
}

variable "tailscale_oauth_client_secret" {
  description = "Tailscale OAuth client secret. Only required when creating tailscale operator, else can be left blank."
  type        = string
  sensitive   = true
}

variable "control_plane_image" {
  description = "Docker image for the control plane backend."
  type        = string
  default     = "registry.corgi-teeth.ts.net/saki/control-plane:latest"
}

variable "control_plane_frontend_image" {
  description = "Docker image for the control plane frontend."
  type        = string
  default     = "registry.corgi-teeth.ts.net/saki/control-plane-frontend:latest"
}
