# pf9_aws_cloud_provider (Resource)

The AWS cloud provider resource allows you to use your AWS account as the infrastructure provider for your Platform9 managed clusters. This is an optional resource. If you have already [created an AWS cloud provider](https://platform9.com/docs/kubernetes/getting-started-with-platform9-free-tier-on-aws) in your Platform9 account you can provide its Id as `cloud_provider_uuid` in the `pf9_cluster` resource and skip creating this resource.

## Example Usage

```terraform
resource "pf9_aws_cloud_provider" "sample_aws_prov" {
    name                = "sample_aws_provider"
    type                = "aws"
    key                 = "<YOUR_AWS_KEY>"
    secret              = "<YOUR_AWS_SECRET>"
    project_uuid        = "<YOUR_P9_PROJECT_UUID>"
}
```

## Schema

### Required

- **name** (String) The name of the cloud provider. This is used internally by P9.
- **type** (String) Set to "aws".
- **key** (String) Your AWS account key.
- **secret** (String) Your AWS account secret.
- **project_uuid** (String) Your Platform9 project UUID. Learn how to find this value, [here](https://platform9.com/docs/kubernetes/introduction-to-platform9-uuid#tenants--project-uuid).
