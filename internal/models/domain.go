package models

import (
	"time"
)

// Domain represents a domain record in the database
type Domain struct {
	ID            uint      `gorm:"primarykey" json:"id"`
	Name          string    `gorm:"uniqueIndex;not null" json:"name"`        // Domain name
	Registrar     string    `json:"registrar"`                                // Registrar
	ExpiryDate    time.Time `json:"expiry_date"`                              // Expiration date
	CreatedDate   time.Time `json:"created_date"`                             // Registration date
	UpdatedDate   time.Time `json:"updated_date"`                             // Update date
	Status        string    `json:"status"`                                   // Domain status
	DaysRemaining int       `json:"days_remaining"`                           // Days remaining
	Tags          string    `json:"tags"`                                     // Tags (JSON or comma separated)
	LastChecked   time.Time `json:"last_checked"`                             // Last check time
	IsActive      bool      `gorm:"default:true" json:"is_active"`            // Monitor enabled
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

// Notification represents a notification record
type Notification struct {
	ID       uint      `gorm:"primarykey" json:"id"`
	DomainID uint      `json:"domain_id"`                      // Associated domain
	Type     string    `json:"type"`                           // Notification type (email/webhook/telegram)
	Content  string    `json:"content"`                        // Notification content
	Status   string    `json:"status"`                         // Send status (success/failed)
	SentAt   time.Time `json:"sent_at"`
}

// Setting represents system configuration
type Setting struct {
	Key   string `gorm:"primarykey" json:"key"`
	Value string `json:"value"`
}

// User represents a user account
type User struct {
	ID        uint      `gorm:"primarykey" json:"id"`
	Username  string    `gorm:"uniqueIndex;not null" json:"username"` // Username
	Password  string    `gorm:"not null" json:"-"`                    // Hashed password (excluded from JSON)
	Email     string    `json:"email"`                                // Email
	IsActive  bool      `gorm:"default:true" json:"is_active"`        // Account status
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}
