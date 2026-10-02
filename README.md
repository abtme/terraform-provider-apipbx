# terraform-provider-apipbx

Terraform provider for [apipbx](../apipbx). Resources are added in step with the
apipbx API; currently `apipbx_tenant` only (delete is a stub until the API has
DELETE /v1/tenants/{id}).

    provider "apipbx" { endpoint = "https://pbx.example.com", token = "..." }  # or APIPBX_ENDPOINT / APIPBX_TOKEN
