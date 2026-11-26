package repository

import "time"

// NodeOperation stores the state of an operator's command.
type NodeOperation struct {
	CommandID     string `gorm:"primaryKey;size:128"`
	NodeID        int64  `gorm:"index:idx_node_operations_history,priority:1"`
	CommandType   string
	Payload       string
	Reason        string
	Status        string
	Message       string
	CreatedAt     time.Time `gorm:"index:idx_node_operations_history,priority:2,sort:desc"`
	UpdatedAt     time.Time
	DispatchedAt  time.Time
	AgentInstance string
	Node          *Node `gorm:"foreignKey:NodeID;constraint:OnDelete:CASCADE"`
}
