#!/bin/sh
set -eu

# AI 查询账号只允许使用便于安全转义的密码字符，避免初始化 SQL 被注入。
case "${AGENT_QUERY_DB_PASSWORD:-}" in
  *[!A-Za-z0-9._-]*|'')
    echo "AGENT_QUERY_DB_PASSWORD 必须是非空的字母、数字、点、下划线或短横线组合" >&2
    exit 1
    ;;
esac

if [ "${#AGENT_QUERY_DB_PASSWORD}" -lt 16 ]; then
  echo "AGENT_QUERY_DB_PASSWORD 长度不能少于 16 位" >&2
  exit 1
fi

case "${MYSQL_DATABASE:-}" in
  *[!A-Za-z0-9_]*|'')
    echo "MYSQL_DATABASE 只能包含字母、数字和下划线" >&2
    exit 1
    ;;
esac

mysql --protocol=socket -uroot --password="${MYSQL_ROOT_PASSWORD}" <<SQL
CREATE USER IF NOT EXISTS 'agent_reader'@'%' IDENTIFIED BY '${AGENT_QUERY_DB_PASSWORD}';
ALTER USER 'agent_reader'@'%' IDENTIFIED BY '${AGENT_QUERY_DB_PASSWORD}' WITH MAX_USER_CONNECTIONS 10;
REVOKE ALL PRIVILEGES, GRANT OPTION FROM 'agent_reader'@'%';
GRANT SELECT ON \`${MYSQL_DATABASE}\`.* TO 'agent_reader'@'%';
FLUSH PRIVILEGES;
SQL

echo "AI 数据库只读账号 agent_reader 已就绪"
