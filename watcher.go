package httplb

import (
	"fmt"
	"net"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/hashicorp/consul/api"
	"github.com/miekg/dns"
)

var (
	defaultDNSResolverInterval = time.Second * 10 // DNS解析频率，每10s更新
)

// watcher 监听器，监听各种类型consul/dns等的变化，并返回服务节点列表
type watcher struct {
	config          *Config
	consulClient    *api.Client
	once            sync.Once
	consulFlag      bool
	ConsulWaitIndex uint64
	first           bool
	nodes           []*Node
	dnsClient       *dns.Client
}

func newWatcher(cfg *Config) *watcher {
	return &watcher{
		config:    cfg,
		nodes:     cfg.NodeList,
		dnsClient: &dns.Client{},
	}
}

func (w *watcher) initConsulClient() {
	client, err := api.NewClient(&api.Config{
		Address: w.config.Consul.ConsulAgent,
	})

	if err == nil {
		w.consulFlag = true
	} else {
		// TODO 连接consul报错
	}
	w.consulClient = client
}

func (w *watcher) watch(config *Config) []*Node {
	// 更新配置，以方便动态加载
	w.config = config
	switch config.Type {
	case TypeDNS:
		return w.watchDns()
	case TypeConsul:
		return w.watchConsul()
	case TypeStatic:
		// static 无需watch
	}
	return w.nodes
}

// 轮询监听dns变化，如果watcher.clients=nil，则立即
func (w *watcher) watchDns() []*Node {
	switch strings.ToUpper(w.config.DNS.Type) {
	case DNSTypeA:
		return w.digA()
	case DNSTypeSRV:
		return w.digSRV()
	}
	return nil
}

func (w *watcher) digA() []*Node {

	// 构建dns请求体
	msg := new(dns.Msg)
	msg.Id = dns.Id()
	msg.SetQuestion(w.config.DNS.Domain+".", dns.TypeA)
	msg.RecursionDesired = true

	for {
		// 默认使用配置的第一个dns服务器进行解析
		resp, _, err := w.dnsClient.Exchange(
			msg, net.JoinHostPort(w.config.DNS.dnsServerIP, w.config.DNS.dnsServerPort),
		)
		if nil == resp {
			fmt.Printf("error: %v\n", err)
		} else {
			if dns.RcodeSuccess != resp.Rcode {
				fmt.Printf("error: invalid host name, %s\n", w.config.DNS.Domain)
			} else {
				nodes := make([]*Node, 0, len(resp.Answer))
				for _, a := range resp.Answer {
					nodes = append(nodes, &Node{
						IP:     a.(*dns.A).A.String(),
						Port:   w.config.DNS.Port,
						Weight: 100, // DNS A记录，权重没有意义，统一一致的权重即可
					})
				}
				if !w.equals(nodes) {
					w.nodes = nodes
					return nodes
				}
			}
		}
		// dns解析休眠
		time.Sleep(defaultDNSResolverInterval)
	}
}

// digSRV 获取域名SRV信息，并拼接为节点列表
func (w *watcher) digSRV() []*Node {

	// 构建dns请求体
	msg := new(dns.Msg)
	msg.Id = dns.Id()
	msg.SetQuestion(w.config.DNS.Domain+".", dns.TypeSRV)
	msg.RecursionDesired = true

	for {
		// 默认使用配置的第一个dns服务器进行解析
		resp, _, err := w.dnsClient.Exchange(
			msg, net.JoinHostPort(w.config.DNS.dnsServerIP, w.config.DNS.dnsServerPort),
		)
		if nil == resp {
			fmt.Printf("error: %v\n", err)
		} else {
			if dns.RcodeSuccess != resp.Rcode {
				fmt.Printf("error: invalid host name, %s\n", w.config.DNS.Domain)
			} else {
				nodes := resolvSRVInvoker(resp)
				if !w.equals(nodes) {
					w.nodes = nodes
					return nodes
				}
			}
		}
		// dns解析休眠
		time.Sleep(defaultDNSResolverInterval)
	}
}

func resolvSRVInvoker(msg *dns.Msg) []*Node {
	invokers := make([]*Node, 0, len(msg.Answer))
	targetIPMap := make(map[string]string) // IP和SRV中的Target字段映射
	for _, ext := range msg.Extra {
		a, ok := ext.(*dns.A)
		if !ok {
			continue
		}
		targetIPMap[a.Hdr.Name] = a.A.String()
	}
	for _, ans := range msg.Answer {
		srv, ok := ans.(*dns.SRV)
		if !ok {
			continue
		}
		invokers = append(invokers, &Node{
			IP:     targetIPMap[srv.Target],
			Port:   srv.Port,
			Weight: srv.Weight,
		})
	}
	return invokers
}

// 使用consul服务发现来持续监听节点变化
func (w *watcher) watchConsul() []*Node {
	w.once.Do(w.initConsulClient)

	option := &api.QueryOptions{
		WaitIndex: w.ConsulWaitIndex,
		UseCache:  false,
		Token:     w.config.Consul.Token,
	}
	entrys, meta, err := w.consulClient.Health().Service(w.config.Consul.ServiceName, w.config.Consul.TagName, true, option)
	// 如果请求异常，或者返回列表为空，则直接返回旧的node列表
	if err != nil || len(entrys) == 0 {
		return w.nodes
	}
	w.ConsulWaitIndex = meta.LastIndex
	nodes := make([]*Node, 0, len(entrys))
	for _, entry := range entrys {
		node := Node{
			IP:     entry.Service.Address,
			Port:   uint16(entry.Service.Port),
			Weight: uint16(entry.Service.Weights.Passing),
		}
		nodes = append(nodes, &node)
	}
	w.nodes = nodes

	return nodes
}

// 长度和原map相等，且每个node对应的client都存在，表示和原来的相等
func (w *watcher) equals(nodes []*Node) bool {
	// 如果新获取节点为空数组，则不更新，标记为和旧的一致即可
	if len(nodes) == 0 {
		return true
	}
	if len(nodes) != len(w.nodes) {
		return false
	}
	// 判断两个Node数组的String()值是否一致
	sa := make([]string, 0, len(nodes))
	sb := make([]string, 0, len(w.nodes))
	for i := 0; i < len(nodes); i++ {
		sa = append(sa, nodes[i].String())
		sb = append(sb, w.nodes[i].String())
	}
	sort.Strings(sa)
	sort.Strings(sb)
	for i := 0; i < len(sa); i++ {
		if sa[i] != sb[i] {
			return false
		}
	}
	return true
}
