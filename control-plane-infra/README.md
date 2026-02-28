# Terraform IaC

Terraform baseline for Saki infrastructure.

## Files

- `versions.tf`: Terraform CLI version constraint.
- `variables.tf`: Input variables for project/environment/region/tags.
- `locals.tf`: Shared naming and default tagging logic.
- `main.tf`: Root module entry point where provider blocks and resources are added.
- `outputs.tf`: Common outputs reused by automation and CI.
- `terraform.tfvars.example`: Example input values.

## Quick start

```bash
cd control-plane-infra
cp terraform.tfvars.example terraform.tfvars
terraform init
terraform fmt -recursive
terraform validate
terraform plan
```

## Notes

- State uses Terraform defaults (local state) until a backend is configured.
- Keep provider/resource definitions in `main.tf` or split into additional `*.tf` files as the stack grows.
