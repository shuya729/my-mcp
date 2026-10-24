locals {
  databases = toset(["app", "logto"])

  proxy_arguments = [
    "--psc",
    "--port=5432",
    "--lazy-refresh",
    "--structured-logs",
    "--health-check",
    "--http-address=0.0.0.0",
    google_sql_database_instance.production.connection_name,
  ]
}

resource "google_sql_database_instance" "production" {
  name             = "my-mcp-production"
  database_version = "POSTGRES_18"

  settings {
    edition   = "ENTERPRISE"
    tier      = "db-custom-1-3840"
    disk_size = 10

    ip_configuration {
      ipv4_enabled = false

      psc_config {
        psc_enabled               = true
        allowed_consumer_projects = [var.project_id]
      }
    }
  }

  depends_on = [google_project_service.required["sqladmin.googleapis.com"]]
}

resource "google_sql_database" "application" {
  for_each = local.databases

  name     = each.value
  instance = google_sql_database_instance.production.name
}

# 初回に管理者が app_owner / logto_owner を手動作成する。
# カスタムロールを指定し、既定の cloudsqlsuperuser を付与しない。
resource "google_sql_user" "application" {
  for_each = local.databases

  name                = each.value
  instance            = google_sql_database_instance.production.name
  database_roles      = ["${each.value}_owner"]
  password_wo         = ephemeral.google_secret_manager_secret_version.database_password[each.value].secret_data
  password_wo_version = tonumber(google_secret_manager_secret_version.database_password[each.value].version)
}
