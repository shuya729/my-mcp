resource "google_compute_network" "production" {
  name                    = "my-mcp-production"
  auto_create_subnetworks = false

  depends_on = [google_project_service.required["compute.googleapis.com"]]
}

resource "google_compute_subnetwork" "production" {
  name          = "my-mcp-production"
  network       = google_compute_network.production.id
  ip_cidr_range = var.subnet_cidr
}

# Cloud SQL の自動エンドポイント作成は使わず、IP と接続口を明示的に管理する。
resource "google_compute_address" "database" {
  name         = "my-mcp-database"
  address_type = "INTERNAL"
  subnetwork   = google_compute_subnetwork.production.id
}

resource "google_compute_forwarding_rule" "database" {
  name                  = "my-mcp-database"
  network               = google_compute_network.production.id
  ip_address            = google_compute_address.database.id
  target                = google_sql_database_instance.production.psc_service_attachment_link
  load_balancing_scheme = ""
}

# Auth Proxy が使用する Cloud SQL 提供の DNS 名を、同一 VPC 内で PSC IP に解決する。
resource "google_dns_managed_zone" "database" {
  name       = "my-mcp-database"
  dns_name   = google_sql_database_instance.production.dns_name
  visibility = "private"

  private_visibility_config {
    networks {
      network_url = google_compute_network.production.id
    }
  }

  depends_on = [google_project_service.required["dns.googleapis.com"]]
}

resource "google_dns_record_set" "database" {
  managed_zone = google_dns_managed_zone.database.name
  name         = google_dns_managed_zone.database.dns_name
  type         = "A"
  ttl          = 300
  rrdatas      = [google_compute_address.database.address]
}
