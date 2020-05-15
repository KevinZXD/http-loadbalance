package httplb

import (
	"fmt"
	"regexp"
	"strconv"

	"http-loadbalance/libs/validate"
)

// Node 服务节点最小单元
type Node struct {
	IP     string `toml:"ip" validate:"required"`        // IP地址
	Port   uint16 `toml:"port" validate:"required"`      // 端口号
	Weight uint16 `toml:"weight" validate:"default=100"` // 权重值
	name   string // 一个节点的唯一标记
}

func (i *Node) Validate() error {
	return validate.Validator.Struct(i)
}

// Addr 获取节点服务地址，如：10.85.101.122:8080
func (i *Node) Addr() string {
	return fmt.Sprintf("%s:%d", i.IP, i.Port)
}

func (i *Node) String() string {
	if i.name == "" {
		i.name = fmt.Sprintf("%s:%d_w%d", i.IP, i.Port, i.Weight)
	}
	return i.name
}

var (
	nodeInfoReg          = regexp.MustCompile(`^([\d.]+)(?:(?::(\d+))|)(?:(?:[\t ]+weight[\t ]*=[\t ]*(\d+))|)`)
	defaultPort   uint64 = 80
	defaultWeight uint64 = 1
)

// newNode 新建IP Port节点，根据info字符串解析得出
func newNode(info string) (node *Node, err error) {
	g := nodeInfoReg.FindAllStringSubmatch(info, -1)
	if len(g) < 1 || len(g[0]) < 3 {
		return nil, fmt.Errorf("node info [%s] with wrong format. check if \"IP[:Port][ weight=XXXX]\"", info)
	}
	ip := g[0][1]
	port := g[0][2]
	weight := g[0][3]

	var portInt uint64
	portInt, err = strconv.ParseUint(port, 10, 16)
	if err != nil {
		portInt = defaultPort
	}
	weightInt, err := strconv.ParseUint(weight, 10, 16)
	if err != nil {
		weightInt = defaultWeight
	}
	return &Node{
		IP:     ip,
		Port:   uint16(portInt),
		Weight: uint16(weightInt),
	}, nil
}
