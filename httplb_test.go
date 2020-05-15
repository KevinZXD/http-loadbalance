package httplb_test

import (
	"fmt"
	"testing"
	"time"

	"github.com/valyala/fasthttp"

	httplb "http-loadbalance"
)

var (
	cfg = httplb.Config{
		LBStrategy: httplb.LBRandom,
		Type:       httplb.TypeStatic,
		Consul: &httplb.ConsulConfig{
			ConsulAgent: "127.0.0.1:8500",
			ServiceName: "flash-mock-http",
			TagName:     "",
		},
		DNS: &httplb.DnsConfig{
			Domain:    "flash-mock-http.service.consul",
			Type:      httplb.DNSTypeSRV,
			Port:      7780,
			DNSServer: "127.0.0.1:53",
		},
		IPList: []string{
			"127.0.0.1:7780 weight=1000",
			"127.0.0.1:7781 weight=200",
			"127.0.0.1:7780 weight=400",
		},
		Opts: &httplb.Opts{
			MaxConns:            1,
			ConnectTimeout:      time.Second * 10,
			ReadTimeout:         time.Second * 10,
			WriteTimeout:        time.Second * 10,
			MaxConnDuration:     time.Minute * 10,
			MaxIdleConnDuration: time.Minute,
			MaxCallAttempts:     1,
		},
	}
)

func TestHTTPLB(t *testing.T) {
	var err error
	if err = cfg.Validate(); err != nil {
		t.Fatal(err)
	}
	lb := httplb.New(&cfg)
	i := 0
	for {
		time.Sleep(time.Millisecond * 10)
		req := fasthttp.Request{}
		req.SetRequestURI("http://test/api")
		var resp fasthttp.Response
		c := lb.Get()
		err = c.Do(&req, &resp)

		if err != nil {
			fmt.Println(i, "name:", c.Name(), " error:", err)
		} else {
			fmt.Println(i, "name:", c.Name(), " response:", resp.StatusCode())
		}
		i++
	}
}

func BenchmarkLB(b *testing.B) {
	// 测试负载均衡性能，需要注释掉对应负载均衡.Do()函数中的请求语句
	var err error
	if err = cfg.Validate(); err != nil {
		b.Fatal(err)
	}

	lb := httplb.New(&cfg)

	for i := 0; i < b.N; i++ {
		req := fasthttp.Request{}
		req.SetRequestURI("http://test/api")
		var resp fasthttp.Response
		c := lb.Get()
		_ = c.Do(&req, &resp)
	}
}
