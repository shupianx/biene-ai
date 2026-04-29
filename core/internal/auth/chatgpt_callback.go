package auth

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"sync"
	"time"
)

// CallbackResult captures what the OAuth provider redirected to our
// localhost listener. The state value lets the consumer match this
// against the originating PreparedFlow; missing code with a non-empty
// error means the user denied (or OpenAI rejected) the request.
type CallbackResult struct {
	Code  string
	State string
	Error string
}

// callbackPaths is the set of URL paths the listener accepts. Codex CLI
// registered exactly /auth/callback, but we accept the alternate forms
// other reference implementations use to keep the listener resilient if
// OpenAI ever expands the registration.
var callbackPaths = map[string]struct{}{
	"/auth/callback":  {},
	"/oauth/callback": {},
	"/callback":       {},
}

// ChatGPTCallbackListener is a one-shot HTTP server bound to the fixed
// localhost:1455 port. It resolves once the user's browser hits the
// redirect URL; the server is shut down regardless of outcome.
//
// We don't share this listener with the main Biene HTTP server because
// the redirect_uri OpenAI registered for the Codex CLI public client is
// fixed at port 1455. The Biene server runs on a dynamic port chosen
// by the Electron main process at startup.
type ChatGPTCallbackListener struct {
	srv     *http.Server
	result  chan CallbackResult
	once    sync.Once
	closeFn func() error
}

// StartChatGPTCallback binds to localhost:1455 and returns a listener
// whose Wait() resolves on the first request to a callback path. If the
// port is already in use (Codex CLI also running, prior process held
// over, etc.), the error from net.Listen surfaces directly.
func StartChatGPTCallback() (*ChatGPTCallbackListener, error) {
	addr := fmt.Sprintf("127.0.0.1:%d", chatgptCallbackPort)
	l, err := net.Listen("tcp", addr)
	if err != nil {
		return nil, fmt.Errorf("bind chatgpt callback (%s): %w", addr, err)
	}

	cb := &ChatGPTCallbackListener{
		result: make(chan CallbackResult, 1),
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/", cb.handle)

	cb.srv = &http.Server{
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
	}
	cb.closeFn = func() error {
		return l.Close()
	}

	go func() {
		// Serve returns ErrServerClosed when Close() is called from
		// handle(); other errors propagate via the result channel as
		// a synthetic "error" callback.
		err := cb.srv.Serve(l)
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			cb.deliver(CallbackResult{Error: "callback listener stopped: " + err.Error()})
		}
	}()

	return cb, nil
}

func (c *ChatGPTCallbackListener) handle(w http.ResponseWriter, r *http.Request) {
	if _, ok := callbackPaths[r.URL.Path]; !ok {
		http.NotFound(w, r)
		return
	}
	q := r.URL.Query()
	res := CallbackResult{
		Code:  q.Get("code"),
		State: q.Get("state"),
		Error: q.Get("error_description"),
	}
	if res.Error == "" {
		res.Error = q.Get("error")
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	if res.Code != "" && res.Error == "" {
		_, _ = w.Write([]byte(callbackSuccessHTML))
	} else {
		_, _ = w.Write([]byte(callbackFailureHTML))
	}

	c.deliver(res)
	// Schedule shutdown after the response flushes.
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		_ = c.srv.Shutdown(ctx)
	}()
}

func (c *ChatGPTCallbackListener) deliver(res CallbackResult) {
	c.once.Do(func() {
		c.result <- res
		close(c.result)
	})
}

// Wait blocks until the listener receives a callback or the context is
// cancelled. The listener is automatically closed in either case.
func (c *ChatGPTCallbackListener) Wait(ctx context.Context) (CallbackResult, error) {
	defer c.Close()
	select {
	case <-ctx.Done():
		return CallbackResult{}, ctx.Err()
	case res, ok := <-c.result:
		if !ok {
			return CallbackResult{}, errors.New("callback listener closed without delivering a result")
		}
		return res, nil
	}
}

// Close shuts down the listener. Safe to call multiple times.
func (c *ChatGPTCallbackListener) Close() {
	if c.closeFn != nil {
		_ = c.closeFn()
		c.closeFn = nil
	}
	if c.srv != nil {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		_ = c.srv.Shutdown(ctx)
		cancel()
	}
}

const callbackSuccessHTML = `<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="utf-8">
<title>Biene · ChatGPT login</title>
<style>
body{font-family:-apple-system,BlinkMacSystemFont,Segoe UI,Roboto,sans-serif;display:flex;align-items:center;justify-content:center;height:100vh;margin:0;background:#fafafa;color:#222}
.card{background:#fff;border:1px solid #e5e5e5;border-radius:12px;padding:32px 40px;box-shadow:0 2px 12px rgba(0,0,0,0.04);text-align:center;max-width:420px}
h1{margin:0 0 8px;font-size:20px}
p{margin:0;color:#666;font-size:14px;line-height:1.5}
</style>
</head>
<body>
<div class="card">
<h1>Login complete</h1>
<p>You can return to Biene. This tab can be closed.</p>
</div>
</body>
</html>`

const callbackFailureHTML = `<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="utf-8">
<title>Biene · login failed</title>
<style>
body{font-family:-apple-system,BlinkMacSystemFont,Segoe UI,Roboto,sans-serif;display:flex;align-items:center;justify-content:center;height:100vh;margin:0;background:#fafafa;color:#222}
.card{background:#fff;border:1px solid #e5e5e5;border-radius:12px;padding:32px 40px;box-shadow:0 2px 12px rgba(0,0,0,0.04);text-align:center;max-width:420px}
h1{margin:0 0 8px;font-size:20px;color:#b00020}
p{margin:0;color:#666;font-size:14px;line-height:1.5}
</style>
</head>
<body>
<div class="card">
<h1>Login failed</h1>
<p>Return to Biene to see the error and try again.</p>
</div>
</body>
</html>`
