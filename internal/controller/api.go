package controller

import (
	"crypto/rand"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"net"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/unicornultrafoundation/zerogo/internal/protocol"
)

// SetupRoutes configures all API routes.
func (ctrl *Controller) SetupRoutes(r *gin.Engine) {
	// Public routes
	r.POST("/api/v1/auth/login", ctrl.handleLogin)
	r.POST("/api/v1/auth/register", ctrl.handleRegister)

	// Agent WebSocket (authenticated via headers)
	r.GET("/api/v1/agent/connect", ctrl.ws.HandleAgentConnect)

	// Protected API routes
	api := r.Group("/api/v1")
	api.Use(AuthMiddleware(ctrl.jwtSecret))
	{
		// Networks
		api.GET("/networks", ctrl.listNetworks)
		api.POST("/networks", ctrl.createNetwork)
		api.GET("/networks/:id", ctrl.getNetwork)
		api.PUT("/networks/:id", ctrl.updateNetwork)
		api.DELETE("/networks/:id", ctrl.deleteNetwork)

		// Members
		api.GET("/networks/:id/members", ctrl.listMembers)
		api.POST("/networks/:id/members", ctrl.authorizeMember)
		api.PUT("/networks/:id/members/:nid", ctrl.updateMember)
		api.DELETE("/networks/:id/members/:nid", ctrl.removeMember)

		// Peers (real-time status)
		api.GET("/peers", ctrl.listPeers)
	}
}

// --- Auth handlers ---

func (ctrl *Controller) handleLogin(c *gin.Context) {
	var req protocol.LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	var user User
	if err := ctrl.db.Where("username = ?", req.Username).First(&user).Error; err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid credentials"})
		return
	}

	if !CheckPassword(req.Password, user.Password) {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid credentials"})
		return
	}

	token, expiresAt, err := GenerateToken(&user, ctrl.jwtSecret)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "generate token failed"})
		return
	}

	c.JSON(http.StatusOK, protocol.LoginResponse{
		Token:     token,
		ExpiresAt: expiresAt,
	})
}

func (ctrl *Controller) handleRegister(c *gin.Context) {
	var req protocol.LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Check if any users exist (first user can register freely)
	var count int64
	ctrl.db.Model(&User{}).Count(&count)
	if count > 0 {
		// Require authentication for subsequent registrations
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			c.JSON(http.StatusForbidden, gin.H{"error": "registration requires admin authentication"})
			return
		}
	}

	hash, err := HashPassword(req.Password)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "hash password failed"})
		return
	}

	user := User{
		Username: req.Username,
		Password: hash,
		Role:     "admin",
	}
	if err := ctrl.db.Create(&user).Error; err != nil {
		c.JSON(http.StatusConflict, gin.H{"error": "username already exists"})
		return
	}

	c.JSON(http.StatusCreated, gin.H{"id": user.ID, "username": user.Username})
}

// --- Network handlers ---

func (ctrl *Controller) listNetworks(c *gin.Context) {
	var networks []Network
	ctrl.db.Find(&networks)

	online := ctrl.ws.GetOnlineAgents()
	result := make([]protocol.Network, 0, len(networks))
	for _, n := range networks {
		var memberCount int64
		ctrl.db.Model(&Member{}).Where("network_id = ?", n.ID).Count(&memberCount)

		var onlineCount int
		var members []Member
		ctrl.db.Where("network_id = ? AND authorized = ?", n.ID, true).Find(&members)
		for _, m := range members {
			if online[m.NodeAddress] {
				onlineCount++
			}
		}

		result = append(result, protocol.Network{
			ID:          n.ID,
			Name:        n.Name,
			Description: n.Description,
			IPRange:     n.IPRange,
			IP6Range:    n.IP6Range,
			MTU:         n.MTU,
			Multicast:   n.Multicast,
			MemberCount: int(memberCount),
			OnlineCount: onlineCount,
			CreatedAt:   n.CreatedAt,
		})
	}
	c.JSON(http.StatusOK, result)
}

func (ctrl *Controller) createNetwork(c *gin.Context) {
	var req protocol.CreateNetworkRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Generate random 32-bit network ID
	var idBytes [4]byte
	rand.Read(idBytes[:])
	networkID := binary.BigEndian.Uint32(idBytes[:])

	mtu := req.MTU
	if mtu == 0 {
		mtu = 2800
	}
	multicast := true
	if req.Multicast != nil {
		multicast = *req.Multicast
	}

	// Generate per-network PSK (32 bytes)
	var pskBytes [32]byte
	rand.Read(pskBytes[:])
	pskHex := hex.EncodeToString(pskBytes[:])

	network := Network{
		ID:          networkID,
		Name:        req.Name,
		Description: req.Description,
		IPRange:     req.IPRange,
		IP6Range:    req.IP6Range,
		MTU:         mtu,
		Multicast:   multicast,
		PSK:         pskHex,
	}

	if err := ctrl.db.Create(&network).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "create network failed"})
		return
	}

	c.JSON(http.StatusCreated, protocol.Network{
		ID:        network.ID,
		Name:      network.Name,
		IPRange:   network.IPRange,
		MTU:       network.MTU,
		Multicast: network.Multicast,
		CreatedAt: network.CreatedAt,
	})
}

func (ctrl *Controller) getNetwork(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid network ID"})
		return
	}

	var network Network
	if err := ctrl.db.First(&network, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "network not found"})
		return
	}

	c.JSON(http.StatusOK, network)
}

func (ctrl *Controller) updateNetwork(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid network ID"})
		return
	}

	var network Network
	if err := ctrl.db.First(&network, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "network not found"})
		return
	}

	var req protocol.CreateNetworkRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	updates := map[string]interface{}{}
	if req.Name != "" {
		updates["name"] = req.Name
	}
	if req.Description != "" {
		updates["description"] = req.Description
	}
	if req.IPRange != "" {
		updates["ip_range"] = req.IPRange
	}
	if req.MTU > 0 {
		updates["mtu"] = req.MTU
	}
	if req.Multicast != nil {
		updates["multicast"] = *req.Multicast
	}

	ctrl.db.Model(&network).Updates(updates)
	ctrl.db.First(&network, id)

	c.JSON(http.StatusOK, network)
}

func (ctrl *Controller) deleteNetwork(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid network ID"})
		return
	}
	ctrl.db.Delete(&Network{}, id)
	c.JSON(http.StatusOK, gin.H{"deleted": true})
}

// --- Member handlers ---

func (ctrl *Controller) listMembers(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid network ID"})
		return
	}

	var members []Member
	ctrl.db.Where("network_id = ?", id).Preload("Node").Find(&members)

	online := ctrl.ws.GetOnlineAgents()
	result := make([]protocol.Member, 0, len(members))
	for _, m := range members {
		result = append(result, protocol.Member{
			NetworkID:   m.NetworkID,
			NodeAddress: m.NodeAddress,
			Authorized:  m.Authorized,
			IPAddress:   m.IPAddress,
			Name:        m.Name,
			Online:      online[m.NodeAddress],
			Platform:    m.Node.Platform,
			LastSeen:    m.Node.LastSeen,
			CreatedAt:   m.CreatedAt,
		})
	}
	c.JSON(http.StatusOK, result)
}

func (ctrl *Controller) authorizeMember(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid network ID"})
		return
	}

	var req protocol.AuthorizeMemberRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Get network for IP allocation
	var network Network
	if err := ctrl.db.First(&network, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "network not found"})
		return
	}

	// Auto-allocate IP if authorizing and no IP specified
	if req.Authorized && req.IPAddress == "" {
		allocatedIP, err := ctrl.allocateIP(network)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "IP allocation failed: " + err.Error()})
			return
		}
		req.IPAddress = allocatedIP
	}

	member := Member{
		NetworkID:   uint32(id),
		NodeAddress: req.NodeAddress,
		Authorized:  req.Authorized,
		IPAddress:   req.IPAddress,
		Name:        req.Name,
	}

	result := ctrl.db.Where("network_id = ? AND node_address = ?", id, req.NodeAddress).
		Assign(member).FirstOrCreate(&member)
	if result.Error != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "authorize member failed"})
		return
	}

	// If authorizing, push full network config to the agent and notify other peers
	if req.Authorized {
		var node Node
		if err := ctrl.db.First(&node, "address = ?", req.NodeAddress).Error; err == nil {
			// Push network config to the newly authorized agent
			ctrl.ws.SendNetworkConfigToAgent(req.NodeAddress, fmt.Sprintf("%d", id))

			// Notify all other connected agents about the new peer
			ctrl.ws.BroadcastPeerUpdate(uint32(id), "add", protocol.PeerInfo{
				Address:   node.Address,
				PublicKey: node.PublicKey,
				Name:      req.Name,
			})
		}
	}

	c.JSON(http.StatusOK, member)
}

// allocateIP finds the next available IP in the network's range.
func (ctrl *Controller) allocateIP(network Network) (string, error) {
	_, ipNet, err := net.ParseCIDR(network.IPRange)
	if err != nil {
		return "", fmt.Errorf("invalid IP range: %w", err)
	}

	// Get all used IPs in this network
	var members []Member
	ctrl.db.Where("network_id = ? AND ip_address != ''", network.ID).Find(&members)
	usedIPs := make(map[string]bool)
	for _, m := range members {
		// Extract IP from CIDR notation if present
		ip, _, err := net.ParseCIDR(m.IPAddress)
		if err != nil {
			ip = net.ParseIP(m.IPAddress)
		}
		if ip != nil {
			usedIPs[ip.String()] = true
		}
	}

	// Get mask size for CIDR notation
	ones, _ := ipNet.Mask.Size()

	// Iterate through IPs, skip network and broadcast
	ip := make(net.IP, len(ipNet.IP))
	copy(ip, ipNet.IP)
	for inc(ip); ipNet.Contains(ip); inc(ip) {
		// Skip broadcast (last IP)
		ipCopy := make(net.IP, len(ip))
		copy(ipCopy, ip)
		inc(ipCopy)
		if !ipNet.Contains(ipCopy) {
			break // this is the broadcast address
		}
		if !usedIPs[ip.String()] {
			return fmt.Sprintf("%s/%d", ip.String(), ones), nil
		}
	}
	return "", fmt.Errorf("no available IPs in range %s", network.IPRange)
}

// inc increments an IP address by one.
func inc(ip net.IP) {
	for j := len(ip) - 1; j >= 0; j-- {
		ip[j]++
		if ip[j] > 0 {
			break
		}
	}
}

func (ctrl *Controller) updateMember(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid network ID"})
		return
	}
	nodeAddr := c.Param("nid")

	var req protocol.AuthorizeMemberRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	updates := map[string]interface{}{}
	updates["authorized"] = req.Authorized
	if req.IPAddress != "" {
		updates["ip_address"] = req.IPAddress
	}
	if req.Name != "" {
		updates["name"] = req.Name
	}

	result := ctrl.db.Model(&Member{}).
		Where("network_id = ? AND node_address = ?", id, nodeAddr).
		Updates(updates)
	if result.RowsAffected == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "member not found"})
		return
	}

	var member Member
	ctrl.db.First(&member, "network_id = ? AND node_address = ?", id, nodeAddr)
	c.JSON(http.StatusOK, member)
}

func (ctrl *Controller) removeMember(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid network ID"})
		return
	}
	nodeAddr := c.Param("nid")

	ctrl.db.Where("network_id = ? AND node_address = ?", id, nodeAddr).Delete(&Member{})

	// Notify peers
	ctrl.ws.BroadcastPeerUpdate(uint32(id), "remove", protocol.PeerInfo{Address: nodeAddr})

	c.JSON(http.StatusOK, gin.H{"deleted": true})
}

// --- Peer status ---

func (ctrl *Controller) listPeers(c *gin.Context) {
	online := ctrl.ws.GetOnlineAgents()
	type PeerWithStatus struct {
		Address  string    `json:"address"`
		Platform string    `json:"platform"`
		Online   bool      `json:"online"`
		LastSeen time.Time `json:"last_seen"`
	}

	var nodes []Node
	ctrl.db.Find(&nodes)

	result := make([]PeerWithStatus, 0, len(nodes))
	for _, n := range nodes {
		result = append(result, PeerWithStatus{
			Address:  n.Address,
			Platform: n.Platform,
			Online:   online[n.Address],
			LastSeen: n.LastSeen,
		})
	}
	c.JSON(http.StatusOK, result)
}
