#!/bin/sh
# Docker siz lokal PostgreSQL klasteri (foydalanuvchi papkasida, port 5433).
# Talab: tizimda PostgreSQL o'rnatilgan bo'lsin (initdb, pg_ctl). PostGIS kerak emas.
set -e
PGBIN=${PGBIN:-$(ls -d /usr/lib/postgresql/*/bin 2>/dev/null | sort -V | tail -1)}
DATA=${YOLDOSH_PGDATA:-$HOME/.local/share/yoldosh}
case "$1" in
  start)
    if [ ! -d "$DATA/pg" ]; then
      mkdir -p "$DATA"
      "$PGBIN/initdb" -D "$DATA/pg" -U yoldosh --auth=trust -E UTF8 >/dev/null
      NEW=1
    fi
    "$PGBIN/pg_ctl" -D "$DATA/pg" -o "-p 5433 -k $DATA -c listen_addresses=localhost" -l "$DATA/pg.log" status >/dev/null 2>&1 \
      || "$PGBIN/pg_ctl" -D "$DATA/pg" -o "-p 5433 -k $DATA -c listen_addresses=localhost" -l "$DATA/pg.log" -w start
    if [ -n "$NEW" ]; then
      "$PGBIN/createdb" -h localhost -p 5433 -U yoldosh yoldosh
      "$PGBIN/createdb" -h localhost -p 5433 -U yoldosh yoldosh_test
    fi
    echo "PostgreSQL: localhost:5433 (user yoldosh)";;
  stop)
    "$PGBIN/pg_ctl" -D "$DATA/pg" stop;;
  *) echo "usage: $0 start|stop"; exit 1;;
esac
