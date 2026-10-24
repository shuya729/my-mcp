variable "project_id" {
  description = "手動で作成・課金を有効化した本番用 GCP プロジェクト ID。"
  type        = string
  nullable    = false

  validation {
    condition     = can(regex("^[a-z][a-z0-9-]{4,28}[a-z0-9]$", var.project_id))
    error_message = "project_id に実際の GCP プロジェクト ID を指定してください。"
  }
}

variable "state_bucket" {
  description = "backend.hcl と同じ、手動で作成済みの非公開 GCS バケット名。"
  type        = string
  nullable    = false

  validation {
    condition     = length(var.state_bucket) >= 3
    error_message = "state_bucket に既存の state 用 GCS バケット名を指定してください。"
  }
}

variable "github_repository" {
  description = "Actions を実行する GitHub リポジトリ（OWNER/REPO）。"
  type        = string
  nullable    = false

  validation {
    condition     = can(regex("^[A-Za-z0-9_.-]+/[A-Za-z0-9_.-]+$", var.github_repository))
    error_message = "github_repository は OWNER/REPO 形式で指定してください。"
  }
}

variable "github_repository_id" {
  description = "リポジトリ名の再取得によるなりすましを防ぐための GitHub 数値リポジトリ ID。"
  type        = string
  nullable    = false

  validation {
    condition     = can(regex("^[0-9]+$", var.github_repository_id))
    error_message = "github_repository_id に数値 ID を文字列で指定してください。"
  }
}

variable "github_owner_id" {
  description = "GitHub リポジトリ所有者の数値 ID。"
  type        = string
  nullable    = false

  validation {
    condition     = can(regex("^[0-9]+$", var.github_owner_id))
    error_message = "github_owner_id に数値 ID を文字列で指定してください。"
  }
}

variable "github_default_branch" {
  description = "手動で本番反映を許可するデフォルトブランチ。"
  type        = string
  nullable    = false

  validation {
    condition     = can(regex("^[A-Za-z0-9_./-]+$", var.github_default_branch))
    error_message = "github_default_branch に実際のデフォルトブランチを指定してください。"
  }
}

variable "database_password_revision" {
  description = "パスワード更新時だけ増やす番号。通常の apply では変更しない。"
  type = object({
    app   = number
    logto = number
  })
  default = {
    app   = 1
    logto = 1
  }

  validation {
    condition = alltrue([
      for revision in values(var.database_password_revision) : revision >= 1 && floor(revision) == revision
    ])
    error_message = "各パスワードの revision は 1 以上の整数にしてください。"
  }
}

variable "initial_images" {
  description = "新規作成・再作成に使うイメージ digest。通常の更新は release.yaml が担当する。"
  type = object({
    api       = string
    mcp       = string
    migration = string
    logto     = string
  })
  default = {
    api       = ""
    mcp       = ""
    migration = ""
    # 既存の compose.yaml と同じ Logto 1.43.0。
    logto = "docker.io/svhd/logto@sha256:41196a1fadbff35eae337965321c57946936755066bd92625ccf8a5b65d4b1be"
  }
}

variable "sql_proxy_image" {
  description = "公式 Cloud SQL Auth Proxy のバージョンを固定した digest。"
  type        = string
  # Cloud SQL Auth Proxy 2.26.3（公式マルチアーキテクチャマニフェスト）。
  default = "gcr.io/cloud-sql-connectors/cloud-sql-proxy@sha256:f333a9c59890c3c67b8505166c5dbdcc0b29fca9173a2e09ca0446ab6997a736"
}

variable "iap_admin_emails" {
  description = "Logto 管理画面へのアクセスを許可する Google アカウント。"
  type        = set(string)
  default     = []

  validation {
    condition     = alltrue([for email in var.iap_admin_emails : can(regex("^[^@[:space:]]+@[^@[:space:]]+\\.[^@[:space:]]+$", email))])
    error_message = "iap_admin_emails に実際の Google アカウントのメールアドレスを指定してください。"
  }
}

variable "subnet_cidr" {
  description = "Cloud Run Direct VPC egress と Cloud SQL PSC 用の、重複しない RFC 1918 /24。"
  type        = string
  nullable    = false

  validation {
    condition = can(cidrhost(var.subnet_cidr, 0)) && can(regex("/24$", var.subnet_cidr)) && (
      can(regex("^10\\.", var.subnet_cidr)) ||
      can(regex("^172\\.(1[6-9]|2[0-9]|3[01])\\.", var.subnet_cidr)) ||
      can(regex("^192\\.168\\.", var.subnet_cidr))
    ) && try(cidrhost(var.subnet_cidr, 0) == split("/", var.subnet_cidr)[0], false)
    error_message = "subnet_cidr にはネットワークアドレスから始まる RFC 1918 の /24 を指定してください。"
  }
}
