#!/usr/bin/env python3
"""Run an isolated real Balemoh API/BFF against an atomically replaced snapshot."""

import argparse
import base64
import http.server
import json
import os
from pathlib import Path
import secrets
import signal
import socket
import ssl
import subprocess
import threading
import time
import urllib.request


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("plugin", type=Path)
    parser.add_argument("snapshot", type=Path)
    parser.add_argument("state", type=Path)
    parser.add_argument("--port", type=int, default=14083)
    args = parser.parse_args()
    plugin, snapshot, state = args.plugin.resolve(), args.snapshot.resolve(), args.state.resolve()
    if not snapshot.is_file():
        raise ValueError("normalize a validated full-dry-run snapshot before starting the host")
    state.mkdir(mode=0o700)
    os.chmod(state, 0o700)

    def write(name, data):
        p = state / name
        p.write_bytes(data if isinstance(data, bytes) else data.encode())
        p.chmod(0o600)
        return str(p)

    source_token, service_token = secrets.token_hex(32), secrets.token_hex(32)

    class Source(http.server.BaseHTTPRequestHandler):
        def do_GET(self):
            if self.headers.get("Authorization") != "Bearer " + source_token:
                self.send_error(401)
                return
            body = snapshot.read_bytes()
            self.send_response(200)
            self.send_header("Content-Type", "application/json")
            self.send_header("Content-Length", str(len(body)))
            self.end_headers()
            self.wfile.write(body)

        def log_message(self, *_):
            pass

    subprocess.run(["openssl", "req", "-x509", "-newkey", "rsa:2048", "-nodes", "-days", "2",
                    "-keyout", str(state / "key.pem"), "-out", str(state / "cert.pem"),
                    "-subj", "/CN=127.0.0.1", "-addext", "subjectAltName=IP:127.0.0.1"],
                   check=True, stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL)
    os.chmod(state / "key.pem", 0o600)
    server = http.server.ThreadingHTTPServer(("127.0.0.1", 0), Source)
    tls = ssl.SSLContext(ssl.PROTOCOL_TLS_SERVER)
    tls.load_cert_chain(state / "cert.pem", state / "key.pem")
    server.socket = tls.wrap_socket(server.socket, server_side=True)
    threading.Thread(target=server.serve_forever, daemon=True).start()
    der = ssl.PEM_cert_to_DER_cert((state / "cert.pem").read_text())
    ca_file = write("source-ca", base64.b64encode(der))
    source_file = write("source-token", source_token)
    service_file = write("service-token", service_token)
    package = plugin / "dist/package"
    digest = json.loads(subprocess.check_output(["go", "run", "./cmd/package-info", str(package)], cwd=plugin))
    install = {"package": str(package), **digest}
    config = {"snapshotURL": f"https://127.0.0.1:{server.server_port}",
              "browserRepositoryURL": "https://github.com/guilycst/balemoh-renovate-engine-poc-20261101",
              "intervalSeconds": 30, "notificationsEnabled": False, "serviceMappingsJSON": "[]"}
    api_config = write("api.json", json.dumps({"installations": [{**install, "connections": [{
        "key": "engine-proof", "name": "Isolated Renovate engine proof", "config": config,
        "secrets": {"snapshot-token": source_file, "snapshot-ca": ca_file},
        "grants": ["upgrades.list", "upgrades.get", "upgrades.summary"], "capabilityGrants": []}]}]}))
    ui_config = write("ui.json", json.dumps({"installations": [{**install, "connectionKey": "engine-proof"}]}))
    with socket.socket() as listener:
        listener.bind(("127.0.0.1", 0))
        api_port = listener.getsockname()[1]
    api_url, ui_url = f"http://127.0.0.1:{api_port}", f"http://127.0.0.1:{args.port}"
    processes = []

    def start(binary, environment):
        log = open(state / (binary.name + ".log"), "wb")
        processes.append(subprocess.Popen([str(binary)], cwd=state, stdout=log, stderr=log,
                         env={"HOME": str(state), "PATH": "/usr/bin:/bin:/usr/sbin:/sbin", **environment}))
        log.close()

    def stop(*_):
        raise KeyboardInterrupt

    signal.signal(signal.SIGTERM, stop)
    try:
        start(plugin / ".local/deployed-balemoh-api-latest", {
            "BALEMOH_HTTP_ADDR": f"127.0.0.1:{api_port}", "BALEMOH_DATABASE_PATH": str(state / "db.sqlite"),
            "BALEMOH_PLUGIN_CONFIG": api_config, "BALEMOH_PLUGIN_SERVICE_TOKEN_FILE": service_file})
        start(plugin / ".local/deployed-balemoh-ui-latest", {
            "BALEMOH_UI_HTTP_ADDR": f"127.0.0.1:{args.port}", "BALEMOH_UI_API_BASE_URL": api_url,
            "BALEMOH_UI_PLUGIN_CONFIG": ui_config, "BALEMOH_UI_PLUGIN_SERVICE_TOKEN_FILE": service_file,
            "BALEMOH_UI_ICON_CACHE_DIR": str(state / "icons"), "BALEMOH_UI_ICON_STARTUP_TIMEOUT": "10ms"})
        deadline = time.monotonic() + 45
        while time.monotonic() < deadline:
            try:
                request = urllib.request.Request(api_url + "/api/v1/plugins", headers={"Authorization": "Bearer " + service_token})
                with urllib.request.urlopen(request, timeout=2) as response:
                    connection = json.load(response)["plugins"][0]["connections"][0]
                if connection.get("lastSuccessAt"):
                    break
            except (OSError, IndexError, KeyError):
                pass
            time.sleep(0.2)
        else:
            raise RuntimeError("isolated host did not retain the snapshot")
        page = f"/plugins/araihu.image-upgrades/connections/{connection['id']}/pages/current"
        write("receipt.json", json.dumps({"apiURL": api_url, "page": ui_url + page,
              "connectionId": connection["id"], **digest}, indent=2))
        print(ui_url + page, flush=True)
        while True:
            signal.pause()
    finally:
        for p in processes:
            p.terminate()
        for p in processes:
            try:
                p.wait(timeout=10)
            except subprocess.TimeoutExpired:
                p.kill()
                p.wait()
        server.shutdown()


if __name__ == "__main__":
    try:
        main()
    except KeyboardInterrupt:
        pass
