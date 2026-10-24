-- seed 成功後、logto DB に postgres で接続して実行する。
ALTER ROLE logto_owner NOCREATEROLE;
GRANT CONNECT ON DATABASE logto TO logto_tenant_logto;
GRANT USAGE ON SCHEMA public TO logto_runtime, logto_tenant_logto;

-- Logto 1.43.0 の sharedPool が起動確認・テナント接続・管理署名鍵取得に使う参照だけを許可する。
-- https://github.com/logto-io/logto/blob/v1.43.0/packages/core/src/tenants/utils.ts
-- 通常の CRUD は、Logto が作成したテナントユーザーと既存の RLS を使用する。
GRANT SELECT ON systems TO logto_runtime;
GRANT SELECT (id, db_user, db_user_password) ON tenants TO logto_runtime;
GRANT SELECT (tenant_id, key, value) ON logto_configs TO logto_runtime;
CREATE POLICY tenants_runtime_lookup ON tenants
  FOR SELECT TO logto_runtime USING (true);
-- テナント制約はテナントロールへ適用し、資格情報取得用の sharedPool と分離する。
ALTER POLICY logto_configs_tenant_id ON logto_configs TO logto_tenant_logto;
