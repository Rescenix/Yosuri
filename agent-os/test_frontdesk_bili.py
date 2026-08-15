import importlib.util
import pathlib
import unittest
from unittest import mock


MODULE_PATH = pathlib.Path(__file__).with_name("frontdesk_bili.py")
SPEC = importlib.util.spec_from_file_location("frontdesk_bili", MODULE_PATH)
frontdesk = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(frontdesk)


class FrontdeskDeliveryTests(unittest.TestCase):
    def test_multiline_invite_is_normalized_to_one_message(self):
        text = "收到，马上安排\n你的邀请码：RS-ABCD-EFGH-JK23\n去下载页输入"
        self.assertEqual(
            frontdesk.normalize_dm_text(text),
            "收到，马上安排 你的邀请码：RS-ABCD-EFGH-JK23 去下载页输入",
        )

    def test_recovery_detects_promise_without_code(self):
        history = "我: 三连后告诉我，我马上给你安排\n对方: 已三连，好了\n我: 收到"
        self.assertTrue(frontdesk.history_requires_code_recovery(history))

    def test_recovery_does_not_require_old_confirmation_in_window(self):
        history = "我: 收到，马上给你安排邀\n对方: 好的\n我: 有什么想了解的随时告诉我"
        self.assertTrue(frontdesk.history_requires_code_recovery(history))

    def test_recovery_stops_after_code_is_present(self):
        history = "对方: 已三连\n我: 马上安排\n我: 邀请码 RS-ABCD-EFGH-JK23"
        self.assertFalse(frontdesk.history_requires_code_recovery(history))

    @mock.patch.object(frontdesk.time, "sleep", return_value=None)
    @mock.patch.object(frontdesk, "dm_delivery_confirmed", return_value=True)
    @mock.patch.object(frontdesk, "send_dm", return_value=True)
    def test_dm_try_requires_history_confirmation(self, send_dm, confirmed, _sleep):
        self.assertTrue(frontdesk.dm_try("123", "邀请码 RS-ABCD-EFGH-JK23", "RS-ABCD-EFGH-JK23"))
        send_dm.assert_called_once()
        confirmed.assert_called_once_with("123", "RS-ABCD-EFGH-JK23")

    @mock.patch.object(frontdesk, "log")
    def test_empty_uid_is_rejected_without_browser_work(self, log):
        self.assertFalse(frontdesk.send_dm("", "hello"))
        log.assert_called_once()

    @mock.patch.object(frontdesk.time, "sleep", return_value=None)
    @mock.patch.object(frontdesk, "dm_delivery_confirmed", return_value=False)
    @mock.patch.object(frontdesk, "send_dm", return_value=True)
    @mock.patch.object(frontdesk, "log")
    def test_truncated_delivery_retries_then_reports_failure(self, _log, send_dm, confirmed, _sleep):
        self.assertFalse(frontdesk.dm_try("123", "邀请码 RS-ABCD-EFGH-JK23", "RS-ABCD-EFGH-JK23"))
        self.assertEqual(send_dm.call_count, 2)
        self.assertEqual(confirmed.call_count, 6)


if __name__ == "__main__":
    unittest.main()
