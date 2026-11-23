// Package engine owns a private, bundled local mail parser process.
package engine

import (
	"bufio"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"
)

type Engine struct {
	URL    string `json:"url"`
	Token  string `json:"token"`
	key    string
	cmd    *exec.Cmd
	cancel context.CancelFunc
	Client *http.Client
}

func Start(resources string) (*Engine, error) {
	python := filepath.Join(resources, "runtime", "python", "bin", "python3")
	if runtime.GOOS == "windows" {
		python = filepath.Join(resources, "runtime", "python.exe")
	}
	for _, path := range []string{python, filepath.Join(resources, "app", "app.py")} {
		if _, err := os.Stat(path); err != nil {
			return nil, errors.New("Postbox resources are missing. Extract the complete download before opening Postbox.")
		}
	}
	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		return nil, err
	}
	ctx, cancel := context.WithCancel(context.Background())
	e := &Engine{key: hex.EncodeToString(key), cancel: cancel}
	e.cmd = exec.CommandContext(ctx, python, "-I", "-B", filepath.Join(resources, "app", "app.py"), "--desktop", "--no-open")
	hideConsole(e.cmd)
	e.cmd.Stdin = strings.NewReader(e.key + "\n")
	// Do not inherit a console or write message contents/tracebacks to logs.
	e.cmd.Stderr = io.Discard
	stdout, err := e.cmd.StdoutPipe()
	if err != nil {
		cancel()
		return nil, err
	}
	if err = e.cmd.Start(); err != nil {
		cancel()
		return nil, err
	}
	ready := make(chan error, 1)
	go func() {
		line, err := bufio.NewReader(stdout).ReadBytes('\n')
		if err == nil {
			err = json.Unmarshal(line, e)
		}
		ready <- err
	}()
	select {
	case err = <-ready:
	case <-time.After(30 * time.Second):
		err = errors.New("The bundled mail engine did not start. Re-extract Postbox and try again.")
	}
	if err != nil {
		e.Close()
		return nil, err
	}
	u, err := url.Parse(e.URL)
	if err != nil || u.Scheme != "http" || u.Hostname() != "127.0.0.1" || len(e.Token) < 24 {
		e.Close()
		return nil, errors.New("Invalid local mail engine response")
	}
	// Never honour HTTP proxy environment variables or follow redirects.
	e.Client = &http.Client{Timeout: 10 * time.Minute, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }, Transport: &http.Transport{
		Proxy: nil, MaxConnsPerHost: 4, ResponseHeaderTimeout: 10 * time.Minute,
		DialContext: func(ctx context.Context, network, address string) (net.Conn, error) {
			if address != u.Host {
				return nil, errors.New("Only the private mail engine is allowed")
			}
			return (&net.Dialer{Timeout: 5 * time.Second}).DialContext(ctx, network, address)
		},
	}}
	return e, nil
}

func (e *Engine) Close() {
	if e.cancel != nil {
		e.cancel()
	}
	if e.cmd != nil {
		_ = e.cmd.Wait()
	}
}
func (e *Engine) Request(ctx context.Context, method, path string, body io.Reader, headers http.Header) (*http.Response, error) {
	if !strings.HasPrefix(path, "/") || strings.HasPrefix(path, "//") || strings.ContainsAny(path, "\r\n") {
		return nil, errors.New("Invalid engine path")
	}
	req, err := http.NewRequestWithContext(ctx, method, e.URL+path, body)
	if err != nil {
		return nil, err
	}
	if headers != nil {
		req.Header = headers.Clone()
		if n, err := strconv.ParseInt(headers.Get("Content-Length"), 10, 64); err == nil {
			req.ContentLength = n
		}
	}
	if file, ok := body.(*os.File); ok {
		info, err := file.Stat()
		if err != nil {
			return nil, err
		}
		req.ContentLength = info.Size()
	}
	req.Header.Set("X-Viewer-Token", e.Token)
	req.Header.Set("X-Postbox-Desktop", e.key)
	req.Header.Del("Origin")
	req.Header.Del("Sec-Fetch-Site")
	return e.Client.Do(req)
}
func (e *Engine) JSON(ctx context.Context, path string, payload any, result any) error {
	raw, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	res, err := e.Request(ctx, "POST", path, strings.NewReader(string(raw)), http.Header{"Content-Type": {"application/json"}})
	if err != nil {
		return err
	}
	defer res.Body.Close()
	if res.StatusCode != 200 {
		var v struct {
			Error string `json:"error"`
		}
		_ = json.NewDecoder(res.Body).Decode(&v)
		if v.Error == "" {
			v.Error = fmt.Sprintf("Mail engine request failed (%d)", res.StatusCode)
		}
		return errors.New(v.Error)
	}
	return json.NewDecoder(res.Body).Decode(result)
}
