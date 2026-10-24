locals {
  jobs = {
    migration = {
      name    = "my-mcp-migrate"
      image   = var.initial_images.migration
      account = "app-migration"
    }
    logto = {
      name    = "my-mcp-logto-maintenance"
      image   = var.initial_images.logto
      account = "auth-maintenance"
    }
  }
}

resource "google_cloud_run_v2_job" "maintenance" {
  for_each = local.jobs

  name     = each.value.name
  location = local.region

  template {
    template {
      service_account = google_service_account.application[each.value.account].email
      max_retries     = 0

      vpc_access {
        network_interfaces {
          subnetwork = google_compute_subnetwork.production.name
        }
      }

      containers {
        name       = "application"
        image      = each.value.image
        depends_on = ["cloud-sql-proxy"]
        command    = each.key == "logto" ? ["/bin/sh", "-ec"] : null
        # 初回 seed は実行時に指定する。通常の Job 定義には alteration だけを残す。
        args = each.key == "logto" ? [<<-SH
          trap 'wget -q -T 5 -O /dev/null http://127.0.0.1:9091/quitquitquit' EXIT
          npm run alteration deploy latest
        SH
        ] : null

        resources {
          limits = {
            cpu    = "1"
            memory = each.key == "logto" ? "1Gi" : "512Mi"
          }
        }

        dynamic "env" {
          for_each = each.key == "migration" ? {
            GOOSE_TABLE = "migration.goose_db_version"
            PGHOST      = "127.0.0.1"
            PGUSER      = "app_migration"
            PGDATABASE  = "app"
            PGSSLMODE   = "disable"
          } : { CI = "true" }
          content {
            name  = env.key
            value = env.value
          }
        }

        env {
          name = each.key == "migration" ? "PGPASSWORD" : "DB_URL"
          value_source {
            secret_key_ref {
              secret  = each.key == "migration" ? google_secret_manager_secret.database_password["app_migration"].secret_id : google_secret_manager_secret.logto_database_url["logto_maintenance"].secret_id
              version = each.key == "migration" ? google_secret_manager_secret_version.database_password["app_migration"].version : google_secret_manager_secret_version.logto_database_url["logto_maintenance"].version
            }
          }
        }
      }

      containers {
        name  = "cloud-sql-proxy"
        image = var.sql_proxy_image
        args  = concat(local.proxy_arguments, ["--quitquitquit"])

        startup_probe {
          period_seconds    = 5
          timeout_seconds   = 5
          failure_threshold = 36
          http_get {
            path = "/readiness"
            port = 9090
          }
        }
      }
    }
  }

  lifecycle {
    ignore_changes = [template[0].template[0].containers[0].image]

    precondition {
      condition     = can(regex("@sha256:[0-9a-f]{64}$", each.value.image)) && can(regex("^gcr.io/cloud-sql-connectors/cloud-sql-proxy@sha256:[0-9a-f]{64}$", var.sql_proxy_image))
      error_message = "Job の作成前に initial_images と sql_proxy_image の digest を指定してください。"
    }
  }

  depends_on = [
    google_project_service.required["run.googleapis.com"],
    google_dns_record_set.database,
    google_compute_forwarding_rule.database,
    google_sql_user.application,
    google_project_iam_member.database_client,
    google_secret_manager_secret_iam_member.runtime_database,
  ]
}
