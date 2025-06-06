---
# generated by https://github.com/hashicorp/terraform-plugin-docs
page_title: "sentryenv_environment Resource - terraform-provider-sentryenv"
subcategory: ""
description: |-
  
---

# sentryenv_environment (Resource)



## Example Usage

```terraform
resource "sentryenv_environment" "environment" {
  for_each = var.projects

  auth_token   = var.sentry_auth_token
  slug         = var.slug
  dsn          = sentry_key.key[each.key].dsn_public
  project_name = sentry_key.key[each.key].name
  envs         = join(",", each.value.sentry_environments)
}
```

<!-- schema generated by tfplugindocs -->
## Schema

### Required

- `auth_token` (String, Sensitive)
- `dsn` (String)
- `envs` (String)
- `project_name` (String)
- `slug` (String)

### Read-Only

- `hash` (String)
- `id` (String) The ID of this resource.
