package httplb

import (
	"sync/atomic"
	"time"

	"github.com/valyala/fasthttp"
)

type lbClient struct {
	c           Client
	healthCheck func(req *fasthttp.Request, resp *fasthttp.Response, err error) bool
	penalty     uint32

	// total amount of requests handled.
	total uint64
}

func (c *lbClient) Do(req *fasthttp.Request, resp *fasthttp.Response) error {
	err := c.c.Do(req, resp)
	c.panalty(req, resp, err)
	return err
}
func (c *lbClient) DoTimeout(req *fasthttp.Request, resp *fasthttp.Response, timeout time.Duration) error {
	err := c.c.DoTimeout(req, resp, timeout)
	c.panalty(req, resp, err)
	return err
}

func (c *lbClient) DoDeadline(req *fasthttp.Request, resp *fasthttp.Response, deadline time.Time) error {
	err := c.c.DoDeadline(req, resp, deadline)
	c.panalty(req, resp, err)
	return err
}

func (c *lbClient) panalty(req *fasthttp.Request, resp *fasthttp.Response, err error) {
	if !c.isHealthy(req, resp, err) && c.incPenalty() {
		// Penalize the client returning error, so the next requests
		// are routed to another clients.
		time.AfterFunc(penaltyDuration, c.decPenalty)
	} else {
		atomic.AddUint64(&c.total, 1)
	}
}

func (c *lbClient) PendingRequests() int {
	n := c.c.PendingRequests()
	m := atomic.LoadUint32(&c.penalty)
	return n + int(m)
}

func (c *lbClient) Name() string {
	return c.c.Name()
}

func (c *lbClient) Node() *Node {
	return c.c.Node()
}
func (c *lbClient) isHealthy(req *fasthttp.Request, resp *fasthttp.Response, err error) bool {
	if c.healthCheck == nil {
		return err == nil
	}
	return c.healthCheck(req, resp, err)
}

func (c *lbClient) incPenalty() bool {
	m := atomic.AddUint32(&c.penalty, 1)
	if m > maxPenalty {
		c.decPenalty()
		return false
	}
	return true
}

func (c *lbClient) decPenalty() {
	atomic.AddUint32(&c.penalty, ^uint32(0))
}

const (
	maxPenalty = 300

	penaltyDuration = time.Second
)
