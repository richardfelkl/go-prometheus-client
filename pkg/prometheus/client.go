package prometheus

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/pkg/errors"
	"go.uber.org/zap"
)

// Option functional option for ManblockExternalSvcServer methods
type Option func(*Client)

// WithLogger sets Client logger
func WithLogger(logger *zap.Logger) Option {
	return func(args *Client) {
		args.logger = logger
	}
}

// WithTimeout sets Client logger
func WithTimeout(timeout time.Duration) Option {
	return func(args *Client) {
		args.timeout = timeout
	}
}

// Client Prometheus client struct
type Client struct {
	logger   *zap.Logger
	protocol string
	address  string
	port     string
	timeout  time.Duration
}

// NewClient creates new Client instance
func NewClient(protocol, address, port string, opts ...Option) *Client {
	client := &Client{
		protocol: protocol,
		address:  address,
		port:     port,
		logger:   zap.NewExample(),
	}

	for _, opt := range opts {
		opt(client)
	}

	return client
}

// QueryRequest Prometheus query returns scalar value
// param: query - Prometheus query string
// result: []byte - contains JSON marshalled type *json.RawMessage
// result: string - contains parsed 'resultType' field from response
func (m *Client) QueryRequest(query string) ([]byte, string, error) {
	prometheusRequest := fmt.Sprintf("%v://%v:%v/api/v1/query?query=%v",
		m.protocol, m.address, m.port, query)

	m.logger.Debug("Prometheus request", zap.String("query", prometheusRequest))

	resp, resultType, err := m.query(prometheusRequest)
	if err != nil {
		return nil, "", errors.Wrapf(err, "%v: reading response body failed", funcInfo())
	}

	return resp, resultType, nil
}

// QueryRangeRequest Prometheus query range returns matrix values
// param: query - Prometheus query string
// param: start - start time of range interval
// param: end   - end time of range interval
// param: step  - sampling interval
// result: []byte - contains JSON marshalled type *json.RawMessage
// result: string - contains parsed 'resultType' field from response
func (m *Client) QueryRangeRequest(query string, start, end time.Time, step time.Duration) ([]byte, string, error) {
	prometheusRequest := fmt.Sprintf("%v://%v:%v/api/v1/query_range?query=%v&start=%v&end=%v&step=%v",
		m.protocol, m.address, m.port, query, start.Format(time.RFC3339Nano), end.Format(time.RFC3339Nano), shortDur(step))

	m.logger.Debug("Prometheus request", zap.String("query", prometheusRequest))

	resp, resultType, err := m.query(prometheusRequest)
	if err != nil {
		return nil, "", errors.Wrapf(err, "%v: reading response body failed", funcInfo())
	}

	return resp, resultType, nil
}

func (m *Client) query(query string) ([]byte, string, error) {
	http.DefaultClient.Timeout = m.timeout

	resp, err := http.DefaultClient.Get(query)
	if err != nil {
		return nil, "", errors.Wrapf(err, "%v: getting result from Prometheus failed", funcInfo())
	}

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, "", errors.Wrapf(err, "%v: reading response body failed", funcInfo())
	}

	m.logger.Debug("Prometheus response", zap.String("result", string(body)))

	response, resultType, err := m.parseResponse(body)
	if err != nil {
		return nil, "", errors.Wrapf(err, "%v: parsing response failed", funcInfo())
	}

	return response, resultType, nil
}

func (m *Client) parseResponse(data []byte) ([]byte, string, error) {
	var err error
	var objmap map[string]*json.RawMessage
	var objlist []*json.RawMessage
	var resultType string

	err = json.Unmarshal(data, &objmap)
	if err != nil {
		return nil, "", errors.Wrapf(err, "%v: response unmarshal failed", funcInfo())
	}

	dataObj, ok := objmap["data"]
	if !ok {
		return nil, "", errors.Errorf("%v: Data parsing failed", funcInfo())
	}

	err = json.Unmarshal([]byte(*dataObj), &objmap)
	if err != nil {
		return nil, "", errors.Wrapf(err, "%v: data unmarshal failed", funcInfo())
	}

	resultObj, ok := objmap["result"]
	if !ok {
		return nil, "", errors.Errorf("%v: Result parsing failed", funcInfo())
	}
	err = json.Unmarshal([]byte(*resultObj), &objlist)
	if err != nil {
		return nil, "", errors.Wrapf(err, "%v: result unmarshal failed", funcInfo())
	}

	resultTypeObj, ok := objmap["resultType"]
	if !ok {
		return nil, "", errors.Errorf("%v: Result parsing failed", funcInfo())
	}
	err = json.Unmarshal([]byte(*resultTypeObj), &resultType)
	if err != nil {
		return nil, "", errors.Wrapf(err, "%v: result type unmarshal failed", funcInfo())
	}

	if len(objlist) == 0 {
		return nil, "", errors.Errorf("%v: Result is empty", funcInfo())
	}

	resp, err := json.Marshal(objlist)
	if err != nil {
		return nil, "", errors.Wrapf(err, "%v: response marshal failed", funcInfo())
	}

	return resp, resultType, nil
}

func shortDur(d time.Duration) string {
	s := d.String()
	if strings.HasSuffix(s, "m0s") {
		s = s[:len(s)-2]
	}
	if strings.HasSuffix(s, "h0m") {
		s = s[:len(s)-2]
	}
	return s
}

func funcInfo() string {
	_, file, line, _ := runtime.Caller(1)
	return filepath.Base(file) + ":" + strconv.Itoa(line)
}
