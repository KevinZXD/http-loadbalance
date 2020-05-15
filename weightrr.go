package httplb

import (
	"sync"
	"time"

	"github.com/valyala/fasthttp"
)

// WeightedRoundRobinLB 加权最小连接数
type WeightedRoundRobinLB struct {

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

	// Timeout is the request timeout used when calling WeightedRoundRobinLB.Do.
	//
	// DefaultLBClientTimeout is used by default.
	Timeout time.Duration

	cs []*lbClient

	once sync.Once

	watcher *watcher

	lock sync.RWMutex

	i         int    // 表示上一次选择的服务器
	cw        uint16 // 表示当前调度的权值
	gcd       uint16 // 当前所有权重的最大公约数 比如 2，4，8 的最大公约数为：2
	maxWeight uint16 // 最大权重
}

// NewWeightedRoundRobinLB 创建WLC负载均衡策略
func NewWeightedRoundRobinLB(config *Config) *WeightedRoundRobinLB {
	clients := createClients(config.NodeList, config.Opts)
	w := newWatcher(config)

	lb := WeightedRoundRobinLB{
		watcher: w,
		config:  config,
		clients: clients,
	}
	lb.fetchOnce()
	go lb.watch()
	return &lb
}

func (cc *WeightedRoundRobinLB) watch() {
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

func (cc *WeightedRoundRobinLB) fetchOnce() {
	nodes := cc.watcher.watch(cc.config)
	newClients, _ := updateClients(cc.clients, nodes, cc.config.Opts)
	cc.lock.Lock()
	cc.i = -1
	cc.cw = 0
	cc.gcd = 0
	cc.maxWeight = 0
	weights := make([]uint16, 0, len(nodes))
	for _, invoker := range nodes {
		weights = append(weights, invoker.Weight)
	}
	cc.gcd = gcdx(weights)
	cc.maxWeight = getMaxWeight(nodes)

	cc.clients = newClients
	cc.init()
	cc.lock.Unlock()
}

// DoDeadline calls DoDeadline on the least loaded client
func (cc *WeightedRoundRobinLB) DoDeadline(req *fasthttp.Request, resp *fasthttp.Response, deadline time.Time) error {
	return cc.get().DoDeadline(req, resp, deadline)
}

// DoTimeout calculates deadline and calls DoDeadline on the least loaded client
func (cc *WeightedRoundRobinLB) DoTimeout(req *fasthttp.Request, resp *fasthttp.Response, timeout time.Duration) error {
	return cc.get().DoTimeout(req, resp, timeout)
}

// Do calls calculates deadline using WeightedRoundRobinLB.Timeout and calls DoDeadline
// on the least loaded client.
func (cc *WeightedRoundRobinLB) Do(req *fasthttp.Request, resp *fasthttp.Response) error {
	return cc.get().Do(req, resp)
}

func (cc *WeightedRoundRobinLB) Get() Client {
	return cc.get()
}

func (cc *WeightedRoundRobinLB) init() {
	if len(cc.clients) == 0 {
		panic("BUG: WeightedRoundRobinLB.clients cannot be empty")
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

func (cc *WeightedRoundRobinLB) get() *lbClient {
	cc.once.Do(cc.init)

	cc.lock.RLock()
	defer cc.lock.RUnlock()

	cs := cc.cs
	for {
		cc.i = (cc.i + 1) % len(cc.clients)
		if cc.i == 0 {
			cc.cw = cc.cw - cc.gcd
			if cc.cw <= 0 {
				cc.cw = cc.maxWeight
				if cc.cw == 0 {
					return nil
				}
			}
		}

		if weight := cc.clients[cc.i].Node().Weight; weight >= cc.cw {
			return cs[cc.i]
		}
	}
}

// gcdx 获取多个数值的最大公约数
func gcdx(array []uint16) uint16 {
	if len(array) == 0 {
		return 0
	}
	var tmp uint16
	var y = array[0]
	for i := 1; i < len(array); i++ {
		x := array[i]
		for {
			tmp = x % y
			if tmp > 0 {
				x = y
				y = tmp
			} else {
				break
			}
		}
	}
	return y
}

// 获取最大权重
func getMaxWeight(nodes []*Node) uint16 {
	var max uint16 = 0
	for _, v := range nodes {
		if v.Weight > max {
			max = v.Weight
		}
	}
	return max
}
