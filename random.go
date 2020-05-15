package httplb

import (
	"math/rand"
	"sync"
	"time"

	"github.com/valyala/fasthttp"
)

// RandomLB 加权轮询
type RandomLB struct {

	// clients must contain non-zero clients list.
	// Incoming requests are balanced among these clients.
	clients []Client
	config  *Config // 记录配置文件
	r       *rand.Rand

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

	// Timeout is the request timeout used when calling RandomLB.Do.
	//
	// DefaultLBClientTimeout is used by default.
	Timeout time.Duration

	cs []*lbClient

	once sync.Once

	watcher *watcher

	lock sync.RWMutex

}

// NewRandomLB 创建随机负载均衡
func NewRandomLB(config *Config) *RandomLB {
	seed := rand.NewSource(time.Now().UnixNano())
	clients := createClients(config.NodeList, config.Opts)
	w := newWatcher(config)

	lb := RandomLB{
		watcher: w,
		config:  config,
		clients: clients,
		r:       rand.New(seed),
	}
	lb.fetchOnce()
	go lb.watch()
	return &lb
}

func (cc *RandomLB) watch() {
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

func (cc *RandomLB) fetchOnce() {
	nodes := cc.watcher.watch(cc.config)
	newClients, _ := updateClients(cc.clients, nodes, cc.config.Opts)
	cc.lock.Lock()
	cc.clients = newClients
	cc.init()
	cc.lock.Unlock()
}

// DoDeadline calls DoDeadline on the least loaded client
func (cc *RandomLB) DoDeadline(req *fasthttp.Request, resp *fasthttp.Response, deadline time.Time) error {
	return cc.get().DoDeadline(req, resp, deadline)
}

// DoTimeout calculates deadline and calls DoDeadline on the least loaded client
func (cc *RandomLB) DoTimeout(req *fasthttp.Request, resp *fasthttp.Response, timeout time.Duration) error {
	return cc.get().DoTimeout(req, resp, timeout)
}

// Do calls calculates deadline using RandomLB.Timeout and calls DoDeadline
// on the least loaded client.
func (cc *RandomLB) Do(req *fasthttp.Request, resp *fasthttp.Response) error {
	return cc.get().Do(req, resp)
}

func (cc *RandomLB) Get() Client {
	return cc.get()
}

func (cc *RandomLB) init() {
	if len(cc.clients) == 0 {
		panic("BUG: RandomLB.clients cannot be empty")
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

func (cc *RandomLB) get() *lbClient {
	cc.once.Do(cc.init)

	cc.lock.RLock()
	defer cc.lock.RUnlock()

	cs := cc.cs
	index:=cc.r.Intn(len(cc.cs))
	return cs[index]

}
