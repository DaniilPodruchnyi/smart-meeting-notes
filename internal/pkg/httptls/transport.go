package httptls

import (
	"crypto/tls"
	"net/http"
)

// NewTransport возвращает *http.Transport с настройкой TLS для исходящих HTTPS.
// insecureSkipVerify — только для отладки; в проде используйте false (по умолчанию).
func NewTransport(insecureSkipVerify bool) *http.Transport {
	tr := http.DefaultTransport.(*http.Transport).Clone()
	if tr.TLSClientConfig == nil {
		tr.TLSClientConfig = &tls.Config{}
	} else {
		tr.TLSClientConfig = tr.TLSClientConfig.Clone()
	}
	tr.TLSClientConfig.MinVersion = tls.VersionTLS12
	tr.TLSClientConfig.InsecureSkipVerify = insecureSkipVerify
	return tr
}
