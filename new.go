package httplb

const (
	LBRoundRobin         = iota + 1 // TODO 轮循
	LBRandom                        // TODO 随机
	LBWeightedRoundRobin            // 加权轮询
	LBLeastConnection               // 最小连接数

	TypeDNS    = "dns"
	TypeConsul = "consul"
	TypeStatic = "static"

	DNSTypeA   = "A"   // DNS A记录
	DNSTypeSRV = "SRV" // DNS SRV记录
)

// New 创建HTTP负载均衡实例
func New(config *Config) LoadBalancer {
	switch config.LBStrategy {
	case LBRandom:
		return NewRandomLB(config)
	case LBRoundRobin:
		return NewRoundRobinLB(config)
	case LBLeastConnection:
		return NewLeastLB(config)
	case LBWeightedRoundRobin:
		return NewWeightedRoundRobinLB(config)
	default:
		return NewLeastLB(config)
	}
}
