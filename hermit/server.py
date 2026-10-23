from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
import json
from pathlib import Path
import secrets
import threading
from urllib.parse import urlsplit
import webbrowser

from hermit import HermitError

WEB = Path(__file__).parent / "web"


def create_server(service, port=0):
    token = secrets.token_urlsafe(32)

    class Handler(BaseHTTPRequestHandler):
        def log_message(self, *args):
            pass

        def send(self, status, body, content_type="application/json; charset=utf-8"):
            raw = json.dumps(body, ensure_ascii=False).encode() if content_type.startswith("application/json") else body
            self.send_response(status)
            for key, value in {
                "Content-Type": content_type, "Content-Length": str(len(raw)), "Cache-Control": "no-store",
                "X-Content-Type-Options": "nosniff", "Referrer-Policy": "no-referrer",
                "Content-Security-Policy": "default-src 'self'; script-src 'self'; style-src 'self'; img-src 'self' data:; connect-src 'self'; frame-ancestors 'none'; base-uri 'none'; object-src 'none'",
            }.items():
                self.send_header(key, value)
            self.end_headers()
            self.wfile.write(raw)

        def authorized(self):
            expected_host = f"127.0.0.1:{self.server.server_port}"
            if self.headers.get("Host") != expected_host:
                self.send(403, {"error": "拒绝非本机来源请求"})
                return False
            provided = self.headers.get("X-Hermit-Token", "")
            if not secrets.compare_digest(provided, token):
                self.send(401, {"error": "会话已过期，请重新启动 Hermit 打开管理界面"})
                return False
            origin = self.headers.get("Origin")
            if origin and origin != "http://" + expected_host:
                self.send(403, {"error": "拒绝跨站请求"})
                return False
            if self.headers.get("Sec-Fetch-Site") not in {None, "same-origin", "none"}:
                self.send(403, {"error": "拒绝跨站请求"})
                return False
            return True

        def do_GET(self):
            path = urlsplit(self.path).path
            static = {"/": ("index.html", "text/html; charset=utf-8"),
                      "/app.js": ("app.js", "text/javascript; charset=utf-8"),
                      "/style.css": ("style.css", "text/css; charset=utf-8"),
                      "/icon.svg": ("icon.svg", "image/svg+xml")}
            if path in static:
                filename, content_type = static[path]
                self.send(200, (WEB / filename).read_bytes(), content_type)
                return
            if not self.authorized():
                return
            try:
                if path == "/api/state":
                    self.send(200, service.status())
                elif path == "/api/nodes":
                    self.send(200, [{"name": name, "type": proxy["type"]} for name, proxy in service.clash.nodes().items()])
                elif path == "/api/doctor":
                    self.send(200, service.clash.doctor())
                else:
                    self.send(404, {"error": "接口不存在"})
            except HermitError as error:
                self.send(409, {"error": str(error)})
            except Exception:
                self.send(500, {"error": "读取本地状态失败，请检查 Clash 及目录权限"})

        def do_POST(self):
            if not self.authorized():
                return
            try:
                if self.headers.get("Content-Type", "").split(";")[0].strip() != "application/json":
                    self.send(415, {"error": "需要 JSON 请求"})
                    return
                length = int(self.headers.get("Content-Length", "0"))
                if not 0 < length <= 32768:
                    self.send(413, {"error": "请求体为空或过大"})
                    return
                data = json.loads(self.rfile.read(length))
                if not isinstance(data, dict):
                    raise ValueError()
                path = urlsplit(self.path).path
                if path == "/api/mappings/save":
                    result = service.save_mapping(data, data.get("id"))
                elif path == "/api/mappings/delete":
                    result = service.delete_mapping(data.get("id"))
                elif path == "/api/enable":
                    result = service.apply(True)
                elif path == "/api/disable":
                    result = service.apply(False)
                elif path == "/api/apply":
                    result = service.apply(None)
                elif path == "/api/recover":
                    result = service.recover()
                elif path == "/api/test":
                    result = service.test(data.get("id"))
                elif path == "/api/quit":
                    self.send(200, {"status": "quitting"})
                    threading.Thread(target=self.server.shutdown, daemon=True).start()
                    return
                else:
                    self.send(404, {"error": "接口不存在"})
                    return
                self.send(200, result)
            except HermitError as error:
                self.send(409, {"error": str(error)})
            except (ValueError, TypeError):
                self.send(400, {"error": "请求字段或 JSON 格式无效"})
            except Exception:
                self.send(500, {"error": "本地操作失败；已保留状态和恢复记录，请查看当前状态"})

    server = ThreadingHTTPServer(("127.0.0.1", port), Handler)
    server.daemon_threads = True
    server.session_token = token
    return server


def serve(service, port=0, open_browser=True):
    server = create_server(service, port)
    if open_browser:
        webbrowser.open(f"http://127.0.0.1:{server.server_port}/#" + server.session_token)
    print(f"Hermit 正在运行：http://127.0.0.1:{server.server_port}/（通过应用启动的浏览器会话访问）", flush=True)
    try:
        server.serve_forever()
    except KeyboardInterrupt:
        pass
    finally:
        server.server_close()
