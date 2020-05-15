package httplb

import (
	"time"

	"github.com/valyala/fasthttp"
)

// LoadBalancer 负载均衡接口，提供Get()函数以获取分配的Client
type LoadBalancer interface {
	DoDeadline(req *fasthttp.Request, resp *fasthttp.Response, deadline time.Time) error
	DoTimeout(req *fasthttp.Request, resp *fasthttp.Response, timeout time.Duration) error
	Do(req *fasthttp.Request, resp *fasthttp.Response) error
	Get() Client
}

// Client HTTP客户端接口，在原基础上添加Name()和Node()函数以方便获取节点信息
type Client interface {
	fasthttp.BalancingClient
	DoTimeout(req *fasthttp.Request, resp *fasthttp.Response, timeout time.Duration) error
	Do(req *fasthttp.Request, resp *fasthttp.Response) error
	Name() string // 获取一个Node名称
	Node() *Node  // 获取对应Node信息，包含IP/端口/权重等
}
