from http.server import BaseHTTPRequestHandler, HTTPServer
import json

C = {
    "SECURITY_RISKS": 32,
    "CIPA": 34,
    "VIOLENCE": 32,
    "SECURITY_THREATS": 21,
    "QUESTIONABLE_CONTENT": 17,

}
class PatternMatchingServer(BaseHTTPRequestHandler):
    # Define URL patterns and their corresponding responses here
    URL_PATTERNS = {
        "/categories/192.168.0.3": {"categories": [C["SECURITY_RISKS"], C["SECURITY_THREATS"], C["CIPA"]]},
        "/categories/192.168.0.191": {"categories": [C["SECURITY_RISKS"], C["SECURITY_THREATS"], C["CIPA"]]},
        "/categories/::1": {"categories": [C["SECURITY_RISKS"], C["SECURITY_THREATS"], C["CIPA"]]},
        # Add more patterns here as needed, for example:
        # "/api/users": {"categories": [1, 2, 3]},
        # "/dashboard": {"categories": [4, 5, 6]},
    }
    
    # Default response for unmatched patterns
    DEFAULT_RESPONSE = {"categories": []}
    
    def do_GET(self):
        # Set response headers
        self.send_response(200)
        self.send_header('Content-Type', 'application/json')
        self.send_header('Access-Control-Allow-Origin', '*')  # Enable CORS for testing
        self.end_headers()
        
        # Get response based on URL pattern
        response = self.URL_PATTERNS.get(self.path, self.DEFAULT_RESPONSE)
        
        # Send the JSON response
        self.wfile.write(json.dumps(response).encode('utf-8'))
        
        # Log the request
        print(f"Request: {self.path} -> Response: {response}")

def run_server(host="0.0.0.0", port=8080):
    """Start the web server on the specified host and port"""
    server_address = (host, port)
    httpd = HTTPServer(server_address, PatternMatchingServer)
    print(f"Server running at http://{host}:{port}")
    httpd.serve_forever()

if __name__ == "__main__":
    try:
        run_server()
    except KeyboardInterrupt:
        print("\nShutting down server...")