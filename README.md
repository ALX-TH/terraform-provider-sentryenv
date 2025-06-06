Terraform Provider Sentry Environment
=========================
The Terraform provider for Sentry Environment allows teams to create Sentry environments via their [API interface](https://docs.sentry.io/api/).  

This provider can be used together with the [jianyuan/sentry](https://github.com/jianyuan/terraform-provider-sentry) provider to create the ***sentry_issue_alert*** resource.  
The provider publishes demo events into Sentry for the specified environments, which triggers the creation of those environments.


Using the Provider
------------------
```tcl
provider "sentryenv" {}

resource "sentry_key" "key" {
  for_each = var.projects

  organization = data.sentry_organization.organization.id
  project      = try(each.value.name, each.key)
  name         = try(each.value.name, each.key)

  depends_on = [
    sentry_team.team,
    sentry_project.project
  ]
}

resource "sentryenv_environment" "environment" {
  for_each = var.projects

  auth_token   = var.sentry_auth_token
  slug         = var.slug
  dsn          = sentry_key.key[each.key].dsn_public
  project_name = sentry_key.key[each.key].name
  envs         = join(",", each.value.sentry_environments)

  depends_on = [
    sentry_team.team,
    sentry_project.project,
    sentry_key.key
  ]
}

variable "projects" {
    description = "A map of project names to their corresponding Sentry environments. Each project has a list of environment names where Sentry events should be created, such as dev, staging, and production."
    type = map(object({
        sentry_environments = list(string)
    }))
    default = {
        "native-application" = {
            sentry_environments = [
                "dev",
                "staging",
                "production"
            ]
        }
    }
}
```