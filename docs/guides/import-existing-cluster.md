---
page_title: "Import Pf9 Cluster in Terraform"
subcategory: ""
description: |-
  Import cluster to manage with terraform.
---

## Importing a PF9 Cluster into Terraform

### Overview

You can import an existing PF9 cluster into Terraform to manage it using Terraform's configuration language. This guide demonstrates how to import a PF9 cluster into Terraform state.

### Steps

1. **Identify the Cluster ID:**
   - Login to the PF9 web console.
   - Navigate to the **Infrastructure** tab and then select **Clusters**.
   - Select the Cluster you want to import and navigate to the **Cluster Details** tab .
   - Copy the cluster ID from **Unique ID** in Metadata table.

2. **Update Terraform Configuration:**
   - In your Terraform configuration file (e.g., `main.tf`), add the required provider configuration:

     ```terraform
     terraform {
       required_providers {
         pf9 = {
           source = "platform9/pf9"
         }
       }
     }
     ```

   - Add a `pf9_cluster` resource block to represent the PF9 cluster. Leave the attributes empty for now:

     ```terraform
     resource "pf9_cluster" "example" {
     }
     ```

3. **Import the Cluster:**
   - Open a terminal and navigate to your Terraform configuration directory.
   - Run the following command to import the cluster, replacing `<cluster_id>` with the actual cluster ID you copied earlier:

     ```shell
     terraform import pf9_cluster.example <cluster_id>
     ```

   - Terraform will import the existing PF9 cluster into its state. You should see a message confirming the import.

4. **Verify the Import:**
   - Run `terraform show` to view the current state of your Terraform resources.
   - Run `terraform state show pf9_cluster.example` and copy the configuration to `main.tf`.
