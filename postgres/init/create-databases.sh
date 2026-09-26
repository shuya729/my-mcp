#!/bin/sh
set -eu

psql \
  --username "$POSTGRES_USER" \
  --dbname postgres \
  --no-password \
  --set app_db="$APP_DB" \
  --set logto_db="$LOGTO_DB" \
  --set ON_ERROR_STOP=1 <<-'EOSQL'
	CREATE DATABASE :"app_db";
	CREATE DATABASE :"logto_db";
EOSQL
