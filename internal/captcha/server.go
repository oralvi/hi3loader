package captcha

import (
	"context"
	"embed"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

//go:embed assets/*.html
var embeddedTemplates embed.FS

type Server struct {
	addr   string
	server *http.Server
	onRet  func(map[string]any)

	mu      sync.RWMutex
	ln      net.Listener
	lastRet map[string]any
}

func NewServer(addr string, onRet func(map[string]any)) *Server {
	s := &Server{addr: addr, onRet: onRet}
	mux := http.NewServeMux()
	mux.HandleFunc("/", s.handleTemplate("index.html", fallbackIndex))
	mux.HandleFunc("/geetest", s.handleTemplate("geetest.html", fallbackGeetest))
	mux.HandleFunc("/ret", s.handleRet)
	mux.HandleFunc("/last", s.handleLast)
	s.server = &http.Server{
		Addr:    addr,
		Handler: mux,
	}
	return s
}

func (s *Server) ListenAndServe() error {
	if err := s.Prepare(); err != nil {
		return err
	}

	s.mu.RLock()
	ln := s.ln
	s.mu.RUnlock()
	if ln == nil {
		return errors.New("captcha listener is not ready")
	}
	return s.server.Serve(ln)
}

func (s *Server) Shutdown(ctx context.Context) error {
	s.mu.RLock()
	ln := s.ln
	s.mu.RUnlock()
	if ln == nil {
		return nil
	}
	return s.server.Shutdown(ctx)
}

func (s *Server) Addr() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.addr
}

func (s *Server) Prepare() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.ln != nil {
		return nil
	}

	ln, err := net.Listen("tcp", s.addr)
	if err != nil {
		return err
	}

	s.ln = ln
	s.addr = ln.Addr().String()
	return nil
}

func (s *Server) Last() map[string]any {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if s.lastRet == nil {
		return nil
	}
	out := make(map[string]any, len(s.lastRet))
	for k, v := range s.lastRet {
		out[k] = v
	}
	return out
}

func (s *Server) handleRet(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost && r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	payload, err := decodePayload(r)
	if err != nil {
		http.Error(w, "invalid payload", http.StatusBadRequest)
		return
	}

	s.mu.Lock()
	s.lastRet = payload
	s.mu.Unlock()

	if s.onRet != nil {
		go s.onRet(cloneMap(payload))
	}

	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	_, _ = w.Write([]byte("1"))
}

func (s *Server) handleLast(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	_ = json.NewEncoder(w).Encode(s.Last())
}

func (s *Server) handleTemplate(name, fallback string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		body, err := readTemplate(name)
		if err != nil {
			body = []byte(fallback)
		}
		body = s.injectRuntimeValues(body)
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write(body)
	}
}

func decodePayload(r *http.Request) (map[string]any, error) {
	payload := map[string]any{}
	if r.Method == http.MethodGet {
		for key, values := range r.URL.Query() {
			if len(values) > 0 {
				payload[key] = values[0]
			}
		}
		if len(payload) == 0 {
			return nil, errors.New("missing query payload")
		}
		return payload, nil
	}

	defer r.Body.Close()
	body, err := io.ReadAll(r.Body)
	if err != nil {
		return nil, err
	}
	if len(body) == 0 {
		return nil, errors.New("empty request body")
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil, err
	}
	return payload, nil
}

func readTemplate(name string) ([]byte, error) {
	if data, err := embeddedTemplates.ReadFile("assets/" + name); err == nil {
		return data, nil
	}
	candidates := []string{
		filepath.Join("..", "fresh_unpack", "target.exe_extracted", "templates", name),
		filepath.Join("..", "..", "fresh_unpack", "target.exe_extracted", "templates", name),
		filepath.Join("target.exe_extracted", "templates", name),
	}
	for _, candidate := range candidates {
		data, err := os.ReadFile(candidate)
		if err == nil {
			return data, nil
		}
	}
	return nil, os.ErrNotExist
}

func cloneMap(src map[string]any) map[string]any {
	dst := make(map[string]any, len(src))
	for k, v := range src {
		dst[k] = v
	}
	return dst
}

func (s *Server) injectRuntimeValues(body []byte) []byte {
	addr := s.Addr()
	if strings.TrimSpace(addr) == "" {
		return body
	}

	updated := strings.ReplaceAll(string(body), "http://127.0.0.1:12983/ret", "http://"+addr+"/ret")
	updated = strings.ReplaceAll(updated, "127.0.0.1:12983", addr)
	return []byte(updated)
}

const fallbackIndex = `<!DOCTYPE html>
<html lang="zh-CN">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <title>Captcha</title>
  <style>
    body { margin: 0; background: #0d1117; color: #f0f6fc; font-family: "Segoe UI Variable Text", "Microsoft YaHei UI", sans-serif; }
    iframe { width: 100vw; height: 100vh; border: 0; }
  </style>
  <script>
    window.addEventListener("DOMContentLoaded", () => {
      const iframe = document.getElementById("captcha-frame");
      iframe.src = window.location.href.replace("/?", "/geetest?");
    });
  </script>
</head>
<body>
  <iframe id="captcha-frame" title="captcha"></iframe>
</body>
</html>`

const fallbackGeetest = `<!DOCTYPE html>
<html lang="zh-CN">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <title>Captcha</title>
  <style>
    body { margin: 0; min-height: 100vh; display: grid; place-items: center; background: #0d1117; color: #f0f6fc; font-family: "Segoe UI Variable Text", "Microsoft YaHei UI", sans-serif; }
    main { max-width: 34rem; padding: 2rem; border-radius: 1rem; background: rgba(22, 27, 34, 0.95); box-shadow: 0 18px 40px rgba(0,0,0,0.35); }
    code { display: block; margin-top: 1rem; padding: 0.75rem; border-radius: 0.75rem; background: #010409; overflow-wrap: anywhere; }
  </style>
</head>
<body>
  <main>
    <h1>Captcha 模板缺失</h1>
    <p>当前运行目录没有原始 <code>geetest.html</code> 模板。</p>
    <code>../fresh_unpack/target.exe_extracted/templates/geetest.html</code>
  </main>
</body>
</html>`
