"""Synthetic regression checks for request validation and private-session lifecycle."""
import json
import unittest
from unittest.mock import patch
from email.message import EmailMessage
import app
import test_viewer


class HardeningTest(unittest.TestCase):
    request_without_socket = test_viewer.ViewerTest.request_without_socket
    def setUp(self):
        self.saved = (dict(app.MESSAGES), dict(app.SOURCES), dict(app.PLANS))
        app.MESSAGES.clear(); app.SOURCES.clear(); app.PLANS.clear()
        message = EmailMessage(); message['Subject'] = 'Synthetic hardening check'
        message.set_content('Keep state after invalid requests.')
        message.add_attachment(b'proof', maintype='text', subtype='plain', filename='proof.txt')
        self.raw = message.as_bytes()
        status, data = self.request('/import', self.raw, {'X-Filename': 'check.eml'})
        self.assertEqual(status, 200)
        self.message = json.loads(data)[0]

    def tearDown(self):
        for target, saved in zip((app.MESSAGES, app.SOURCES, app.PLANS), self.saved):
            target.clear(); target.update(saved)

    def request(self, path, raw, extra=None):
        return self.request_without_socket('POST', path, {
            'Content-Length': str(len(raw)), 'X-Viewer-Token': app.TOKEN, **(extra or {})}, raw)

    def test_invalid_shapes_preserve_session_and_legitimate_export_works(self):
        mid = self.message['id']
        for path, payload in [('/plan', []), ('/export-emails', {'messageIds': [mid], 'format': 'bad'}),
                              ('/plan', {'action': 'messages', 'messageIds': mid}),
                              ('/export', {'attachments': {mid: [True]}}),
                              ('/export', {'attachments': {mid: [-1]}}),
                              ('/export', {'attachments': {mid: [0, 0]}}),
                              ('/export', {'attachments': {mid: []}}),
                              ('/export', {'attachments': {'missing': [0]}})]:
            before = (dict(app.MESSAGES), dict(app.SOURCES))
            self.assertEqual(self.request(path, json.dumps(payload).encode())[0], 400, payload)
            self.assertEqual((app.MESSAGES, app.SOURCES), before)
        self.assertEqual(self.request('/export', json.dumps({'attachments': {mid: [0]}}).encode())[0], 200)

    def test_truncated_json_and_framing_fail_closed(self):
        self.assertEqual(self.request('/plan', b'{', {'Content-Length': '4'})[0], 400)
        self.assertEqual(self.request('/clear', b'{}', {'Transfer-Encoding': 'chunked'})[0], 400)
        self.assertIn(self.message['id'], app.MESSAGES)

    def test_import_capacity_and_type_rejection_preserve_existing_session(self):
        before = (dict(app.MESSAGES), dict(app.SOURCES))
        with patch.object(app, 'MAX_MESSAGES', 1):
            self.assertEqual(self.request('/import', self.raw)[0], 400)
        with patch.object(app, 'MAX_SESSION_BYTES', 1):
            self.assertEqual(self.request('/import', self.raw)[0], 400)
        self.assertEqual(self.request('/import', self.raw, {'X-Filename': 'bad.exe'})[0], 400)
        self.assertEqual((app.MESSAGES, app.SOURCES), before)

    def test_clear_discards_all_server_copies_and_plans(self):
        app.create_plan({'action': 'messages', 'messageIds': [self.message['id']]})
        self.assertTrue(app.PLANS)
        self.assertEqual(self.request('/clear', b'{}')[0], 200)
        self.assertEqual((app.MESSAGES, app.SOURCES, app.PLANS), ({}, {}, {}))

    def test_theme_and_token_delivery_require_no_inline_script(self):
        status, html = self.request_without_socket()
        self.assertEqual(status, 200)
        self.assertIn(b'name="viewer-token"', html)
        self.assertNotIn(b'<script>window.VIEWER_TOKEN', html)


if __name__ == '__main__':
    unittest.main()
