-- ユーザー作成後、app DB に postgres で接続して実行する。
ALTER ROLE app NOCREATEDB NOCREATEROLE;
ALTER ROLE app_migration NOCREATEDB NOCREATEROLE;
ALTER ROLE app_migration IN DATABASE app SET role = 'app_owner';

ALTER SCHEMA public OWNER TO app_owner;
REVOKE ALL ON SCHEMA public FROM PUBLIC;
GRANT USAGE ON SCHEMA public TO app_runtime;
ALTER DEFAULT PRIVILEGES FOR ROLE app_owner IN SCHEMA public
  GRANT SELECT, INSERT, UPDATE, DELETE ON TABLES TO app_runtime;
ALTER DEFAULT PRIVILEGES FOR ROLE app_owner IN SCHEMA public
  GRANT USAGE ON SEQUENCES TO app_runtime;

-- migration 履歴を通常サービスから読み書きできないスキーマへ分離する。
CREATE SCHEMA migration AUTHORIZATION app_owner;
