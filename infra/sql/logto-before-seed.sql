-- ユーザー作成後、logto DB に postgres で接続して実行する。
ALTER ROLE logto NOCREATEDB NOCREATEROLE;
ALTER ROLE logto_maintenance NOCREATEDB NOCREATEROLE;
ALTER ROLE logto_maintenance IN DATABASE logto SET role = 'logto_owner';
ALTER SCHEMA public OWNER TO logto_owner;
REVOKE ALL ON SCHEMA public FROM PUBLIC;

-- seed はテナント用 DB ロールを作るため、初回だけ許可する。
ALTER ROLE logto_owner CREATEROLE;
