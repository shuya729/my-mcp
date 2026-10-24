ephemeral "random_password" "database" {
  for_each = local.databases

  length  = 32
  special = false
}

resource "google_secret_manager_secret" "database_password" {
  for_each = local.databases

  secret_id = "my-mcp-${each.value}-database-password"

  replication {
    auto {}
  }

  depends_on = [google_project_service.required["secretmanager.googleapis.com"]]
}

resource "google_secret_manager_secret_version" "database_password" {
  for_each = local.databases

  secret                 = google_secret_manager_secret.database_password[each.value].id
  secret_data_wo         = ephemeral.random_password.database[each.value].result
  secret_data_wo_version = var.database_password_revision[each.value]
  deletion_policy        = "DISABLE"
}

# 生成結果を直接 SQL ユーザーへ渡さず、保存済みの値を正とする。
# ユーザーだけ再作成しても新たな乱数で Secret と不一致にならない。
ephemeral "google_secret_manager_secret_version" "database_password" {
  for_each = local.databases

  secret  = google_secret_manager_secret.database_password[each.value].secret_id
  version = google_secret_manager_secret_version.database_password[each.value].version
}

# Logto は個別の PG 環境変数ではなく DB_URL を受け取るため、接続 URL も Secret に保存する。
resource "google_secret_manager_secret" "logto_database_url" {
  secret_id = "my-mcp-logto-database-url"

  replication {
    auto {}
  }

  depends_on = [google_project_service.required["secretmanager.googleapis.com"]]
}

resource "google_secret_manager_secret_version" "logto_database_url" {
  secret = google_secret_manager_secret.logto_database_url.id
  secret_data_wo = format(
    "postgres://logto:%s@127.0.0.1:5432/logto?sslmode=disable",
    ephemeral.google_secret_manager_secret_version.database_password["logto"].secret_data,
  )
  secret_data_wo_version = tonumber(google_secret_manager_secret_version.database_password["logto"].version)
  deletion_policy        = "DISABLE"
}
