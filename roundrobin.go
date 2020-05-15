package httplb

import (
	"sync"
	"sync/atomic"
	"time"

	"github.com/valyala/fasthttp"
)

// RoundRobinLB 轮询
type RoundRobinLB struct {

	// clients must contain non-zero clients list.
	// Incoming requests are balanced among these clients.
	clients []Client
	config  *Config // 记录配置文件
	index   int32

	// HealthCheck is a callback called after each request.
	//
	// The request, response and the error returned by the client
	// is passed to HealthCheck, so the callback may determine whether
	// the client is healthy.
	//
	// Load on the current client is decreased if HealthCheck returns false.
	//
	// By default HealthCheck returns false if err != nil.
	HealthCheck func(req *fasthttp.Request, resp *fasthttp.Response, err error) bool

	// Timeout is the request timeout used when calling RoundRobinLB.Do.
	//
	// DefaultLBClientTimeout is used by default.
	Timeout time.Duration

	cs []*lbClient

	once sync.Once

	watcher *watcher

	lock sync.RWMutex
}

// NewRoundRobinLB 创建轮询负载均衡策略
func NewRoundRobinLB(config *Config) *RoundRobinLB {
	clients := createClients(config.NodeList, config.Opts)
	w := newWatcher(config)

	lb := RoundRobinLB{
		watcher: w,
		config:  config,
		clients: clients,
	}
	lb.fetchOnce()
	go lb.watch()
	return &lb
}

func (cc *RoundRobinLB) watch() {
	for {
		time.Sleep(time.Second * 5)
		nodes := cc.watcher.watch(cc.config)
		newClients, isUpdate := updateClients(cc.clients, nodes, cc.config.Opts)
		if !isUpdate {
			continue
		}
		cc.lock.Lock()
		cc.clients = newClients
		cc.init()
		cc.lock.Unlock()
	}
}

func (cc *RoundRobinLB) fetchOnce() {
	nodes := cc.watcher.watch(cc.config)
	newClients, _ := updateClients(cc.clients, nodes, cc.config.Opts)
	cc.lock.Lock()
	cc.clients = newClients
	cc.init()
	cc.lock.Unlock()
}

// DoDeadline calls DoDeadline on the least loaded client
func (cc *RoundRobinLB) DoDeadline(req *fasthttp.Request, resp *fasthttp.Response, deadline time.Time) error {
	return cc.get().DoDeadline(req, resp, deadline)
}

// DoTimeout calculates deadline and calls DoDeadline on the least loaded client
func (cc *RoundRobinLB) DoTimeout(req *fasthttp.Request, resp *fasthttp.Response, timeout time.Duration) error {
	return cc.get().DoTimeout(req, resp, timeout)
}

// Do calls calculates deadline using RoundRobinLB.Timeout and calls DoDeadline
// on the least loaded client.
func (cc *RoundRobinLB) Do(req *fasthttp.Request, resp *fasthttp.Response) error {
	return cc.get().Do(req, resp)
}

func (cc *RoundRobinLB) Get() Client {
	return cc.get()
}

func (cc *RoundRobinLB) init() {
	if len(cc.clients) == 0 {
		panic("BUG: RoundRobinLB.clients cannot be empty")
	}

	cs := make([]*lbClient, 0, len(cc.clients))
	for _, c := range cc.clients {
		cs = append(cs, &lbClient{
			c:           c,
			healthCheck: cc.HealthCheck,
		})
	}
	cc.cs = cs
}

func (cc *RoundRobinLB) get() *lbClient {
	cc.once.Do(cc.init)

	cc.lock.RLock()
	defer cc.lock.RUnlock()

	cs := cc.cs
	i := atomic.AddInt32(&cc.index, 1)
	return cs[int(i)%len(cc.cs)]
}
