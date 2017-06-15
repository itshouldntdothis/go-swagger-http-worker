package main

import (
	"errors"
	"log"
	"net"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	pb "github.com/itshouldntdothis/swagger-http-grpc"

	"go.uber.org/ratelimit"
	"golang.org/x/net/context"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
	"gopkg.in/resty.v0"
)

// GrcpServerOptions implements processing functions
type GrcpServerOptions struct {
	Addr        string
	Server      *grpc.Server
	RateLimited bool
	RateLimiter ratelimit.Limiter
	RestyClient *resty.Client
	UserAgent   string
}

// NewGrcpOptions returns a initialized object
func NewGrcpOptions() *GrcpServerOptions {
	Transport := &http.Transport{
		MaxIdleConns: 200,
		DialContext: (&net.Dialer{
			Timeout:   60 * time.Second,
			KeepAlive: 60 * time.Second,
		}).DialContext,
		IdleConnTimeout:       60 * time.Second,
		TLSHandshakeTimeout:   20 * time.Second,
		ResponseHeaderTimeout: 20 * time.Second,
		ExpectContinueTimeout: 0,
		MaxIdleConnsPerHost:   40,
	}

	port := "50051"
	envGrcpPort := os.Getenv("SW_GRPC_PORT")
	if len(envGrcpPort) != 0 {
		port = envGrcpPort
	}
	Addr := ":" + port
	log.Printf("SET GRPC server port to %v", Addr)

	UserAgent := os.Getenv("SW_USER_AGENT")
	if len(UserAgent) == 0 {
		log.Fatal("Default useragent env `SW_USER_AGENT` is required")
	}
	log.Printf("SET GRPC HTTP WORKER User-Agent to `%v`", UserAgent)

	RateLimit := 700
	var RateLimiter ratelimit.Limiter
	var RateLimited bool
	envRateLimit := os.Getenv("SW_REQUEST_LIMIT")
	if len(envRateLimit) != 0 {
		parsedRequestLimit, err := strconv.Atoi(envRateLimit)
		if err != nil {
			log.Fatalf("Unable to covert ENV 'SW_REQUEST_LIMIT' of %v to int : %v", envRateLimit, err)
		}
		RateLimit = parsedRequestLimit
	}

	if RateLimit == 0 {
		RateLimited = false
		log.Println("Rate Limit of 0 is not recommended")
	} else {
		RateLimited = true
		log.Printf("Rate Limit set to %v", RateLimit)
		RateLimiter = ratelimit.New(RateLimit)
	}

	return &GrcpServerOptions{
		Server:      grpc.NewServer(),
		Addr:        Addr,
		RestyClient: resty.New().SetHeader("Via", via).SetTransport(Transport),
		RateLimited: RateLimited,
		RateLimiter: RateLimiter,
		UserAgent:   UserAgent,
	}
}

func (o *GrcpServerOptions) start() {
	lis, err := net.Listen("tcp", o.Addr)
	if err != nil {
		log.Fatalf("Failed to listen: %v", err)
	}
	pb.RegisterWorkersServer(o.Server, o)
	// Register reflection service on gRPC server.
	reflection.Register(o.Server)

	log.Println("Starting grpc server")
	if err := o.Server.Serve(lis); err != nil {
		log.Fatalf("Failed to serve grpc server: %v", err)
	}
}

func (o *GrcpServerOptions) close() {
	log.Println("Closing down grpc server")
	o.Server.GracefulStop()
}

// DoRequest processes a single http request from grpc
func (o *GrcpServerOptions) DoRequest(ctx context.Context, in *pb.Request) (*pb.Response, error) {
	method := in.GetMethod()
	if method != "GET" && method != "POST" && method != "PUT" && method != "DELETE" {
		return nil, errors.New("We don't support " + method + " method at this time")
	}

	restyReq := o.RestyClient.R()
	if len(in.GetHeaders()) > 0 {
		restyReq = restyReq.SetHeaders(in.GetHeaders())
	}

	if restyReq.Header.Get("User-Agent") == "" {
		restyReq = restyReq.SetHeader("User-Agent", o.UserAgent)
	}

	if in.GetBody() != "" {
		restyReq = restyReq.SetBody(in.GetBody())
	}

	if o.RateLimited {
		o.RateLimiter.Take()
	}

	resp, err := restyReq.Execute(method, in.GetUrl())
	if err != nil {
		log.Printf("Fetch Error %v", err)
		return nil, err
	}

	log.Printf("%v successful for %v took %v", method, resp.Request.URL, resp.Time())

	headers := make(map[string]*pb.Header)
	for k, v := range resp.Header() {
		headers[strings.ToLower(k)] = &pb.Header{Value: v}
	}

	return &pb.Response{
		Ok:         isOk(resp.StatusCode()),
		Url:        resp.Request.URL,
		Status:     int32(resp.StatusCode()),
		StatusText: resp.Status(),
		Body:       resp.String(),
		Headers:    headers,
	}, nil
}

func isOk(status int) bool {
	if status >= 200 && status <= 299 {
		return true
	}
	return false
}
