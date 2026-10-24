resource "google_artifact_registry_repository" "application" {
  repository_id = "my-mcp"
  format        = "DOCKER"

  depends_on = [google_project_service.required["artifactregistry.googleapis.com"]]
}
