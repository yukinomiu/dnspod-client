package modifier

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"dnspod-ddns-client/internal/config"
	"dnspod-ddns-client/internal/util"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"
)

type (
	Modifier struct {
		cfg        *config.Config
		httpClient *http.Client

		lastPublicIP net.IP
	}
)

func NewModifier(cfg *config.Config) *Modifier {
	return &Modifier{
		cfg: cfg,
		httpClient: &http.Client{
			Timeout: time.Second * 5,
		},
	}
}

func (m *Modifier) Run() {
	// loop
	ticker := time.NewTicker(time.Second * time.Duration(m.cfg.IntervalS))
	defer ticker.Stop()

	if m.cfg.UpdateAtStartup {
		// do once immediately
		slog.Info("update D-DNS record at startup")
		_ = m.update()
	}

	for range ticker.C {
		slog.Debug("update D-DNS record")
		_ = m.update()
	}
}

func (m *Modifier) update() error {
	if !m.cfg.ExternalPublicIPGetter.Enabled {
		return m.modify()
	}

	if m.lastPublicIP == nil {
		// DNS lookup
		domain := strings.Join([]string{m.cfg.SubDomain, m.cfg.Domain}, ".")
		if ips, err := net.LookupIP(domain); err != nil {
			slog.Error(
				"lookup domain IP error",
				slog.String("domain", domain),
				slog.String("error", err.Error()),
			)
		} else {
			if len(ips) > 0 {
				// use the first IP address
				m.lastPublicIP = ips[0]

				slog.Info(
					"lookup domain IP success",
					slog.String("domain", domain),
					slog.String("ip-list", fmt.Sprintf("%v", ips)),
					slog.String("used-ip", m.lastPublicIP.String()),
				)
			}
		}
	}

	if m.lastPublicIP == nil {
		// do update
		m.lastPublicIP, _ = m.getExternalPublicIP()
		return m.modify()
	}

	// compare and update
	if latestPublicIP, err := m.getExternalPublicIP(); err != nil {
		return m.modify()
	} else {
		if latestPublicIP.Equal(m.lastPublicIP) {
			slog.Debug("public IP not changed, skip update")
			return nil
		} else {
			slog.Info("public IP changed, update now")
			m.lastPublicIP = latestPublicIP
			return m.modify()
		}
	}
}

func (m *Modifier) getExternalPublicIP() (net.IP, error) {
	const (
		maxRespBodySize = 1024
	)

	if resp, err := m.httpClient.Get(m.cfg.ExternalPublicIPGetter.URL); err != nil {
		slog.Error("get external public ip error", slog.String("error", err.Error()))
		return nil, err
	} else {
		body := resp.Body
		if body == nil {
			slog.Error("response body is empty")
			return nil, errors.New("response body is empty")
		}
		defer func() {
			if e := body.Close(); e != nil {
				slog.Error("close body error", slog.String("error", e.Error()))
			}
		}()

		if respBytes, maxExceed, e := util.ReadMax(body, maxRespBodySize); e != nil {
			slog.Error("read response body error", slog.String("error", e.Error()))
			return nil, e
		} else {
			if !maxExceed {
				if ip := net.ParseIP(string(respBytes)); ip != nil {
					slog.Info(
						"get public IP response",
						slog.String("status", resp.Status),
						slog.Int("code", resp.StatusCode),
						slog.String("body", string(respBytes)),
					)
					return ip, nil
				} else {
					slog.Error(
						"get public IP bad response",
						slog.String("status", resp.Status),
						slog.Int("code", resp.StatusCode),
						slog.String("body", string(respBytes)),
					)
					return nil, errors.New("bad response")
				}
			} else {
				slog.Warn(
					"get public IP response body too large",
					slog.String("status", resp.Status),
					slog.Int("code", resp.StatusCode),
					slog.Int("max-body-size", maxRespBodySize),
					slog.String("body", string(respBytes)),
				)
				return nil, errors.New("response body too large")
			}
		}
	}
}

func (m *Modifier) modify() error {
	const (
		url = "https://dnspod.tencentcloudapi.com"

		hostHeaderKey = "Host"
		host          = "dnspod.tencentcloudapi.com"

		contentTypeHeaderKey = "Content-Type"
		contentType          = "application/json"

		actionHeaderKey = "X-TC-Action"
		action          = "ModifyDynamicDNS"

		timestampHeaderKey = "X-TC-Timestamp"

		versionHeaderKey = "X-TC-Version"
		version          = "2021-03-23"

		authorizationHeaderKey = "Authorization"
	)

	const (
		maxRespBodySize = 1024 * 4
	)

	var (
		payload     *Request
		payloadJson []byte
		request     *http.Request
	)

	// build request
	payload = &Request{
		Domain:     m.cfg.Domain,
		SubDomain:  m.cfg.SubDomain,
		RecordId:   m.cfg.RecordId,
		RecordLine: m.cfg.RecordLine,
		Ttl:        m.cfg.Ttl,
	}
	if jsonBytes, err := json.Marshal(payload); err != nil {
		slog.Error("marshal request to JSON error", slog.String("error", err.Error()))
		return err
	} else {
		payloadJson = jsonBytes
	}

	if req, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(payloadJson)); err != nil {
		slog.Error("build HTTP request error", slog.String("error", err.Error()))
		return err
	} else {
		if req.Body != nil {
			if e := req.Body.Close(); e != nil {
				slog.Error("close body error", slog.String("error", e.Error()))
				return e
			}
		}

		// header
		utcNow := time.Now().UTC()
		authorization := signedAuthorization(
			http.MethodPost,
			"",
			NewCanonicalHeaders([]*KeyValuePair{
				{Key: hostHeaderKey, Value: host},
				{Key: contentTypeHeaderKey, Value: contentType},
				{Key: actionHeaderKey, Value: action},
			}),
			payloadJson,
			utcNow,
			m.cfg.SecretKey,
			m.cfg.SecretId,
		)

		req.Header.Set(hostHeaderKey, host)
		req.Header.Set(contentTypeHeaderKey, contentType)
		req.Header.Set(timestampHeaderKey, strconv.FormatInt(utcNow.Unix(), 10))
		req.Header.Set(authorizationHeaderKey, authorization)
		req.Header.Set(actionHeaderKey, action)
		req.Header.Set(versionHeaderKey, version)

		request = req
	}

	// send request
	if resp, err := m.httpClient.Do(request); err != nil {
		slog.Error("modify D-DNS HTTP call error", slog.String("error", err.Error()))
		return err
	} else {
		body := resp.Body
		if body == nil {
			slog.Warn("response body is empty")
			return nil
		}
		defer func() {
			if e := body.Close(); e != nil {
				slog.Error("close body error", slog.String("error", e.Error()))
			}
		}()

		if respBytes, maxExceed, e := util.ReadMax(body, maxRespBodySize); e != nil {
			slog.Error("read response body error", slog.String("error", e.Error()))
			return e
		} else {
			if !maxExceed {
				slog.Info(
					"modify response",
					slog.String("status", resp.Status),
					slog.Int("code", resp.StatusCode),
					slog.String("body", string(respBytes)),
				)
			} else {
				slog.Warn(
					"modify response body too large",
					slog.String("status", resp.Status),
					slog.Int("code", resp.StatusCode),
					slog.Int("max-body-size", maxRespBodySize),
					slog.String("body", string(respBytes)),
				)
			}
		}
	}

	return nil
}

func signedAuthorization(
	httpMethod string,
	canonicalQueryString string,
	canonicalHeaders CanonicalHeaders,
	payload []byte,
	utcNow time.Time,
	secretKey string,
	secretId string,
) string {

	const (
		canonicalURI = "/"
		algorithm    = "TC3-HMAC-SHA256"
		service      = "dnspod"
		suffix       = "tc3_request"
	)

	canonicalHeadersStr := canonicalHeaders.ToCanonicalHeaders()
	signedHeadersStr := canonicalHeaders.ToSignedHeaders()
	payloadSum := sha256.Sum256(payload)
	payloadSumSlice := payloadSum[:]

	canonicalRequest :=
		httpMethod + "\n" +
			canonicalURI + "\n" +
			canonicalQueryString + "\n" +
			canonicalHeadersStr + "\n" +
			signedHeadersStr + "\n" +
			strings.ToLower(hex.EncodeToString(payloadSumSlice))

	date := utcNow.Format(time.DateOnly)
	credentialScope := date + "/" + service + "/" + suffix
	canonicalRequestSum := sha256.Sum256([]byte(canonicalRequest))
	canonicalRequestSumSlice := canonicalRequestSum[:]

	stringToSign :=
		algorithm + "\n" +
			strconv.FormatInt(utcNow.Unix(), 10) + "\n" +
			credentialScope + "\n" +
			strings.ToLower(hex.EncodeToString(canonicalRequestSumSlice))

	secretDate := hmacSum256([]byte("TC3"+secretKey), []byte(date))
	secretService := hmacSum256(secretDate, []byte(service))
	secretSigning := hmacSum256(secretService, []byte(suffix))
	signature := strings.ToLower(hex.EncodeToString(hmacSum256(secretSigning, []byte(stringToSign))))

	authorization :=
		algorithm + " " +
			"Credential=" + secretId + "/" + credentialScope + ", " +
			"SignedHeaders=" + signedHeadersStr + ", " +
			"Signature=" + signature

	return authorization
}

func hmacSum256(key []byte, data []byte) []byte {
	hash := hmac.New(sha256.New, key)
	hash.Write(data)
	return hash.Sum(nil)
}
