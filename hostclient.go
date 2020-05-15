package httplb

import (
	"net"

	"github.com/valyala/fasthttp"
)

// HostClient 负载均衡器使用的HTTP客户端
type HostClient struct {
	fasthttp.HostClient
	node *Node
}

// Name 获取客户端名称，根据节点信息IP:Port_Weight拼接而成
func (c *HostClient) Name() string {
	return c.HostClient.Name
}

// Node 获取客户端的节点信息，如IP，端口权重等
func (c *HostClient) Node() *Node {
	return c.node
}

// NewHostClient 创建HTTP Client客户端
func NewHostClient(node *Node, opts *Opts) Client {
	c := HostClient{
		HostClient: fasthttp.HostClient{
			Addr: node.Addr(),
			Name: node.String(),
			Dial: func(addr string) (net.Conn, error) {
				return fasthttp.DialTimeout(addr, opts.ConnectTimeout)
			},
			IsTLS:                     opts.IsTLS,
			MaxConns:                  opts.MaxConns,
			MaxConnDuration:           opts.MaxConnDuration,
			MaxIdleConnDuration:       opts.MaxIdleConnDuration,
			MaxIdemponentCallAttempts: opts.MaxCallAttempts,
			ReadTimeout:               opts.ReadTimeout,
			WriteTimeout:              opts.WriteTimeout,
		},
		node: node,
	}
	return &c
}
