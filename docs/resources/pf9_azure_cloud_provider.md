# pf9_azure_cloud_provider (Resource)

The Azure cloud provider resource allows you to use your Azure account as the infrastructure provider for your Platform9 managed clusters. This is an optional resource. If you have already [created an Azure cloud provider](https://platform9.com/docs/kubernetes/getting-started-with-platform9-free-tier-on-azure) in your Platform9 account you can provide its Id as `cloud_provider_uuid` in the `pf9_cluster` resource and skip creating this resource.

## Example Usage

```terraform
resource "pf9_azure_cloud_provider" "sample_azure_prov" {
    name                = "sample_azure_provider"
    type                = "azure"
    project_uuid        = "<YOUR_P9_PROJECT_UUID>"
    client_id           = "<YOUR_AZURE_CLIENT>"
    client_secret       = "<YOUR_AZURE_SECRET>"
    subscription_id     = "<YOUR_AZURE_SUBSCRIPTION>"
    tenant_id           = "<YOUR_AZURE_TENANT_ID>"
}
```

## Schema

### Required

- **name** (String) The name of the cloud provider. This is used internally by P9.
- **type** (String) Set to "azure".
- **project_uuid** (String) Your Platform9 project UUID. Learn how to find this value, [here](https://platform9.com/docs/kubernetes/introduction-to-platform9-uuid#tenants--project-uuid).
- **client_id** (String) Your Azure id.
- **client_secret** (String) Your Azure secret.
- **subscription_id** (String) Your Azure subscription id.
- **tenant_id** (String) Azure Tenant ID.
