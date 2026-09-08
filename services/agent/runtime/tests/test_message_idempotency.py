import unittest

from app.workers.consumer import decode_message, inbox_event_id


class MessageIdempotencyTest(unittest.TestCase):
    def test_formal_event_ignores_kafka_offset(self):
        payload = {"message_id": "9007199254740993", "room_seq": 43}
        self.assertEqual(inbox_event_id(payload, "formal", 0, 10), inbox_event_id(payload, "formal", 0, 20))

    def test_legacy_client_key_is_scoped_by_sender(self):
        left = {"sender_id": 1, "client_msg_id": "same"}
        right = {"sender_id": 2, "client_msg_id": "same"}
        self.assertNotEqual(inbox_event_id(left, "formal", 0, 10), inbox_event_id(right, "formal", 0, 10))

    def test_protobuf_preserves_formal_identity(self):
        from im.v1.message_pb2 import ChatMessage
        message = ChatMessage(room_id=1, sender_id=2, client_msg_id="C1", message_id=9007199254740993, room_seq=43, created_at_ns=1759999999999000000)
        payload = decode_message(message.SerializeToString())
        self.assertEqual(payload["message_id"], "9007199254740993")
        self.assertEqual(payload["room_seq"], 43)


if __name__ == "__main__":
    unittest.main()
