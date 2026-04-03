package bridge

import (
	"context"
	"crypto/ecdh"
	"crypto/ed25519"
	"crypto/hkdf"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/protoadapt"
	"hi3loader/internal/loaderapiv1"
)

func TestLoaderAPIExecuteScanAllowsSlowResponseHeaders(t *testing.T) {
	server, authorityPublicKey := newSecureLoaderAPITestServer(t, func(w http.ResponseWriter, r *http.Request, sessionID string, sessionKey []byte, req *loaderapiv1.ScanExecuteRequest) {
		time.Sleep(120 * time.Millisecond)
		writeSealedTestResponse(t, w, sessionID, sessionKey, r.URL.Path, &loaderapiv1.ScanExecuteResponse{
			RequestID: "req-1",
			Retcode:   0,
			Message:   "ok",
			Confirmed: true,
		})
	})
	defer server.Close()

	restoreAuthority := SetEmbeddedServerAuthorityForTesting(authorityPublicKey)
	defer restoreAuthority()

	client := newLoaderAPIClient(server.URL)

	resp, err := client.ExecuteScan(context.Background(), ScanRequest{
		Ticket:       "ticket",
		UID:          1001,
		AccessKey:    "access-key",
		AsteriskName: "asterisk",
		ClientMeta:   ClientMetaForBaseURL(server.URL),
	})
	if err != nil {
		t.Fatalf("ExecuteScan() error = %v", err)
	}
	if !resp.Confirmed {
		t.Fatalf("expected confirmed response, got %+v", resp)
	}
}

func TestNewLoaderAPIHTTPClientAppliesProbeHeaderTimeout(t *testing.T) {
	client := newLoaderAPIHTTPClient(5*time.Second, 5*time.Second, nil)
	transport, ok := client.Transport.(*http.Transport)
	if !ok {
		t.Fatalf("expected *http.Transport, got %T", client.Transport)
	}
	if transport.ResponseHeaderTimeout != 5*time.Second {
		t.Fatalf("expected ResponseHeaderTimeout=5s, got %v", transport.ResponseHeaderTimeout)
	}
}

func TestProbeLoaderAPIReusesManifestAndSession(t *testing.T) {
	var (
		mu            sync.Mutex
		manifestHits  int
		handshakeHits int
		healthzHits   int
	)
	server, authorityPublicKey := newSecureLoaderAPITestServer(t, func(w http.ResponseWriter, r *http.Request, sessionID string, sessionKey []byte, req *loaderapiv1.ScanExecuteRequest) {
		writeSealedTestResponse(t, w, sessionID, sessionKey, r.URL.Path, &loaderapiv1.ScanExecuteResponse{
			RequestID: "req-cache",
			Retcode:   0,
			Message:   "ok",
			Confirmed: true,
		})
	})
	defer server.Close()

	restoreAuthority := SetEmbeddedServerAuthorityForTesting(authorityPublicKey)
	defer restoreAuthority()

	originalCache := loaderAPICache
	loaderAPICache = struct {
		mu        sync.Mutex
		manifests map[string]loaderAPIManifestCacheEntry
		sessions  map[string]loaderAPISessionCacheEntry
	}{
		manifests: map[string]loaderAPIManifestCacheEntry{},
		sessions:  map[string]loaderAPISessionCacheEntry{},
	}
	defer func() {
		loaderAPICache = originalCache
	}()

	testServer := server.Config.Handler
	server.Config.Handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		switch r.URL.Path {
		case "/v1/session/manifest":
			manifestHits++
		case "/v1/session/handshake":
			handshakeHits++
		case "/v1/healthz":
			healthzHits++
		}
		mu.Unlock()
		testServer.ServeHTTP(w, r)
	})

	meta := ClientMetaForBaseURL(server.URL)
	if err := ProbeLoaderAPI(context.Background(), server.URL, meta); err != nil {
		t.Fatalf("first ProbeLoaderAPI() error = %v", err)
	}
	if err := ProbeLoaderAPI(context.Background(), server.URL, meta); err != nil {
		t.Fatalf("second ProbeLoaderAPI() error = %v", err)
	}

	mu.Lock()
	defer mu.Unlock()
	if manifestHits != 1 {
		t.Fatalf("expected 1 manifest request, got %d", manifestHits)
	}
	if handshakeHits != 1 {
		t.Fatalf("expected 1 handshake request, got %d", handshakeHits)
	}
	if healthzHits != 2 {
		t.Fatalf("expected 2 healthz requests, got %d", healthzHits)
	}
}

func newSecureLoaderAPITestServer(t *testing.T, onScan func(http.ResponseWriter, *http.Request, string, []byte, *loaderapiv1.ScanExecuteRequest)) (*httptest.Server, ed25519.PublicKey) {
	t.Helper()

	curve := ecdh.X25519()
	authorityPublicKey, authorityPrivateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("generate authority key: %v", err)
	}

	var (
		mu       sync.Mutex
		sessions = map[string][]byte{}
		server   *httptest.Server
	)

	server = httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v1/session/manifest":
			req := &loaderapiv1.ManifestRequest{}
			if err := decodeTestProtoRequest(r, req); err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			fingerprint := sha256Hex(server.Certificate().Raw)
			generatedAt := time.Now().Format(time.RFC3339)
			payload := manifestSignaturePayload("test", fingerprint, loaderapiv1.ProtocolValue, "test", generatedAt)
			signature := ed25519.Sign(authorityPrivateKey, payload)
			writeTestProtoResponse(t, w, &loaderapiv1.ManifestResponse{
				KeyId:           "test",
				TlsCertSha256:   fingerprint,
				ProtocolVersion: loaderapiv1.ProtocolValue,
				ServerName:      "test",
				GeneratedAt:     generatedAt,
				Signature:       signature,
			})
		case "/v1/session/handshake":
			req := &loaderapiv1.HandshakeRequest{}
			if err := decodeTestProtoRequest(r, req); err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			clientPublicKey, err := curve.NewPublicKey(req.ClientPublicKey)
			if err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			serverKey, err := curve.GenerateKey(rand.Reader)
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			serverNonce, err := randomBytes(loaderAPIHandshakeNonceSize)
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			sharedSecret, err := serverKey.ECDH(clientPublicKey)
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			sessionKey, err := hkdf.Key(sha256.New, sharedSecret, append(append([]byte(nil), req.ClientNonce...), serverNonce...), loaderAPIProtocolInfo, loaderAPISessionKeySize)
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			sessionID := "sess-" + sha256Hex(serverNonce)[:12]
			mu.Lock()
			sessions[sessionID] = sessionKey
			mu.Unlock()

			writeTestProtoResponse(t, w, &loaderapiv1.HandshakeResponse{
				SessionID:            sessionID,
				ServerPublicKey:      serverKey.PublicKey().Bytes(),
				ServerNonce:          serverNonce,
				ProtocolVersion:      loaderapiv1.ProtocolValue,
				ServerVersion:        "test",
				Message:              "OK",
				SessionExpiresUnixMs: time.Now().Add(30 * time.Second).UnixMilli(),
			})
		case "/v1/healthz":
			wire := &loaderapiv1.SealedRequest{}
			if err := decodeTestProtoRequest(r, wire); err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			mu.Lock()
			sessionKey := append([]byte(nil), sessions[wire.SessionID]...)
			mu.Unlock()
			if len(sessionKey) == 0 {
				http.Error(w, "missing session", http.StatusUnauthorized)
				return
			}
			plaintext, err := openSessionPayload(sessionKey, requestAAD(r.URL.Path, wire.SessionID), wire.Nonce, wire.Ciphertext)
			if err != nil {
				http.Error(w, err.Error(), http.StatusUnauthorized)
				return
			}
			req := &loaderapiv1.HealthzRequest{}
			if err := proto.Unmarshal(plaintext, protoadapt.MessageV2Of(req)); err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			writeSealedTestResponse(t, w, wire.SessionID, sessionKey, r.URL.Path, &loaderapiv1.HealthzResponse{
				Ok:              true,
				ServerName:      "test",
				ServerVersion:   "test",
				ProtocolVersion: loaderapiv1.ProtocolValue,
				Message:         "OK",
			})
		case "/v1/scan/execute":
			wire := &loaderapiv1.SealedRequest{}
			if err := decodeTestProtoRequest(r, wire); err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			mu.Lock()
			sessionKey := append([]byte(nil), sessions[wire.SessionID]...)
			mu.Unlock()
			if len(sessionKey) == 0 {
				http.Error(w, "missing session", http.StatusUnauthorized)
				return
			}
			plaintext, err := openSessionPayload(sessionKey, requestAAD(r.URL.Path, wire.SessionID), wire.Nonce, wire.Ciphertext)
			if err != nil {
				http.Error(w, err.Error(), http.StatusUnauthorized)
				return
			}
			req := &loaderapiv1.ScanExecuteRequest{}
			if err := proto.Unmarshal(plaintext, protoadapt.MessageV2Of(req)); err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			onScan(w, r, wire.SessionID, sessionKey, req)
		default:
			http.NotFound(w, r)
		}
	}))

	return server, authorityPublicKey
}

func writeSealedTestResponse(t *testing.T, w http.ResponseWriter, sessionID string, sessionKey []byte, path string, msg protoadapt.MessageV1) {
	t.Helper()
	raw, err := proto.Marshal(protoadapt.MessageV2Of(msg))
	if err != nil {
		t.Fatalf("marshal sealed response: %v", err)
	}
	nonce, err := randomBytes(loaderAPINonceSize)
	if err != nil {
		t.Fatalf("nonce: %v", err)
	}
	ciphertext, err := sealSessionPayload(sessionKey, responseAAD(path, sessionID), nonce, raw)
	if err != nil {
		t.Fatalf("seal response: %v", err)
	}
	writeTestProtoResponse(t, w, &loaderapiv1.SealedResponse{Nonce: nonce, Ciphertext: ciphertext})
}

func decodeTestProtoRequest(r *http.Request, out protoadapt.MessageV1) error {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		return err
	}
	return proto.Unmarshal(body, protoadapt.MessageV2Of(out))
}

func writeTestProtoResponse(t *testing.T, w http.ResponseWriter, msg protoadapt.MessageV1) {
	t.Helper()
	raw, err := proto.Marshal(protoadapt.MessageV2Of(msg))
	if err != nil {
		t.Fatalf("marshal response: %v", err)
	}
	w.Header().Set("Content-Type", loaderapiv1.ContentType)
	_, _ = w.Write(raw)
}

func sha256Hex(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}
