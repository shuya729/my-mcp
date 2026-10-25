# GCP 構築・運用

東京の Cloud Run（API・MCP・Logto 認証・Logto 管理）と、PSC 経由の Cloud SQL PostgreSQL 18 を使用します。公開 URL は `run.app`。Logto 管理画面は [IAP の Google 管理 OAuth クライアント](https://docs.cloud.google.com/run/docs/securing/identity-aware-proxy-cloud-run)を使うため、プロジェクトと同じ Google Cloud 組織のアカウントが必要です。
Terraform は構成、GitHub Actions はイメージ更新を管理します。

## 初回構築

Terraform 1.12.2（Actions と同じバージョン）、gcloud、Docker Buildx が必要です。以下はリポジトリのルートで実行します。

1. 組織配下に課金を有効にした専用プロジェクトと、非公開の state 用 GCS バケットを準備します。バケットは Uniform bucket-level access と [Object Versioning](https://developer.hashicorp.com/terraform/language/backend/gcs) を有効にします。実行者にはリソース・API・IAM の管理権限（[iam.tf](../infra/iam.tf) の `infrastructure`・`runtime_user` を参照）と、state バケットの管理権限が必要です。以下の `PROJECT_ID` は実値に置き換えます。

```sh
gcloud auth login
GCP_PROJECT_ID=PROJECT_ID
gcloud config set project "$GCP_PROJECT_ID"
gcloud services enable serviceusage.googleapis.com cloudresourcemanager.googleapis.com
gcloud auth application-default login
gcloud auth application-default set-quota-project "$GCP_PROJECT_ID"
```

2. `infra/terraform.tfvars.example`・`infra/backend.hcl.example` を拡張子 `.example` を外してコピーし、`TODO` の実情報を記入します。イメージは後で設定します。`iap_admin_emails` にはプロジェクトと同じ組織の Google アカウントを1件以上指定します。秘密値は記載しません。
3. 初回だけ対象を限定して基盤を作ります。通常の反映では `-target` を使いません。

```sh
terraform -chdir=infra init -backend-config=backend.hcl
terraform -chdir=infra apply \
  -target=google_project_service.required \
  -target=google_sql_database.application \
  -target=google_compute_forwarding_rule.database \
  -target=google_dns_record_set.database \
  -target=google_artifact_registry_repository.application \
  -target=google_iam_workload_identity_pool_provider.github \
  -target=google_service_account_iam_member.github \
  -target=google_project_iam_member.infrastructure \
  -target=google_service_account_iam_member.runtime_user \
  -target=google_artifact_registry_repository_iam_member.release \
  -target=google_storage_bucket_iam_member.infrastructure_state \
  -target=google_storage_bucket_iam_member.plan_state \
  -target=google_storage_bucket_iam_member.plan_lock
```

4. Cloud SQL コンソールで `my-mcp-production` の `postgres` のパスワードを設定します。[Cloud SQL Studio](https://docs.cloud.google.com/sql/docs/postgres/manage-data-using-studio) に `postgres` ユーザー・`postgres` DB で接続し、[roles.sql](../infra/sql/roles.sql) を実行します。実行者には `roles/cloudsql.studioUser` 相当の権限も必要です。

5. GitHub Variables を登録します。

| 名前               | 値                                                                     |
| ------------------ | ---------------------------------------------------------------------- |
| `GCP_PROJECT_ID`   | GCP プロジェクト ID                                                    |
| `TF_STATE_BUCKET`  | state 用 GCS バケット名                                                |
| `WIF_PROVIDER`     | `terraform -chdir=infra output -raw workload_identity_provider` の出力 |
| `SUBNET_CIDR`      | `terraform.tfvars` と同じ CIDR                                         |
| `INITIAL_IMAGES`   | `initial_images` と同じ JSON オブジェクト（ビルド後に登録）            |
| `IAP_ADMIN_EMAILS` | `iap_admin_emails` と同じ JSON 配列（1件以上必須）                     |

GitHub リポジトリ情報は Actions から取得します。本番反映は `main` に固定しています。

初回のイメージは手元からビルド・登録します。

```sh
gcloud auth configure-docker asia-northeast1-docker.pkg.dev --quiet
for component in api mcp migration; do
  docker buildx build --platform linux/amd64 --target "$component" \
    --tag "asia-northeast1-docker.pkg.dev/$GCP_PROJECT_ID/my-mcp/$component:initial" --push .
  docker buildx imagetools inspect "asia-northeast1-docker.pkg.dev/$GCP_PROJECT_ID/my-mcp/$component:initial"
done
```

表示された digest を使い、手元の `initial_images` と GitHub Variables の `INITIAL_IMAGES` を設定します。値は `IMAGE@sha256:…` 形式です。`logto` には、[release.yaml](../.github/workflows/release.yaml) の `LOGTO_VERSION` に対応する公式イメージの digest を指定します。以下の `VERSION` はその値に置き換えます。

```sh
docker buildx imagetools inspect docker.io/svhd/logto:VERSION
```

Auth Proxy の既定イメージは `infra/variables.tf` を参照してください。

6. Job と DB ユーザーを作成します。

```sh
terraform -chdir=infra apply \
  -target=google_cloud_run_v2_job.maintenance \
  -target=google_secret_manager_secret_iam_member.plan_password
```

Studio に `postgres` ユーザーで接続し、`app` DB では [app-access.sql](../infra/sql/app-access.sql)、`logto` DB では [logto-before-seed.sql](../infra/sql/logto-before-seed.sql) を実行します。その後 Logto を初期化します。

```sh
gcloud run jobs execute my-mcp-logto-maintenance --project="$GCP_PROJECT_ID" --region=asia-northeast1 \
  --wait --container=application \
  --args="trap 'wget -q -T 5 -O /dev/null http://127.0.0.1:9091/quitquitquit' EXIT; npm run cli db seed -- --swe"
```

成功後、`logto` で [logto-after-seed.sql](../infra/sql/logto-after-seed.sql) を実行します。通常サービスには所有者権限を渡さず、テナント情報と起動用設定の参照だけを許可します。Logto 更新時は、この権限設定と RLS が新バージョンに対応することを確認してください。

7. アプリ DB を更新し、Google 管理の IAP サービスエージェントを生成してからサービスを作成します。

```sh
gcloud run jobs execute my-mcp-migrate --project="$GCP_PROJECT_ID" --region=asia-northeast1 --wait
gcloud beta services identity create --service=iap.googleapis.com --project="$GCP_PROJECT_ID"
terraform -chdir=infra apply
```

8. `terraform -chdir=infra output service_urls` の `logto-admin` を許可済みアカウントで開き、Logto の管理者を作成します。[config.yaml](../logto/config.yaml) は自動適用されないため、`${API_URL}`・`${MCP_URL}` を出力の `api`・`mcp` に置き換えて手動設定します。リソース識別子はルート URL、MCP の接続先はその URL に `/mcp` を付けたものです。メールは開発用 `mockEmail` の代わりに [Resend SMTP](https://resend.com/docs/send-with-smtp) を設定します。スケールゼロからの起動、メール OTP、認証後の API・MCP 操作、許可外アカウントの管理画面拒否を確認します。

## 通常運用

- PR：Go の CI と、インフラ・関連ワークフロー変更時の Terraform plan（対象は [terraform-plan.yaml](../.github/workflows/terraform-plan.yaml)）。
- `Infrastructure`：手動で Terraform plan・apply。初期化 Job は実行しません。
- `Release`：入力なしで手動実行し、アプリをビルド後、アプリ・Logto 両方の DB 更新と全サービスの配置を行います。Logto のバージョンは `.github/workflows/release.yaml` の `LOGTO_VERSION` で指定します。

DB パスワードは Secret Manager で管理します。更新時は `infra/variables.tf` の `database_password_revision.default` で対象ユーザーの番号を増やし、`main` に反映して Infrastructure を実行します。手元の `terraform.tfvars` では上書きしません。
サービス・Job の再作成前には、手元の `initial_images` と GitHub Variables の `INITIAL_IMAGES` を配置したい digest に更新してください。既存サービス・Job のアプリイメージは Terraform では更新しません。
