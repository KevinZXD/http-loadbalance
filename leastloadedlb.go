package httplb

import (
	"sync"
	"sync/atomic"
	"time"

	"github.com/valyala/fasthttp"
)

// LeastLoadedLB balances requests among available LeastLoadedLB.clients.
//
// It has the following features:
//
//   - Balances load among available clients using 'least loaded' + 'least total'
//     hybrid technique.
//   - Dynamically decreases load on unhealthy clients.
//
// It is forbidden copying LeastLoadedLB instances. Create new instances instead.
//
// It is safe calling LeastLoadedLB methods from concurrently running goroutines.
type LeastLoadedLB struct {

	// clients must contain non-zero clients list.
	// Incoming requests are balanced among these clients.
	clients []Client
	config  *Config // 记录配置文件

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

	// Timeout is the request timeout used when calling LeastLoadedLB.Do.
	//
	// DefaultLBClientTimeout is used by default.
	Timeout time.Duration

	cs []*lbClient

	once sync.Once

	watcher *watcher

	lock sync.RWMutex
}

// NewLeastLB 创建最小连接数负载均衡器
func NewLeastLB(config *Config) *LeastLoadedLB {
	clients := createClients(config.NodeList, config.Opts)
	w := newWatcher(config)

	lb := LeastLoadedLB{
		watcher: w,
		config:  config,
		clients: clients,
		Timeout: config.Opts.ReadTimeout * 2, // 默认负载均衡器超时时间是配置中read_timeout的2倍
	}
	lb.fetchOnce()
	go lb.watch()
	return &lb
}

func (cc *LeastLoadedLB) watch() {
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

func (cc *LeastLoadedLB) fetchOnce() {
	nodes := cc.watcher.watch(cc.config)
	newClients, _ := updateClients(cc.clients, nodes, cc.config.Opts)
	cc.lock.Lock()
	cc.clients = newClients
	cc.init()
	cc.lock.Unlock()
}

// DefaultLBClientTimeout is the default request timeout used by LeastLoadedLB
// when calling LeastLoadedLB.Do.
//
// The timeout may be overridden via LeastLoadedLB.Timeout.
const DefaultLBClientTimeout = time.Second * 2

// DoDeadline calls DoDeadline on the least loaded client
func (cc *LeastLoadedLB) DoDeadline(req *fasthttp.Request, resp *fasthttp.Response, deadline time.Time) error {
	return cc.get().DoDeadline(req, resp, deadline)
}

// DoTimeout calculates deadline and calls DoDeadline on the least loaded client
func (cc *LeastLoadedLB) DoTimeout(req *fasthttp.Request, resp *fasthttp.Response, timeout time.Duration) error {
	return cc.get().DoTimeout(req, resp, timeout)
}

// Do calls calculates deadline using LeastLoadedLB.Timeout and calls DoDeadline
// on the least loaded client.
func (cc *LeastLoadedLB) Do(req *fasthttp.Request, resp *fasthttp.Response) error {
	return cc.get().Do(req, resp)
}

func (cc *LeastLoadedLB) Get() Client {
	return cc.get()
}

func (cc *LeastLoadedLB) init() {
	if len(cc.clients) == 0 {
		panic("BUG: LeastLoadedLB.clients cannot be empty")
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

func (cc *LeastLoadedLB) get() *lbClient {

	cc.lock.RLock()
	defer cc.lock.RUnlock()

	cs := cc.cs

	minC := cs[0]
	minN := minC.PendingRequests()
	minT := atomic.LoadUint64(&minC.total)
	for _, c := range cs[1:] {
		n := c.PendingRequests()
		t := atomic.LoadUint64(&c.total)
		if n < minN || (n == minN && t < minT) {
			minC = c
			minN = n
			minT = t
		}
	}
	return minC
}
