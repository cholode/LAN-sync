from __future__ import annotations

import json
import re
from datetime import date, datetime
from decimal import Decimal
from functools import lru_cache
from typing import Any

from sqlalchemy import create_engine, text
from sqlalchemy.engine import Engine, make_url
from sqlalchemy.exc import SQLAlchemyError
from sqlglot import exp, parse
from sqlglot.errors import ParseError

from app.settings import get_settings


ROOM_MARKER = "__ROOM_ID__"

# 这里只暴露群聊内可查询且不敏感的字段。密码、管理凭据等字段即使真实存在，
# 也不能通过 AI 查询工具访问。
ROOM_QUERY_SCHEMA: dict[str, dict[str, Any]] = {
    "rooms": {
        "scope_column": "id",
        "columns": {
            "id", "type", "name", "creator_id", "agent_enabled",
            "moderation_enabled", "status", "last_active_at", "bot_user_id",
            "created_at", "updated_at", "deleted_at",
        },
    },
    "room_members": {
        "scope_column": "room_id",
        "columns": {"id", "room_id", "user_id", "role", "joined_at", "deleted_at"},
    },
    "users": {
        "scope_column": None,
        "columns": {
            "id", "username", "avatar", "role", "is_bot", "status",
            "last_login_at", "last_active_at", "created_at", "updated_at", "deleted_at",
        },
    },
    "messages": {
        "scope_column": "room_id",
        "columns": {
            "id", "room_id", "sender_id", "client_msg_id", "type", "content",
            "created_at", "deleted_at",
        },
    },
    "file_records": {
        "scope_column": "room_id",
        "columns": {
            "id", "object_key", "original_name", "sha256", "size", "uploader_id",
            "room_id", "backend", "status", "message_id", "created_at", "updated_at",
        },
    },
    "agent_configs": {
        "scope_column": "room_id",
        "columns": {
            "id", "room_id", "system_prompt", "trigger_mode", "trigger_words",
            "max_history", "temperature", "model_name", "rag_enabled", "top_k",
            "similarity_thold", "rerank_enabled", "max_chunk_tokens",
            "topic_chunk_min_msgs", "topic_chunk_model", "created_at", "updated_at",
            "deleted_at",
        },
    },
    "agent_message_inbox": {
        "scope_column": "room_id",
        "columns": {
            "id", "event_id", "topic", "partition_id", "kafka_offset", "room_id",
            "message_id", "sender_id", "sender_type", "content", "message_time",
            "status", "error_message", "created_at", "updated_at",
        },
    },
    "moderation_records": {
        "scope_column": "room_id",
        "columns": {
            "id", "inbox_id", "room_id", "message_id", "sender_id", "status",
            "rule_code", "reason", "evidence", "confidence", "created_at",
        },
    },
    "removal_requests": {
        "scope_column": "room_id",
        "columns": {
            "id", "room_id", "target_user_id", "moderation_record_id", "reason",
            "evidence", "status", "reviewed_by_user_id", "reviewed_at", "created_at",
            "updated_at", "deleted_at",
        },
    },
    "agent_chunks": {
        "scope_column": "room_id",
        "columns": {
            "id", "room_id", "binding_id", "topic_name", "content", "message_ids",
            "start_time", "end_time", "qdrant_point_id", "created_at",
        },
    },
    "room_agent_bindings": {
        "scope_column": "room_id",
        "columns": {
            "id", "room_id", "bot_id", "legacy_bot_user_id", "enabled", "priority",
            "created_at", "updated_at", "deleted_at",
        },
    },
}

ALLOWED_FUNCTIONS = {
    "AVG", "COALESCE", "COUNT", "DATE", "DATE_FORMAT", "DAY", "HOUR",
    "LENGTH", "LOWER", "MAX", "MIN", "MONTH", "ROUND", "SUM", "UPPER", "YEAR",
}


class RoomQueryValidationError(ValueError):
    pass


def schema_for_prompt() -> str:
    lines = [
        "可查询的 MySQL Schema（未列出的表和字段禁止访问）：",
        "所有 SQL 必须使用表别名，并在 WHERE 中用 __ROOM_ID__ 表示当前群，禁止写具体群号。",
    ]
    for table_name, definition in ROOM_QUERY_SCHEMA.items():
        columns = ", ".join(sorted(definition["columns"]))
        scope = definition["scope_column"]
        suffix = f"；群范围字段={scope}" if scope else "；必须通过 room_members.user_id 关联"
        lines.append(f"- {table_name}({columns}){suffix}")
    lines.extend([
        "示例：SELECT m.sender_id, COUNT(*) AS total FROM messages m "
        "WHERE m.room_id = __ROOM_ID__ GROUP BY m.sender_id LIMIT 20",
        "查询 users 时必须同时关联 room_members，例如：SELECT u.id, u.username "
        "FROM users u JOIN room_members rm ON rm.user_id = u.id "
        "WHERE rm.room_id = __ROOM_ID__ AND rm.deleted_at = 0 LIMIT 20",
    ])
    return "\n".join(lines)


def _is_room_marker(node: exp.Expression) -> bool:
    return isinstance(node, exp.Column) and not node.table and node.name.upper() == ROOM_MARKER


def _is_column(node: exp.Expression, alias: str, column: str) -> bool:
    return (
        isinstance(node, exp.Column)
        and node.table.lower() == alias.lower()
        and node.name.lower() == column.lower()
    )


def _is_unconditional_condition_term(equality: exp.EQ, select: exp.Select) -> bool:
    current = equality.parent
    while current is not None and current is not select:
        if isinstance(current, (exp.And, exp.Paren)):
            current = current.parent
            continue
        if isinstance(current, (exp.Where, exp.Join)):
            return current.find_ancestor(exp.Select) is select
        return False
    return False


def _has_equality(select: exp.Select, left_alias: str, left_column: str, right_alias: str, right_column: str) -> bool:
    for equality in select.find_all(exp.EQ):
        if equality.find_ancestor(exp.Select) is not select:
            continue
        left, right = equality.this, equality.expression
        matches = (
            _is_column(left, left_alias, left_column)
            and _is_column(right, right_alias, right_column)
        ) or (
            _is_column(right, left_alias, left_column)
            and _is_column(left, right_alias, right_column)
        )
        if matches and _is_unconditional_condition_term(equality, select):
            return True
    return False


def _is_unconditional_where_term(equality: exp.EQ, where: exp.Where) -> bool:
    current = equality.parent
    while current is not None and current is not where:
        if not isinstance(current, (exp.And, exp.Paren)):
            return False
        current = current.parent
    return current is where


def _where_room_scope_equality(select: exp.Select, alias: str, column: str) -> exp.EQ | None:
    where = select.args.get("where")
    if where is None:
        return None
    for equality in where.find_all(exp.EQ):
        left, right = equality.this, equality.expression
        matches = (
            _is_column(left, alias, column) and _is_room_marker(right)
        ) or (
            _is_column(right, alias, column) and _is_room_marker(left)
        )
        if matches and _is_unconditional_where_term(equality, where):
            return equality
    return None


def validate_room_select(sql: str, max_rows: int) -> str:
    candidate = sql.strip()
    if not candidate:
        raise RoomQueryValidationError("SQL 不能为空")
    if len(candidate) > 8000:
        raise RoomQueryValidationError("SQL 长度超过 8000 字符")
    if re.search(r"(--|#|/\*|\*/)", candidate):
        raise RoomQueryValidationError("SQL 不允许包含注释")

    try:
        statements = parse(candidate, read="mysql")
    except ParseError as exc:
        raise RoomQueryValidationError(f"SQL 语法错误：{exc}") from exc
    if len(statements) != 1 or not isinstance(statements[0], exp.Select):
        raise RoomQueryValidationError("只允许执行一条 SELECT 语句")

    statement = statements[0]
    if len(list(statement.find_all(exp.Select))) != 1 or statement.find(exp.Subquery) is not None:
        raise RoomQueryValidationError("暂不允许子查询、CTE 或 UNION，请改写为单层 SELECT/JOIN")
    forbidden_select_parts = tuple(
        item for item in (getattr(exp, "Into", None), getattr(exp, "Lock", None)) if item is not None
    )
    if any(statement.find(item) is not None for item in forbidden_select_parts):
        raise RoomQueryValidationError("不允许 SELECT INTO 或锁定读取")

    tables = list(statement.find_all(exp.Table))
    if not tables:
        raise RoomQueryValidationError("查询必须访问一个群聊范围内的数据表")

    aliases: dict[str, str] = {}
    for table in tables:
        table_name = table.name.lower()
        if table_name not in ROOM_QUERY_SCHEMA:
            raise RoomQueryValidationError(f"表 {table.name} 不在允许查询的 Schema 中")
        alias = table.alias_or_name.lower()
        if alias in aliases:
            raise RoomQueryValidationError(f"表别名 {alias} 重复")
        aliases[alias] = table_name

    for star in statement.find_all(exp.Star):
        if star.find_ancestor(exp.Count) is None:
            raise RoomQueryValidationError("禁止 SELECT *，请明确列出需要的非敏感字段；COUNT(*) 可以使用")

    marker_columns: list[exp.Column] = []
    for column in statement.find_all(exp.Column):
        if _is_room_marker(column):
            marker_columns.append(column)
            continue
        alias = column.table.lower()
        if not alias:
            raise RoomQueryValidationError(f"字段 {column.name} 必须带表别名")
        table_name = aliases.get(alias)
        if table_name is None:
            raise RoomQueryValidationError(f"未知表别名 {column.table}")
        if column.name.lower() not in ROOM_QUERY_SCHEMA[table_name]["columns"]:
            raise RoomQueryValidationError(f"字段 {table_name}.{column.name} 不允许查询或不存在")

    for function in statement.find_all(exp.Func):
        # sqlglot 的 Func 基类也包含 AND 等语法节点，只校验真正带括号的函数调用。
        function_name = (
            function.name.upper()
            if isinstance(function, exp.Anonymous)
            else function.sql_name().upper()
        )
        rendered_function = function.sql(dialect="mysql").lstrip().upper()
        if not rendered_function.startswith(f"{function_name}("):
            continue
        if function_name not in ALLOWED_FUNCTIONS:
            raise RoomQueryValidationError(f"函数 {function_name} 不在允许列表中")

    scoped_aliases: list[tuple[str, str]] = []
    member_aliases: list[str] = []
    user_aliases: list[str] = []
    for alias, table_name in aliases.items():
        scope_column = ROOM_QUERY_SCHEMA[table_name]["scope_column"]
        if scope_column:
            scoped_aliases.append((alias, scope_column))
        if table_name == "room_members":
            member_aliases.append(alias)
        elif table_name == "users":
            user_aliases.append(alias)

    approved_scope_equalities: set[int] = set()
    for alias, scope_column in scoped_aliases:
        equality = _where_room_scope_equality(statement, alias, scope_column)
        if equality is None:
            raise RoomQueryValidationError(
                f"WHERE 必须以 AND 条件包含 {alias}.{scope_column} = {ROOM_MARKER}，由服务端绑定当前群"
            )
        approved_scope_equalities.add(id(equality))

    for user_alias in user_aliases:
        if not any(
            _has_equality(statement, user_alias, "id", member_alias, "user_id")
            for member_alias in member_aliases
        ):
            raise RoomQueryValidationError(
                f"users 别名 {user_alias} 必须通过 room_members.user_id 与本群成员关联"
            )

    # __ROOM_ID__ 只能出现在已验证的群范围等值条件中，不能作为普通表达式使用。
    for marker in marker_columns:
        equality = marker.find_ancestor(exp.EQ)
        if equality is None or id(equality) not in approved_scope_equalities:
            raise RoomQueryValidationError(f"{ROOM_MARKER} 只能用于 WHERE 中的群范围等值条件")

    limit = statement.args.get("limit")
    if limit is None:
        statement = statement.limit(max_rows)
    else:
        limit_value = limit.expression
        if not isinstance(limit_value, exp.Literal) or not limit_value.is_int:
            raise RoomQueryValidationError("LIMIT 必须是整数常量")
        if int(limit_value.this) > max_rows:
            raise RoomQueryValidationError(f"LIMIT 不能超过 {max_rows}")

    rendered = statement.sql(dialect="mysql")
    return re.sub(rf"\b{ROOM_MARKER}\b", ":room_id", rendered, flags=re.IGNORECASE)


@lru_cache(maxsize=1)
def _query_engine() -> Engine:
    settings = get_settings().database
    url = make_url(settings.query_url or settings.url)
    if url.drivername == "mysql+aiomysql":
        url = url.set(drivername="mysql+pymysql")
    return create_engine(
        url,
        pool_pre_ping=True,
        pool_recycle=settings.pool_recycle_seconds,
        pool_size=min(settings.pool_size, 5),
        max_overflow=min(settings.max_overflow, 5),
    )


def _json_value(value: Any) -> Any:
    if value is None or isinstance(value, (str, int, float, bool)):
        return value
    if isinstance(value, (datetime, date)):
        return value.isoformat()
    if isinstance(value, Decimal):
        return float(value)
    if isinstance(value, bytes):
        return value.decode("utf-8", errors="replace")
    return str(value)


def execute_room_select(sql: str, room_id: int) -> str:
    settings = get_settings().database
    try:
        validated_sql = validate_room_select(sql, settings.query_max_rows)
    except RoomQueryValidationError as exc:
        return json.dumps({
            "ok": False,
            "error_type": "validation_error",
            "message": str(exc),
            "hint": "根据 Schema 和错误信息修改 SQL 后再次调用工具。",
        }, ensure_ascii=False)

    try:
        with _query_engine().connect() as connection:
            connection.exec_driver_sql(
                f"SET SESSION MAX_EXECUTION_TIME = {int(settings.query_timeout_ms)}"
            )
            result = connection.execute(text(validated_sql), {"room_id": room_id})
            rows = [
                {key: _json_value(value) for key, value in row._mapping.items()}
                for row in result.fetchall()
            ]
        payload = json.dumps({
            "ok": True,
            "room_id": room_id,
            "row_count": len(rows),
            "rows": rows,
        }, ensure_ascii=False)
        if len(payload) > settings.query_max_output_chars:
            return json.dumps({
                "ok": False,
                "error_type": "result_too_large",
                "message": f"查询结果超过 {settings.query_max_output_chars} 字符",
                "hint": "减少查询字段、缩小 LIMIT 或使用 COUNT/SUM 等聚合后重试。",
            }, ensure_ascii=False)
        return payload
    except SQLAlchemyError as exc:
        message = str(getattr(exc, "orig", exc))[:1200]
        return json.dumps({
            "ok": False,
            "error_type": "database_error",
            "message": message,
            "hint": "数据库返回错误。检查表名、字段名、聚合和 GROUP BY 后修改 SQL 并重试。",
        }, ensure_ascii=False)
