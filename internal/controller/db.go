package controller

import (
	"fmt"
	"strings"
	"time"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// --- GORM Models ---

// User represents an admin user.
type User struct {
	ID        uint      `gorm:"primarykey" json:"id"`
	Username  string    `gorm:"uniqueIndex;not null" json:"username"`
	Password  string    `gorm:"not null" json:"-"` // bcrypt hash
	Role      string    `gorm:"default:admin" json:"role"`
	CreatedAt time.Time `json:"created_at"`
}

// Network represents a virtual network.
type Network struct {
	ID          uint32    `gorm:"primarykey" json:"id"`
	Name        string    `gorm:"not null" json:"name"`
	Description string    `json:"description,omitempty"`
	IPRange     string    `gorm:"not null" json:"ip_range"`
	IP6Range    string    `json:"ip6_range,omitempty"`
	MTU         int       `gorm:"default:2800" json:"mtu"`
	Multicast   bool      `gorm:"default:true" json:"multicast"`
	PSK         string    `gorm:"not null" json:"-"` // Per-network PSK (hex), not exposed in JSON
	CreatedAt   time.Time `json:"created_at"`
	Members     []Member  `gorm:"foreignKey:NetworkID" json:"members,omitempty"`
	Rules       []Rule    `gorm:"foreignKey:NetworkID" json:"rules,omitempty"`
}

// Node represents a registered device.
type Node struct {
	Address     string    `gorm:"primarykey" json:"address"`
	PublicKey   string    `gorm:"not null" json:"public_key"`
	Name        string    `json:"name,omitempty"`
	Description string    `json:"description,omitempty"`
	Platform    string    `json:"platform,omitempty"`
	LastSeen    time.Time `json:"last_seen,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
}

// Member represents network membership.
type Member struct {
	NetworkID   uint32    `gorm:"primaryKey" json:"network_id"`
	NodeAddress string    `gorm:"primaryKey" json:"node_address"`
	Authorized  bool      `gorm:"default:false" json:"authorized"`
	IPAddress   string    `json:"ip_address,omitempty"`
	Name        string    `json:"name,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
	Node        Node      `gorm:"foreignKey:NodeAddress;references:Address" json:"node,omitempty"`
}

// Rule represents an ACL rule.
type Rule struct {
	ID          uint      `gorm:"primarykey" json:"id"`
	NetworkID   uint32    `json:"network_id"`
	Priority    int       `gorm:"default:100" json:"priority"`
	Action      string    `gorm:"not null" json:"action"` // allow, drop
	Src         string    `json:"src,omitempty"`
	Dst         string    `json:"dst,omitempty"`
	Protocol    string    `json:"protocol,omitempty"`
	PortRange   string    `json:"port_range,omitempty"`
	Description string    `json:"description,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
}

// InitDB initializes the database connection and runs migrations.
func InitDB(dsn string) (*gorm.DB, error) {
	var db *gorm.DB
	var err error

	// Parse DSN: "sqlite:///path/to/db" or "postgres://..."
	if strings.HasPrefix(dsn, "sqlite://") {
		dbPath := strings.TrimPrefix(dsn, "sqlite://")
		db, err = gorm.Open(sqlite.Open(dbPath), &gorm.Config{
			Logger: logger.Default.LogMode(logger.Warn),
		})
	} else {
		return nil, fmt.Errorf("unsupported database DSN: %s (only sqlite:// supported in MVP)", dsn)
	}
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}

	// Run migrations
	if err := db.AutoMigrate(&User{}, &Network{}, &Node{}, &Member{}, &Rule{}); err != nil {
		return nil, fmt.Errorf("migrate database: %w", err)
	}

	return db, nil
}
