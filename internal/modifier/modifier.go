package modifier

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"dnspod-ddns-client/internal/config"
	"dnspod-ddns-client/internal/util"
	"encoding/hex"
	"encoding/json"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"
)

func Run(cfg *config.Config) {
	const (
		defaultIntervalS = 600
	)

	// build HTTP client
	httpClient := &http.Client{
		Timeout: time.Second * 5,
	}

	// loop
	intervalS := cfg.IntervalS
	if intervalS <= 0 {
		intervalS = defaultIntervalS
		slog.Warn(
			"bad interval config, use default instead",
			slog.Int("config-value", cfg.IntervalS),
			slog.Int("default", defaultIntervalS),
			slog.Int("used-value", intervalS),
		)
	}
	ticker := time.NewTicker(time.Second * time.Duration(intervalS))
	defer ticker.Stop()

	if cfg.ModifyAtStartup {
		// do once immediately
		slog.Info("modify D-DNS record at startup")
		_ = modify(cfg, httpClient)
	}

	for range ticker.C {
		slog.Info("modify D-DNS record")
		_ = modify(cfg, httpClient)
	}
}

func modify(cfg *config.Config, httpClient *http.Client) error {
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
		Domain:     cfg.Domain,
		SubDomain:  cfg.SubDomain,
		RecordId:   cfg.RecordId,
		RecordLine: cfg.RecordLine,
		Ttl:        cfg.Ttl,
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
			cfg.SecretKey,
			cfg.SecretId,
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
	if resp, err := httpClient.Do(request); err != nil {
		slog.Error("http call error", slog.String("error", err.Error()))
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
					"response",
					slog.String("status", resp.Status),
					slog.Int("code", resp.StatusCode),
					slog.String("body", string(respBytes)),
				)
			} else {
				slog.Warn(
					"response body too large",
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
