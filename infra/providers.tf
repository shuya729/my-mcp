provider "google" {
  project = var.project_id
  region  = local.region

  user_project_override = true
  billing_project       = var.project_id
}
