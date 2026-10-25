ephemeral "random_password" "database" {
  for_each = local.database_users

  length  = 32
  special = false
}

resource "google_secret_manager_secret" "database_password" {
  for_each = local.database_users

  secret_id = "my-mcp-${replace(each.key, "_", "-")}-database-password"

  replication {
    auto {}
  }

  depends_on = [google_project_service.required["secretmanager.googleapis.com"]]
}

resource "google_secret_manager_secret_version" "database_password" {
  for_each = local.database_users

  secret                 = google_secret_manager_secret.database_password[each.key].id
  secret_data_wo         = ephemeral.random_password.database[each.key].result
  secret_data_wo_version = var.database_password_revision[each.key]
  deletion_policy        = "DISABLE"
}

# 生成結果を直接 SQL ユーザーへ渡さず、保存済みの値を正とする。
# ユーザーだけ再作成しても新たな乱数で Secret と不一致にならない。
ephemeral "google_secret_manager_secret_version" "database_password" {
  for_each = local.database_users

  secret  = google_secret_manager_secret.database_password[each.key].secret_id
  version = google_secret_manager_secret_version.database_password[each.key].version
}

# Logto は個別の PG 環境変数ではなく DB_URL を受け取るため、接続 URL も Secret に保存する。
resource "google_secret_manager_secret" "logto_database_url" {
  for_each = { for user, configuration in local.database_users : user => configuration if configuration.database == "logto" }

  secret_id = "my-mcp-${replace(each.key, "_", "-")}-database-url"

  replication {
    auto {}
  }

  depends_on = [google_project_service.required["secretmanager.googleapis.com"]]
}

resource "google_secret_manager_secret_version" "logto_database_url" {
  for_each = google_secret_manager_secret.logto_database_url

  secret = each.value.id
  secret_data_wo = format(
    "postgres://%s:%s@127.0.0.1:5432/logto?sslmode=disable",
    each.key,
    ephemeral.google_secret_manager_secret_version.database_password[each.key].secret_data,
  )
  secret_data_wo_version = google_secret_manager_secret_version.database_password[each.key].version
  deletion_policy        = "DISABLE"
}
