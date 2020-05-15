package httplb

// 根据旧的client和新的节点信息，判断是否需要更新节点
// 如果未更新，则第二个参数返回false
// 如果已更新，第一个参数返回新的HTTP Clients
func updateClients(oldClients []Client, nodes []*Node, opts *Opts) ([]Client, bool) {
	if len(nodes) == 0 {
		return oldClients, false
	}
	if len(oldClients) == 0 {
		return createClients(nodes, opts), true
	}

	if equals(oldClients, nodes) {
		return oldClients, false
	}

	oldClientMap := make(map[string]Client)
	newClients := make([]Client, 0, len(nodes))
	if len(oldClients) > 0 {
		for _, c := range oldClients {
			oldClientMap[c.Name()] = c
		}
	}
	for _, n := range nodes {
		if oc := oldClientMap[n.String()]; oc != nil {
			newClients = append(newClients, oc)
		} else {
			c := NewHostClient(n, opts)
			newClients = append(newClients, c)
		}
	}
	return newClients, true
}

// 直接创建指定机器列表的HTTP Clients
func createClients(nodes []*Node, opts *Opts) []Client {
	if len(nodes) == 0 {
		return nil
	}
	clients := make([]Client, 0, len(nodes))
	for _, node := range nodes {
		c := NewHostClient(node, opts)
		clients = append(clients, c)
	}
	return clients
}

// 判断新的节点和旧的clients是否一致
func equals(oldClients []Client, nodes []*Node) bool {
	if len(oldClients) != len(nodes) {
		return false
	}
	oldClientMap := make(map[string]bool)
	for _, c := range oldClients {
		oldClientMap[c.Name()] = true
	}

	for _, n := range nodes {
		if !oldClientMap[n.String()] {
			return false
		}
	}
	return true
}
