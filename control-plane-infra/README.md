# Homelab Infra

## Setup

### Base
Create the base namespaces and secrets
```shell
terraform apply -target=module.base
```

### Edit Secrets
Manually set the Kubernetes secrets created in the previous step

### DB
Create the postgres DB
```shell
terraform apply -target=module.db
```

### Tailscale
Get OAuth credentials from the [Tailscale admin console](https://login.tailscale.com/admin/settings/oauth):
1. Go to **Settings → OAuth clients → Generate OAuth client**
2. Grant the **Devices** scope with write access
3. Copy the client ID and secret, then pass them to Terraform:

```shell
terraform apply \
  -var="tailscale_oauth_client_id=<client-id>" \
  -var="tailscale_oauth_client_secret=<client-secret>"
```

Or set them in a `terraform.tfvars` file (do not commit this file):
```hcl
tailscale_oauth_client_id     = "<client-id>"
tailscale_oauth_client_secret = "<client-secret>"
```

### Everything else
```shell
terraform apply
```

## Format Files
```
terraform fmt --recursive
```