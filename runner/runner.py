from http.server import HTTPServer, BaseHTTPRequestHandler
import json
import traceback

class RunHandler(BaseHTTPRequestHandler):
    def do_POST(self):
        if self.path != '/run':
            self.send_response(404)
            self.end_headers()
            return

        length = int(self.headers['Content-Length'])
        body = json.loads(self.rfile.read(length))
        code = body.get('code', '')
        event = body.get('event', {})

        try:
            namespace = {}
            exec(code, namespace)
            result = namespace['handler'](event)
            response = {'status': 'ok', 'output': str(result)}
        except Exception as e:
            response = {'status': 'error', 'output': traceback.format_exc()}

        self.send_response(200)
        self.send_header('Content-Type', 'application/json')
        self.end_headers()
        self.wfile.write(json.dumps(response).encode())

    def log_message(self, format, *args):
        pass

if __name__ == '__main__':
    server = HTTPServer(('0.0.0.0', 9000), RunHandler)
    print('runner listening on 9000', flush=True)
    server.serve_forever()
