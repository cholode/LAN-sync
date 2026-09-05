from __future__ import annotations

import unittest

from app.storage.room_query import RoomQueryValidationError, validate_room_select


class RoomQueryValidationTests(unittest.TestCase):
    def test_accepts_scoped_aggregate_and_adds_limit(self) -> None:
        sql = (
            "SELECT m.sender_id, COUNT(*) AS total FROM messages m "
            "WHERE m.room_id = __ROOM_ID__ GROUP BY m.sender_id"
        )
        validated = validate_room_select(sql, 100)
        self.assertIn(":room_id", validated)
        self.assertIn("LIMIT 100", validated)

    def test_accepts_users_only_through_scoped_membership(self) -> None:
        sql = (
            "SELECT u.id, u.username FROM users u "
            "JOIN room_members rm ON rm.user_id = u.id "
            "WHERE rm.room_id = __ROOM_ID__ AND rm.deleted_at = 0 LIMIT 20"
        )
        self.assertIn(":room_id", validate_room_select(sql, 100))

    def test_rejects_write_statement(self) -> None:
        with self.assertRaises(RoomQueryValidationError):
            validate_room_select("DELETE FROM messages WHERE room_id = 1", 100)

    def test_rejects_literal_room_id(self) -> None:
        with self.assertRaises(RoomQueryValidationError):
            validate_room_select("SELECT m.id FROM messages m WHERE m.room_id = 99", 100)

    def test_rejects_or_scope_bypass(self) -> None:
        with self.assertRaises(RoomQueryValidationError):
            validate_room_select(
                "SELECT m.id FROM messages m WHERE m.room_id = __ROOM_ID__ OR 1 = 1",
                100,
            )

    def test_rejects_sensitive_column(self) -> None:
        with self.assertRaises(RoomQueryValidationError):
            validate_room_select(
                "SELECT u.password FROM users u "
                "JOIN room_members rm ON rm.user_id = u.id "
                "WHERE rm.room_id = __ROOM_ID__",
                100,
            )

    def test_rejects_users_without_membership_join(self) -> None:
        with self.assertRaises(RoomQueryValidationError):
            validate_room_select("SELECT u.id FROM users u", 100)

    def test_rejects_bypassable_user_membership_join(self) -> None:
        with self.assertRaises(RoomQueryValidationError):
            validate_room_select(
                "SELECT u.id FROM users u JOIN room_members rm "
                "ON rm.user_id = u.id OR 1 = 1 WHERE rm.room_id = __ROOM_ID__",
                100,
            )

    def test_rejects_multiple_statements(self) -> None:
        with self.assertRaises(RoomQueryValidationError):
            validate_room_select(
                "SELECT m.id FROM messages m WHERE m.room_id = __ROOM_ID__; SELECT 1",
                100,
            )

    def test_rejects_dangerous_function(self) -> None:
        with self.assertRaises(RoomQueryValidationError):
            validate_room_select(
                "SELECT SLEEP(10) FROM messages m WHERE m.room_id = __ROOM_ID__",
                100,
            )

    def test_requires_every_scoped_join_to_use_current_room(self) -> None:
        with self.assertRaises(RoomQueryValidationError):
            validate_room_select(
                "SELECT m.id, f.original_name FROM messages m "
                "JOIN file_records f ON f.message_id = m.id "
                "WHERE m.room_id = __ROOM_ID__",
                100,
            )

    def test_rejects_room_marker_outside_scope_condition(self) -> None:
        with self.assertRaises(RoomQueryValidationError):
            validate_room_select(
                "SELECT __ROOM_ID__, m.id FROM messages m WHERE m.room_id = __ROOM_ID__",
                100,
            )


if __name__ == "__main__":
    unittest.main()
