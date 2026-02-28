terraform {
  required_providers {
    kubernetes = {
      source  = "hashicorp/kubernetes"
      version = "2.36.0"
    }
    helm = {
      source  = "hashicorp/helm"
      version = "2.17.0"
    }
    random = {
      source  = "hashicorp/random"
      version = "~> 3.0"
    }
  }
}

provider "kubernetes" {
  config_path    = var.kubeconfig_path
  config_context = var.kubeconfig_context
}

provider "helm" {
  kubernetes {
    config_path    = var.kubeconfig_path
    config_context = var.kubeconfig_context
  }
}

# base resources: namespaces and secrets - create these first specifically
module "base" {
  source = "./modules/base"

  kubeconfig_path    = var.kubeconfig_path
  kubeconfig_context = var.kubeconfig_context
}

module "tailscale" {
  source = "./modules/tailscale"

  tailscale_oauth_client_id     = var.tailscale_oauth_client_id
  tailscale_oauth_client_secret = var.tailscale_oauth_client_secret
}

module "db" {
  source = "./modules/db"

  postgres_secret_name = module.base.secret_names["postgres-secret"]
}

module "registry" {
  source = "./modules/registry"

  depends_on = [module.tailscale]
}

module "control_plane" {
  source = "./modules/control-plane"

  image          = var.control_plane_image
  frontend_image = var.control_plane_frontend_image
  postgres_host  = module.db.postgres_host

  depends_on = [module.base, module.db, module.tailscale]
}
