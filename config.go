package httplb

import (
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/miekg/dns"

	"http-loadbalance/libs/validate"
)

var (
	dnsReg = regexp.MustCompile(`^([\d.]+):([\d]+)$`)
)

// Config 一组服务的配置
//
// 标识类别（static/dns/consul）等
// static: 列出服务静态IP列表
// dns: 提供domain域名
// consul: 提供服务发现地址
type Config struct {
	LBStrategy int           `toml:"lb_strategy"`              // 负载均衡策略 eg：LBRoundRobin, LBWeightRandom
	Type       string        `toml:"type" validate:"required"` // "dns" or "consul" or "static"
	Consul     *ConsulConfig `toml:"consul"`
	DNS        *DnsConfig    `toml:"dns"`
	IPList     []string      `toml:"ip_list"`   // 从配置文件中读取的host列表, 如果服务发现服务失效, 使用IPList
	NodeList   []*Node       `toml:"node_list"` // 从配置文件中读取的host列表, 如果服务发现服务失效, 使用StaticHosts
	Opts       *Opts         `toml:"opts"`
}

// Opts HTTP资源细节配置，如连接超时等
type Opts struct {
	MaxConns            int           `toml:"max_conns" validate:"default=2"` // 每个ip的最大连接个数，默认2
	IsTLS               bool          `toml:"is_tls"`                         // 是否安全连接
	ConnectTimeout      time.Duration `toml:"connect_timeout"`
	ReadTimeout         time.Duration `toml:"read_timeout"`
	WriteTimeout        time.Duration `toml:"write_timeout"`
	MaxConnDuration     time.Duration `toml:"max_conn_duration"`                      // 空闲
	MaxIdleConnDuration time.Duration `toml:"max_idle_conn_duration"`                 // 空闲连接的keep alive 时间，默认10s
	MaxCallAttempts     int           `toml:"max_call_attempts" validate:"default=1"` // 尝试请求次数，默认1
}

// 转换IPList格式，将配置文件中的[]string转换为[]*Node
func (c *Config) convertIPList() error {
	if len(c.IPList) > 0 && len(c.NodeList) == 0 {
		var nodes []*Node
		for _, info := range c.IPList {
			node, err := newNode(info)
			if err != nil {
				return err
			}
			nodes = append(nodes, node)
		}
		c.NodeList = nodes
	}
	return nil
}

func (c *Config) Validate() error {
	var err error
	if err = c.convertIPList(); err != nil {
		return err
	}
	switch strings.ToLower(c.Type) {
	case TypeStatic:
		if len(c.NodeList) == 0 {
			return errors.New("type=statc static_ip_list cannot empty")
		}
		for _, node := range c.NodeList {
			if err = node.Validate(); err != nil {
				return err
			}
		}
	case TypeConsul:
		if c.Consul == nil {
			return errors.New("type=consul consul config cannot empty")
		}
		if err = c.Consul.Validate(); err != nil {
			return err
		}
	case TypeDNS:
		if c.DNS == nil {
			return errors.New("type=dns dns config cannot empty")
		}
		if err = c.DNS.Validate(); err != nil {
			return err
		}
	}
	// 默认负载均衡策略设置为最小连接
	if c.LBStrategy == 0 {
		c.LBStrategy = LBLeastConnection
	}
	return validate.Validator.Struct(c)
}

// DnsConfig DNS配置
type DnsConfig struct {
	Domain        string `toml:"domain" validate:"required"` // 域名
	Type          string `toml:"type" validate:"default=A"`  // 类型，可选值 SRV / A，默认A记录
	dnsType       uint16 // 转换为数值 dns.TypeA和dns.TypeSRV
	Port          uint16 `toml:"port" validate:"default=80"`                      // A记录时所使用的端口，默认80；SRV不使用这个全局端口
	ResolvFile    string `toml:"resolv_file" validate:"default=/etc/resolv.conf"` // dns server获取的文件路径
	DNSServer     string `toml:"dns_server"`                                      // DNS服务器，格式："10.13.40.145:53"，如设置该值，则ResolvFile配置失效
	dnsServerIP   string // DNS服务器IP，从DNSServer配置中解析
	dnsServerPort string // DNS服务器的端口号，从DNSServer配置中解析
}

func (dc *DnsConfig) Validate() error {
	err := validate.Validator.Struct(dc)
	if err != nil {
		return err
	}
	// 转换DNS类型，默认A记录
	switch strings.ToUpper(dc.Type) {
	case "SRV":
		dc.dnsType = dns.TypeSRV
	default:
		dc.dnsType = dns.TypeA
	}
	if dc.DNSServer != "" {
		g := dnsReg.FindAllStringSubmatch(dc.DNSServer, -1)
		if len(g) < 1 || len(g[0]) < 3 {
			return fmt.Errorf("[%s] dns_server with wrong format. check if \"IP:Port\"", dc.Domain)
		}
		dc.dnsServerIP = g[0][1]
		dc.dnsServerPort = g[0][2]
	} else {
		config, err := dns.ClientConfigFromFile(dc.ResolvFile)
		if err != nil {
			return err
		}
		dc.dnsServerIP = config.Servers[0]
		dc.dnsServerPort = config.Port
	}
	return nil
}

type ConsulConfig struct {
	ConsulAgent string `toml:"consul_agent" validate:"default=127.0.0.1:8500"` // consul地址，默认127.0.0.1:8500
	ServiceName string `toml:"service_name" validate:"required"`               // consul服务发现名称
	TagName     string `toml:"tag_name"`                                       // consul服务tag名称
	Token       string `toml:"token"`                                          // 所需要的token
}

func (c *ConsulConfig) Validate() error {
	return validate.Validator.Struct(c)
}
