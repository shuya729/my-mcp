-- DB 作成後、ログインユーザー作成前に Cloud SQL Studio の postgres で実行する。
CREATE ROLE app_runtime;
CREATE ROLE app_owner;
CREATE ROLE logto_runtime;
CREATE ROLE logto_owner;
GRANT app_owner, logto_owner TO postgres;

ALTER DATABASE app OWNER TO app_owner;
ALTER DATABASE logto OWNER TO logto_owner;
REVOKE ALL ON DATABASE app, logto FROM PUBLIC;
GRANT CONNECT ON DATABASE app TO app_runtime, app_owner;
GRANT CONNECT ON DATABASE logto TO logto_runtime, logto_owner;
