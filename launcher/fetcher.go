package launcher

import (
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"crypto/tls"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"time"
)

const (
	defaultRequestTimeout = 60 * time.Second
	defaultMaxAttempts    = 3
)

type Fetcher struct {
	client       *http.Client
	allowedHosts []string
	maxAttempts  int
	retryDelay   func(context.Context, time.Duration) error
}

func NewFetcher(allowedHosts []string, baseClient *http.Client) *Fetcher {
	var transport *http.Transport
	if baseClient != nil {
		if existing, ok := baseClient.Transport.(*http.Transport); ok {
			transport = existing.Clone()
		}
	}
	if transport == nil {
		transport = &http.Transport{
			Proxy:                 http.ProxyFromEnvironment,
			DialContext:           (&net.Dialer{Timeout: 10 * time.Second, KeepAlive: 30 * time.Second}).DialContext,
			TLSHandshakeTimeout:   10 * time.Second,
			ResponseHeaderTimeout: 15 * time.Second,
			IdleConnTimeout:       30 * time.Second,
			TLSClientConfig:       &tls.Config{MinVersion: tls.VersionTLS12},
		}
	} else {
		transport.DialContext = (&net.Dialer{Timeout: 10 * time.Second, KeepAlive: 30 * time.Second}).DialContext
		transport.TLSHandshakeTimeout = 10 * time.Second
		transport.ResponseHeaderTimeout = 15 * time.Second
		transport.IdleConnTimeout = 30 * time.Second
		if transport.TLSClientConfig == nil {
			transport.TLSClientConfig = &tls.Config{MinVersion: tls.VersionTLS12}
		} else {
			transport.TLSClientConfig = transport.TLSClientConfig.Clone()
			if transport.TLSClientConfig.MinVersion < tls.VersionTLS12 {
				transport.TLSClientConfig.MinVersion = tls.VersionTLS12
			}
		}
	}
	transport.DisableCompression = true

	client := &http.Client{Transport: transport, Timeout: defaultRequestTimeout}
	if baseClient != nil && baseClient.Timeout > 0 && baseClient.Timeout < client.Timeout {
		client.Timeout = baseClient.Timeout
	}
	client.CheckRedirect = func(request *http.Request, via []*http.Request) error {
		if len(via) >= 5 {
			return fmt.Errorf("too many redirects")
		}
		return ValidateHTTPSURL(request.URL.String(), allowedHosts)
	}

	return &Fetcher{
		client:       client,
		allowedHosts: append([]string(nil), allowedHosts...),
		maxAttempts:  defaultMaxAttempts,
		retryDelay: func(ctx context.Context, delay time.Duration) error {
			timer := time.NewTimer(delay)
			defer timer.Stop()
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-timer.C:
				return nil
			}
		},
	}
}

func (fetcher *Fetcher) FetchManifest(ctx context.Context, rawURL string) (Manifest, error) {
	if err := ValidateHTTPSURL(rawURL, fetcher.allowedHosts); err != nil {
		return Manifest{}, err
	}
	var body []byte
	err := fetcher.retry(ctx, func() error {
		request, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
		if err != nil {
			return err
		}
		request.Header.Set("Accept", "application/json")
		response, err := fetcher.client.Do(request)
		if err != nil {
			return err
		}
		defer response.Body.Close()
		if response.StatusCode != http.StatusOK {
			return &httpStatusError{code: response.StatusCode}
		}
		body, err = io.ReadAll(io.LimitReader(response.Body, maxManifestBytes+1))
		if err != nil {
			return err
		}
		if len(body) > maxManifestBytes {
			return fmt.Errorf("manifest exceeds %d bytes", maxManifestBytes)
		}
		return nil
	})
	if err != nil {
		return Manifest{}, fmt.Errorf("fetch manifest: %w", err)
	}
	return DecodeManifest(bytesReader(body))
}

func (fetcher *Fetcher) DownloadComponent(ctx context.Context, component Component, destination string) error {
	if err := ValidateHTTPSURL(component.URL, fetcher.allowedHosts); err != nil {
		return err
	}
	expectedHash, err := decodeSHA256(component.SHA256)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(destination), 0o700); err != nil {
		return err
	}

	err = fetcher.retry(ctx, func() error {
		temporary, err := os.CreateTemp(filepath.Dir(destination), ".download-*.partial")
		if err != nil {
			return err
		}
		temporaryName := temporary.Name()
		keep := false
		defer func() {
			_ = temporary.Close()
			if !keep {
				_ = os.Remove(temporaryName)
			}
		}()

		request, err := http.NewRequestWithContext(ctx, http.MethodGet, component.URL, nil)
		if err != nil {
			return err
		}
		request.Header.Set("Accept", "application/zip, application/octet-stream")
		response, err := fetcher.client.Do(request)
		if err != nil {
			return err
		}
		defer response.Body.Close()
		if response.StatusCode != http.StatusOK {
			return &httpStatusError{code: response.StatusCode}
		}
		if response.ContentLength > component.Size {
			return fmt.Errorf("component %q response exceeds declared size", component.Name)
		}

		hasher := sha256.New()
		written, err := io.Copy(io.MultiWriter(temporary, hasher), io.LimitReader(response.Body, component.Size+1))
		if err != nil {
			return err
		}
		if written != component.Size {
			return fmt.Errorf("component %q size mismatch: expected %d, received %d", component.Name, component.Size, written)
		}
		actualHash := hasher.Sum(nil)
		if subtle.ConstantTimeCompare(actualHash, expectedHash) != 1 {
			return fmt.Errorf("component %q SHA-256 mismatch", component.Name)
		}
		if err := temporary.Sync(); err != nil {
			return err
		}
		if err := temporary.Close(); err != nil {
			return err
		}
		if err := replaceFile(temporaryName, destination); err != nil {
			return err
		}
		keep = true
		return syncDirectory(filepath.Dir(destination))
	})
	if err != nil {
		return fmt.Errorf("download component %q: %w", component.Name, err)
	}
	return nil
}

func (fetcher *Fetcher) retry(ctx context.Context, operation func() error) error {
	attempts := fetcher.maxAttempts
	if attempts < 1 {
		attempts = 1
	}
	var lastError error
	for attempt := 1; attempt <= attempts; attempt++ {
		if err := ctx.Err(); err != nil {
			return err
		}
		lastError = operation()
		if lastError == nil {
			return nil
		}
		if !isRetryable(lastError) || attempt == attempts {
			break
		}
		if err := fetcher.retryDelay(ctx, time.Duration(attempt)*100*time.Millisecond); err != nil {
			return err
		}
	}
	return lastError
}

type httpStatusError struct{ code int }

func (err *httpStatusError) Error() string { return fmt.Sprintf("HTTP status %d", err.code) }

func isRetryable(err error) bool {
	if status, ok := err.(*httpStatusError); ok {
		return status.code == http.StatusTooManyRequests || status.code >= 500
	}
	// Schema, URL and path validation happen outside retry. Transfer, size and
	// hash failures are retried at most twice to tolerate a truncated CDN edge.
	return true
}

type byteReader struct {
	data []byte
	pos  int
}

func bytesReader(data []byte) *byteReader { return &byteReader{data: data} }

func (reader *byteReader) Read(destination []byte) (int, error) {
	if reader.pos >= len(reader.data) {
		return 0, io.EOF
	}
	count := copy(destination, reader.data[reader.pos:])
	reader.pos += count
	return count, nil
}
