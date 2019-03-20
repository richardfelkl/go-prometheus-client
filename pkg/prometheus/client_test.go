package prometheus

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"reflect"
	"strconv"
	"strings"
	"testing"
	"time"

	"go.uber.org/zap"

	"github.com/gorilla/mux"
)

var (
	unicornResponse      = []byte(`{"data":{"resultType":"vector","result":[{"value":[1.1, "1"]}]}}`)
	dataFailResponse     = []byte("{}")
	resultFailResponse   = []byte(`{"data":{}}`)
	resultFailResponse2  = []byte(`{"data":[]}`)
	resultFailResponse3  = []byte("{\"data\":{\"result\":{}}}")
	resultFailResponse4  = []byte("{\"data\":{\"result\":[],\"resultType\":\"\"}}")
	resultsFailResponse  = []byte("{\"data\":{\"result\":[]}}")
	resultsFailResponse2 = []byte("{\"data\":{\"result\":[],\"resultType\":[]}}")

	unicornHandler = func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, string(unicornResponse))
	}
	dataFailhandler = func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, string(dataFailResponse))
	}
	timeoutHandler = func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(time.Millisecond * 20)
	}
)

func startHTTPServer(path, port string, handler func(w http.ResponseWriter, r *http.Request)) *http.Server {
	router := mux.NewRouter()

	router.HandleFunc(path, handler)

	srv := &http.Server{Addr: ":" + port, Handler: router}

	go func() {
		if err := srv.ListenAndServe(); err != http.ErrServerClosed {
			log.Fatalf("ListenAndServe(): %s", err)
		}
	}()

	return srv
}

func TestNewClient(t *testing.T) {
	logger := zap.NewExample()
	type args struct {
		protocol string
		address  string
		port     string
		opts     []Option
	}
	tests := []struct {
		name string
		args args
		want *Client
	}{
		{
			name: "Test NewClient unicorn path",
			args: args{protocol: "http", address: "127.0.0.1", port: "9090", opts: []Option{WithLogger(logger), WithTimeout(time.Second * 30)}},
			want: &Client{protocol: "http", address: "127.0.0.1", port: "9090", logger: logger, timeout: time.Second * 30},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := NewClient(tt.args.protocol, tt.args.address, tt.args.port, tt.args.opts...); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("NewClient() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestClient_QueryRequest(t *testing.T) {
	logger := zap.NewExample(zap.Development())

	type args struct {
		query   string
		handler func(w http.ResponseWriter, r *http.Request)
	}
	tests := []struct {
		name    string
		m       *Client
		args    args
		want    []byte
		want1   string
		wantErr bool
	}{
		{
			name:  "Test QueryRangeRequest unicorn path",
			m:     &Client{protocol: "http", address: "127.0.0.1", port: "9090", logger: logger, timeout: time.Second * 30},
			args:  args{query: "QUERY", handler: unicornHandler},
			want:  []byte(`[{"value":[1.1,"1"]}]`),
			want1: "vector",
		},
		{
			name:    "Test QueryRangeRequest data fail",
			m:       &Client{protocol: "http", address: "127.0.0.1", port: "9090", logger: logger, timeout: time.Second * 30},
			args:    args{query: "QUERY", handler: dataFailhandler},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		httpServer := startHTTPServer("/api/v1/query", "9090", tt.args.handler)
		t.Run(tt.name, func(t *testing.T) {
			got, got1, err := tt.m.QueryRequest(tt.args.query)
			if (err != nil) != tt.wantErr {
				t.Errorf("Client.QueryRequest() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Client.QueryRequest() got = %v, want %v", got, tt.want)
			}
			if got1 != tt.want1 {
				t.Errorf("Client.QueryRequest() got1 = %v, want %v", got1, tt.want1)
			}
		})
		httpServer.Shutdown(context.Background())
	}
}

func TestClient_QueryRangeRequest(t *testing.T) {
	logger := zap.NewExample(zap.Development())

	type args struct {
		query   string
		handler func(w http.ResponseWriter, r *http.Request)
		start   time.Time
		end     time.Time
		step    time.Duration
	}
	tests := []struct {
		name    string
		m       *Client
		args    args
		want    []byte
		want1   string
		wantErr bool
	}{
		{
			name:  "Test QueryRangeRequest unicorn path",
			m:     &Client{protocol: "http", address: "127.0.0.1", port: "9090", logger: logger, timeout: time.Second * 30},
			args:  args{query: "QUERY", handler: unicornHandler},
			want:  []byte(`[{"value":[1.1,"1"]}]`),
			want1: "vector",
		},
		{
			name:    "Test QueryRangeRequest data fail",
			m:       &Client{protocol: "http", address: "127.0.0.1", port: "9090", logger: logger, timeout: time.Second * 30},
			args:    args{query: "QUERY", handler: dataFailhandler},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		httpServer := startHTTPServer("/api/v1/query_range", "9090", tt.args.handler)

		t.Run(tt.name, func(t *testing.T) {
			got, got1, err := tt.m.QueryRangeRequest(tt.args.query, tt.args.start, tt.args.end, tt.args.step)
			if (err != nil) != tt.wantErr {
				t.Errorf("Client.QueryRangeRequest() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Client.QueryRangeRequest() got = %v, want %v", got, tt.want)
			}
			if got1 != tt.want1 {
				t.Errorf("Client.QueryRangeRequest() got1 = %v, want %v", got1, tt.want1)
			}
		})

		httpServer.Shutdown(context.Background())
	}
}

func TestClient_query(t *testing.T) {
	logger := zap.NewExample(zap.Development())
	type args struct {
		query   string
		handler func(w http.ResponseWriter, r *http.Request)
	}
	tests := []struct {
		name    string
		m       *Client
		args    args
		want    []byte
		want1   string
		wantErr bool
	}{
		{
			name:  "Test query unicorn path",
			m:     &Client{protocol: "http", address: "127.0.0.1", port: "9090", logger: logger, timeout: time.Second * 30},
			args:  args{query: "http://127.0.0.1:9090/api/v1/query_range", handler: unicornHandler},
			want:  []byte(`[{"value":[1.1,"1"]}]`),
			want1: "vector",
		},
		{
			name:    "Test query data fail",
			m:       &Client{protocol: "http", address: "127.0.0.1", port: "9090", logger: logger, timeout: time.Second * 30},
			args:    args{query: "http://127.0.0.1:9090/api/v1/query_range", handler: dataFailhandler},
			wantErr: true,
		},
		{
			name:    "Test query timeout fail",
			m:       &Client{protocol: "http", address: "127.0.0.1", port: "9090", logger: logger, timeout: time.Microsecond},
			args:    args{query: "http://127.0.0.1:9090/api/v1/query_range", handler: timeoutHandler},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		httpServer := startHTTPServer("/api/v1/query_range", "9090", tt.args.handler)

		t.Run(tt.name, func(t *testing.T) {
			got, got1, err := tt.m.query(tt.args.query)
			if (err != nil) != tt.wantErr {
				t.Errorf("Client.query() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Client.query() got = %v, want %v", string(got), tt.want)
			}
			if got1 != tt.want1 {
				t.Errorf("Client.query() got1 = %v, want %v", got1, tt.want1)
			}
		})

		httpServer.Shutdown(context.Background())
	}
}

func TestClient_parseResponse(t *testing.T) {
	type args struct {
		data []byte
	}
	tests := []struct {
		name    string
		m       *Client
		args    args
		want    []byte
		want1   string
		wantErr bool
	}{
		{
			name:  "Test parseResponse unicorn path",
			m:     &Client{},
			args:  args{data: unicornResponse},
			want:  []byte(`[{"value":[1.1,"1"]}]`),
			want1: "vector",
		},
		{
			name:    "Test parseResponse data fail",
			m:       &Client{},
			args:    args{data: dataFailResponse},
			wantErr: true,
		},
		{
			name:    "Test parseResponse result fail",
			m:       &Client{},
			args:    args{data: resultFailResponse},
			wantErr: true,
		},
		{
			name:    "Test parseResponse result fail 2",
			m:       &Client{},
			args:    args{data: resultFailResponse2},
			wantErr: true,
		},
		{
			name:    "Test parseResponse result fail 3",
			m:       &Client{},
			args:    args{data: resultFailResponse3},
			wantErr: true,
		},
		{
			name:    "Test parseResponse result fail 4",
			m:       &Client{},
			args:    args{data: resultFailResponse4},
			wantErr: true,
		},
		{
			name:    "Test parseResponse result fail 5",
			m:       &Client{},
			args:    args{data: []byte{}},
			wantErr: true,
		},
		{
			name:    "Test parseResponse results fail",
			m:       &Client{},
			args:    args{data: resultsFailResponse},
			wantErr: true,
		},
		{
			name:    "Test parseResponse results fail 2",
			m:       &Client{},
			args:    args{data: resultsFailResponse2},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, got1, err := tt.m.parseResponse(tt.args.data)
			if (err != nil) != tt.wantErr {
				t.Errorf("Client.parseResponse() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Client.parseResponse() got = %v, want %v", got, tt.want)
			}
			if got1 != tt.want1 {
				t.Errorf("Client.parseResponse() got1 = %v, want %v", got1, tt.want1)
			}
		})
	}
}

func Test_shortDur(t *testing.T) {
	type args struct {
		d time.Duration
	}
	tests := []struct {
		name string
		args args
		want string
	}{
		{
			name: "Test shortDur 1",
			args: args{time.Hour*1 + time.Minute*1},
			want: "1h1m",
		},
		{
			name: "Test shortDur 2",
			args: args{time.Hour * 1},
			want: "1h",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := shortDur(tt.args.d); got != tt.want {
				t.Errorf("shortDur() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_funcInfo(t *testing.T) {
	funci := funcInfo()
	funciArr := strings.Split(funci, ":")
	funci1, _ := strconv.ParseInt(funciArr[1], 10, 64)
	funci1 = funci1 + 17
	want := fmt.Sprintf("%v:%v", funciArr[0], funci1)

	tests := []struct {
		name string
		want string
	}{
		{
			name: "Test funcInfo",
			want: want,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := funcInfo(); got != tt.want {
				t.Errorf("funcInfo() = %v, want %v", got, tt.want)
			}
		})
	}
}
