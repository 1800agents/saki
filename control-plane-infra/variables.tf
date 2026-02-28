variable "project_name" {
  description = "Short project identifier used for naming."
  type        = string
}

variable "environment" {
  description = "Deployment environment name (for example: dev, staging, prod)."
  type        = string

  validation {
    condition     = can(regex("^[a-z0-9-]+$", var.environment))
    error_message = "environment must contain only lowercase letters, digits, and hyphens."
  }
}

variable "region" {
  description = "Target region for infrastructure resources."
  type        = string
}

variable "tags" {
  description = "Additional tags/labels to apply to resources."
  type        = map(string)
  default     = {}
}
