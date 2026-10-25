locals {
  runtime_accounts = {
    app-runtime      = "app"
    app-migration    = "app_migration"
    auth-runtime     = "logto"
    auth-maintenance = "logto_maintenance"
  }
  accounts = setunion(
    keys(local.runtime_accounts),
    ["terraform-apply", "release"],
  )
  default_branch_ref = "refs/heads/main"
  workflow_prefix    = "${var.github_repository}/.github/workflows"

  # 各ワークフローは自身のサービスアカウントだけを利用できる。
  github_access = <<-CEL
    assertion.event_name == 'workflow_dispatch' && assertion.ref == '${local.default_branch_ref}' && assertion.workflow_ref == '${local.workflow_prefix}/infrastructure.yaml@${local.default_branch_ref}' ? 'infrastructure' :
    assertion.event_name == 'workflow_dispatch' && assertion.ref == '${local.default_branch_ref}' && assertion.workflow_ref == '${local.workflow_prefix}/release.yaml@${local.default_branch_ref}' ? 'release' : 'none'
  CEL
}

resource "google_service_account" "application" {
  for_each = local.accounts

  account_id   = each.value
  display_name = "my-mcp ${each.value}"

  depends_on = [google_project_service.required["iam.googleapis.com"]]
}

resource "google_project_iam_member" "database_client" {
  for_each = local.runtime_accounts

  project = var.project_id
  role    = "roles/cloudsql.client"
  member  = google_service_account.application[each.key].member

  condition {
    title      = "production_database"
    expression = "resource.name == 'projects/${var.project_id}/instances/${google_sql_database_instance.production.name}' && resource.type == 'sqladmin.googleapis.com/Instance'"
  }
}

# 各サービス・Job は自身の資格情報だけを読み取る。
resource "google_secret_manager_secret_iam_member" "runtime_database" {
  for_each = local.runtime_accounts

  secret_id = local.database_users[each.value].database == "logto" ? google_secret_manager_secret.logto_database_url[each.value].id : google_secret_manager_secret.database_password[each.value].id
  role      = "roles/secretmanager.secretAccessor"
  member    = google_service_account.application[each.key].member
}

# インフラと IAM の変更は Infrastructure ワークフローだけが行う。
resource "google_project_iam_member" "infrastructure" {
  for_each = toset([
    "roles/artifactregistry.admin",
    "roles/cloudsql.admin",
    "roles/compute.networkAdmin",
    "roles/dns.admin",
    "roles/iam.serviceAccountAdmin",
    "roles/iam.workloadIdentityPoolAdmin",
    "roles/iap.admin",
    "roles/resourcemanager.projectIamAdmin",
    "roles/run.admin",
    "roles/secretmanager.admin",
    "roles/serviceusage.serviceUsageAdmin",
  ])

  project = var.project_id
  role    = each.value
  member  = google_service_account.application["terraform-apply"].member
}

resource "google_service_account_iam_member" "runtime_user" {
  for_each = {
    for pair in setproduct(toset(["terraform-apply", "release"]), toset(keys(local.runtime_accounts))) :
    "${pair[0]}/${pair[1]}" => { caller = pair[0], runtime = pair[1] }
  }

  service_account_id = google_service_account.application[each.value.runtime].name
  role               = "roles/iam.serviceAccountUser"
  member             = google_service_account.application[each.value.caller].member
}

resource "google_artifact_registry_repository_iam_member" "release" {
  location   = google_artifact_registry_repository.application.location
  repository = google_artifact_registry_repository.application.name
  role       = "roles/artifactregistry.writer"
  member     = google_service_account.application["release"].member
}

resource "google_cloud_run_v2_service_iam_member" "release" {
  for_each = local.services

  name     = google_cloud_run_v2_service.application[each.key].name
  location = local.region
  role     = "roles/run.developer"
  member   = google_service_account.application["release"].member
}

resource "google_cloud_run_v2_job_iam_member" "release" {
  for_each = local.jobs

  name     = google_cloud_run_v2_job.maintenance[each.key].name
  location = local.region
  role     = "roles/run.developer"
  member   = google_service_account.application["release"].member
}

# state と、そのバケットの IAM を管理する。バケット自体は事前作成する。
resource "google_storage_bucket_iam_member" "infrastructure_state" {
  bucket = var.state_bucket
  role   = "roles/storage.admin"
  member = google_service_account.application["terraform-apply"].member
}

resource "google_iam_workload_identity_pool" "github" {
  workload_identity_pool_id = "github-actions"
  display_name              = "GitHub Actions"

  depends_on = [google_project_service.required["iam.googleapis.com"]]
}

resource "google_iam_workload_identity_pool_provider" "github" {
  workload_identity_pool_id          = google_iam_workload_identity_pool.github.workload_identity_pool_id
  workload_identity_pool_provider_id = "github"

  attribute_mapping = {
    "google.subject"   = "assertion.sub"
    "attribute.access" = local.github_access
  }

  attribute_condition = "assertion.repository_id == '${var.github_repository_id}' && assertion.repository_owner_id == '${var.github_owner_id}' && attribute.access in ['infrastructure', 'release']"

  oidc {
    issuer_uri = "https://token.actions.githubusercontent.com"
  }
}

resource "google_service_account_iam_member" "github" {
  for_each = {
    terraform-apply = "infrastructure"
    release         = "release"
  }

  service_account_id = google_service_account.application[each.key].name
  role               = "roles/iam.workloadIdentityUser"
  member             = "principalSet://iam.googleapis.com/${google_iam_workload_identity_pool.github.name}/attribute.access/${each.value}"
}
