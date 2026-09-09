package update

import (
	"net/http"
	"net/url"
	"sort"
	"strings"
)

//go:generate mockgen -destination=mock_$GOPACKAGE/$GOFILE . DebugLogger

type DebugLogger interface {
	Debug(s string)
}

func makeLogClient(client *http.Client, logger DebugLogger) *http.Client {
	transport := client.Transport
	if transport == nil {
		transport = http.DefaultTransport
	}

	client.Transport = &loggingRoundTripper{
		proxied: transport,
		logger:  logger,
	}

	return client
}

type loggingRoundTripper struct {
	proxied http.RoundTripper
	logger  DebugLogger
}

const redactedValue = "[REDACTED]"

func (lrt *loggingRoundTripper) RoundTrip(request *http.Request) (
	response *http.Response, err error,
) {
	lrt.logger.Debug(requestToString(request))

	response, err = lrt.proxied.RoundTrip(request)
	if err != nil {
		return response, err
	}

	lrt.logger.Debug(responseToString(response))

	return response, nil
}

func requestToString(request *http.Request) (s string) {
	s = request.Method + " " + redactURL(request.URL)

	if request.Header != nil {
		s += " | headers: " + headerToString(request.Header)
	}

	// Request bodies can contain provider credentials in arbitrary formats.
	// Do not serialize them into logs.
	if request.Body != nil && request.Body != http.NoBody {
		s += " | body: " + redactedValue
	}

	return s
}

func responseToString(response *http.Response) (s string) {
	s = response.Status

	if response.Header != nil {
		s += " | headers: " + headerToString(response.Header)
	}

	// Response bodies may echo request credentials or session material.
	if response.Body != nil && response.Body != http.NoBody {
		s += " | body: " + redactedValue
	}

	return s
}

func headerToString(header http.Header) (s string) {
	keys := make([]string, 0, len(header))
	for key := range header {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	headers := make([]string, 0, len(keys))
	for _, key := range keys {
		values := header.Values(key)
		if isSensitiveName(key) {
			values = []string{redactedValue}
		}
		headerString := key + ": " + strings.Join(values, ",")
		headers = append(headers, headerString)
	}
	return strings.Join(headers, "; ")
}

func redactURL(input *url.URL) string {
	if input == nil {
		return ""
	}

	redacted := *input
	if redacted.User != nil {
		if _, hasPassword := redacted.User.Password(); hasPassword {
			redacted.User = url.UserPassword(redactedValue, redactedValue)
		} else {
			redacted.User = url.User(redactedValue)
		}
	}

	query := redacted.Query()
	for key := range query {
		if isSensitiveName(key) {
			query.Set(key, redactedValue)
		}
	}
	redacted.RawQuery = query.Encode()

	return redacted.String()
}

func isSensitiveName(name string) bool {
	normalized := strings.ToLower(name)
	replacer := strings.NewReplacer("-", "", "_", "", ".", "")
	normalized = replacer.Replace(normalized)

	switch normalized {
	case "authorization", "proxyauthorization",
		"cookie", "setcookie",
		"token", "accesstoken", "refreshtoken", "idtoken",
		"password", "passwd",
		"secret", "clientsecret", "consumersecret",
		"apikey", "key", "consumerkey",
		"appsecret", "appkey",
		"xapikey", "xauthtoken",
		"auth", "credential", "credentials":
		return true
	default:
		return strings.HasSuffix(normalized, "token") ||
			strings.HasSuffix(normalized, "secret") ||
			strings.HasSuffix(normalized, "password")
	}
}
