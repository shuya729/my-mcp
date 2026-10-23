\set ON_ERROR_STOP on

\getenv app_db APP_DB
\getenv logto_db LOGTO_DB

CREATE DATABASE :"app_db";
CREATE DATABASE :"logto_db";
