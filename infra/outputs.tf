output "artifact_registry" {
  description = "ビルド済みイメージの push 先。"
  value       = "${local.region}-docker.pkg.dev/${var.project_id}/${google_artifact_registry_repository.application.repository_id}"
}

output "database_connection_name" {
  description = "Cloud SQL Auth Proxy に渡す接続名。"
  value       = google_sql_database_instance.production.connection_name
}

output "database_dns_name" {
  description = "Cloud SQL の接続用 DNS 名。"
  value       = google_sql_database_instance.production.dns_name
}

output "database_psc_ip" {
  description = "Cloud SQL の PSC エンドポイント IP。"
  value       = google_compute_address.database.address
}

output "workload_identity_provider" {
  description = "GitHub Actions の WIF_PROVIDER に設定するリソース名。"
  value       = google_iam_workload_identity_pool_provider.github.name
}

output "actions_service_accounts" {
  description = "GitHub Actions が使用するサービスアカウント。"
  value = {
    for name in ["terraform-plan", "terraform-apply", "release"] : name => google_service_account.application[name].email
  }
}

output "service_urls" {
  description = "各サービスの公開 URL。"
  value       = local.service_urls
}

output "maintenance_jobs" {
  description = "DB 更新に使用する Cloud Run Job 名。"
  value       = { for key, job in google_cloud_run_v2_job.maintenance : key => job.name }
}
