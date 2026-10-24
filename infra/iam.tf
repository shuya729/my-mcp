locals {
  accounts           = toset(["app-runtime", "auth-runtime", "deployer", "terraform-plan"])
  default_branch_ref = "refs/heads/${var.github_default_branch}"
  workflow_prefix    = "${var.github_repository}/.github/workflows"

  # 実行イベント・ブランチ・ワークフローファイルを識別し、plan と deploy を分離する。
  github_access = <<-CEL
    assertion.event_name == 'pull_request' && assertion.workflow_ref.startsWith('${local.workflow_prefix}/terraform-plan.yaml@refs/pull/') ? 'plan' :
    assertion.event_name == 'workflow_dispatch' && assertion.ref == '${local.default_branch_ref}' &&
    assertion.workflow_ref in ['${local.workflow_prefix}/infrastructure.yaml@${local.default_branch_ref}', '${local.workflow_prefix}/release.yaml@${local.default_branch_ref}'] ? 'deploy' : 'none'
  CEL
}

resource "google_service_account" "application" {
  for_each = local.accounts

  account_id   = each.value
  display_name = "my-mcp ${each.value}"

  depends_on = [google_project_service.required["iam.googleapis.com"]]
}

resource "google_project_iam_member" "database_client" {
  for_each = toset(["app-runtime", "auth-runtime"])

  project = var.project_id
  role    = "roles/cloudsql.client"
  member  = google_service_account.application[each.value].member
}

resource "google_secret_manager_secret_iam_member" "app_password" {
  secret_id = google_secret_manager_secret.database_password["app"].id
  role      = "roles/secretmanager.secretAccessor"
  member    = google_service_account.application["app-runtime"].member
}

resource "google_secret_manager_secret_iam_member" "logto_database_url" {
  secret_id = google_secret_manager_secret.logto_database_url.id
  role      = "roles/secretmanager.secretAccessor"
  member    = google_service_account.application["auth-runtime"].member
}

# plan 用の参照権限。別途、DB パスワード2件の参照と state・ロックの更新権限を付与する。
resource "google_project_iam_member" "plan" {
  for_each = toset(["roles/viewer", "roles/iam.securityReviewer", "roles/serviceusage.serviceUsageConsumer"])

  project = var.project_id
  role    = each.value
  member  = google_service_account.application["terraform-plan"].member
}

resource "google_secret_manager_secret_iam_member" "plan_password" {
  for_each = local.databases

  secret_id = google_secret_manager_secret.database_password[each.value].id
  role      = "roles/secretmanager.secretAccessor"
  member    = google_service_account.application["terraform-plan"].member
}

# 単一の deployer がインフラを管理するため、各プロダクトの管理権限を持つ。
# Owner / Editor は使わないが、IAM を変更できる強い権限である。
resource "google_project_iam_member" "deployer" {
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
  member  = google_service_account.application["deployer"].member
}

resource "google_service_account_iam_member" "deployer_runtime" {
  for_each = toset(["app-runtime", "auth-runtime"])

  service_account_id = google_service_account.application[each.value].name
  role               = "roles/iam.serviceAccountUser"
  member             = google_service_account.application["deployer"].member
}

# GCS backend の state 参照・更新とロックに使用。事前作成したバケット自体は管理しない。
resource "google_storage_bucket_iam_member" "state" {
  for_each = toset(["terraform-plan", "deployer"])

  bucket = var.state_bucket
  role   = "roles/storage.objectAdmin"
  member = google_service_account.application[each.value].member
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

  attribute_condition = "assertion.repository_id == '${var.github_repository_id}' && assertion.repository_owner_id == '${var.github_owner_id}' && attribute.access in ['plan', 'deploy']"

  oidc {
    issuer_uri = "https://token.actions.githubusercontent.com"
  }
}

resource "google_service_account_iam_member" "github" {
  for_each = {
    terraform-plan = "plan"
    deployer       = "deploy"
  }

  service_account_id = google_service_account.application[each.key].name
  role               = "roles/iam.workloadIdentityUser"
  member             = "principalSet://iam.googleapis.com/${google_iam_workload_identity_pool.github.name}/attribute.access/${each.value}"
}
