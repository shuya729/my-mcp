locals {
  services = {
    api = {
      name    = "my-mcp-api"
      port    = 8080
      memory  = "512Mi"
      maximum = 2
      account = "app-runtime"
      image   = var.initial_images.api
    }
    mcp = {
      name    = "my-mcp-mcp"
      port    = 8081
      memory  = "512Mi"
      maximum = 2
      account = "app-runtime"
      image   = var.initial_images.mcp
    }
    logto = {
      name    = "my-mcp-logto"
      port    = 3001
      memory  = "1Gi"
      maximum = 2
      account = "auth-runtime"
      image   = var.initial_images.logto
    }
    logto-admin = {
      name    = "my-mcp-logto-admin"
      port    = 3002
      memory  = "1Gi"
      maximum = 1
      account = "auth-runtime"
      image   = var.initial_images.logto
    }
  }

  # 作成前に決まる標準 URL を使用し、サービス自身の URL 参照による循環を避ける。
  service_urls = {
    for key, service in local.services : key => "https://${service.name}-${data.google_project.production.number}.${local.region}.run.app"
  }

  service_environment = {
    api = {
      API_URL       = local.service_urls.api
      LOGTO_URL     = local.service_urls.logto
      POSTGRES_USER = "app"
      APP_DB        = "app"
    }
    mcp = {
      MCP_URL       = local.service_urls.mcp
      LOGTO_URL     = local.service_urls.logto
      POSTGRES_USER = "app"
      APP_DB        = "app"
    }
    logto = {
      ENDPOINT                = local.service_urls.logto
      ADMIN_DISABLE_LOCALHOST = "1"
      TRUST_PROXY_HEADER      = "1"
    }
    logto-admin = {
      ENDPOINT                = local.service_urls.logto
      ADMIN_ENDPOINT          = local.service_urls["logto-admin"]
      ADMIN_DISABLE_LOCALHOST = "1"
      TRUST_PROXY_HEADER      = "1"
    }
  }
}

resource "google_cloud_run_v2_service" "application" {
  for_each = local.services

  name                 = each.value.name
  location             = local.region
  ingress              = "INGRESS_TRAFFIC_ALL"
  iap_enabled          = each.key == "logto-admin"
  invoker_iam_disabled = each.key != "logto-admin"

  scaling {
    max_instance_count = each.value.maximum
  }

  template {
    service_account       = google_service_account.application[each.value.account].email
    execution_environment = "EXECUTION_ENVIRONMENT_GEN2"

    vpc_access {
      network_interfaces {
        subnetwork = google_compute_subnetwork.production.name
      }
    }

    containers {
      name       = "application"
      image      = each.value.image
      depends_on = ["cloud-sql-proxy"]

      ports {
        container_port = each.value.port
      }

      resources {
        limits = {
          cpu    = "1"
          memory = each.value.memory
        }
        cpu_idle = true
      }

      dynamic "env" {
        for_each = local.service_environment[each.key]
        content {
          name  = env.key
          value = env.value
        }
      }

      env {
        name = each.value.account == "app-runtime" ? "POSTGRES_PASSWORD" : "DB_URL"
        value_source {
          secret_key_ref {
            secret  = each.value.account == "app-runtime" ? google_secret_manager_secret.database_password["app"].secret_id : google_secret_manager_secret.logto_database_url["logto"].secret_id
            version = each.value.account == "app-runtime" ? google_secret_manager_secret_version.database_password["app"].version : google_secret_manager_secret_version.logto_database_url["logto"].version
          }
        }
      }
    }

    containers {
      name  = "cloud-sql-proxy"
      image = var.sql_proxy_image
      args  = local.proxy_arguments

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

  lifecycle {
    ignore_changes = [template[0].containers[0].image]

    precondition {
      condition     = can(regex("@sha256:[0-9a-f]{64}$", each.value.image)) && can(regex("^gcr.io/cloud-sql-connectors/cloud-sql-proxy@sha256:[0-9a-f]{64}$", var.sql_proxy_image))
      error_message = "initial_images と sql_proxy_image に、ビルド・確認済みの digest を指定してください。"
    }
    precondition {
      condition     = each.key != "logto-admin" || length(var.iap_admin_emails) > 0
      error_message = "管理用サービスを作成する前に iap_admin_emails を指定してください。"
    }
  }

  depends_on = [
    google_project_service.required["run.googleapis.com"],
    google_project_service.required["iap.googleapis.com"],
    google_dns_record_set.database,
    google_compute_forwarding_rule.database,
    google_sql_user.application,
    google_project_iam_member.database_client,
    google_secret_manager_secret_iam_member.runtime_database,
  ]
}

resource "google_cloud_run_v2_service_iam_member" "iap_invoker" {
  name     = google_cloud_run_v2_service.application["logto-admin"].name
  location = local.region
  role     = "roles/run.invoker"
  # Google 管理のサービスエージェントは初回準備で生成する。
  member = "serviceAccount:service-${data.google_project.production.number}@gcp-sa-iap.iam.gserviceaccount.com"
}

resource "google_iap_web_cloud_run_service_iam_binding" "administrators" {
  project                = var.project_id
  location               = local.region
  cloud_run_service_name = google_cloud_run_v2_service.application["logto-admin"].name
  role                   = "roles/iap.httpsResourceAccessor"
  members                = [for email in var.iap_admin_emails : "user:${email}"]
}
