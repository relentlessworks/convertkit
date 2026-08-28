package model

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"time"
)

// ConversionRecord stores a single conversion operation.
type ConversionRecord struct {
	Handle    string    `json:"handle"`
	FromFmt   string    `json:"from"`
	ToFmt     string    `json:"to"`
	Input     string    `json:"input"`
	Output    string    `json:"output"`
	CreatedAt time.Time `json:"created_at"`
}

// Workspace represents a tenant in the system.
type Workspace struct {
	Handle    string    `json:"handle"`
	Name      string    `json:"name"`
	Email     string    `json:"email"`
	Plan      string    `json:"plan"`
	CreatedAt time.Time `json:"created_at"`
}

// AuditEntry records an action taken by a user.
type AuditEntry struct {
	ID        int       `json:"id"`
	Handle    string    `json:"handle"`
	Action    string    `json:"action"`
	Detail    string    `json:"detail"`
	Timestamp time.Time `json:"timestamp"`
}

// Token represents an auth token.
type Token struct {
	Token     string    `json:"token"`
	Email     string    `json:"email"`
	Workspace string    `json:"workspace"`
	CreatedAt time.Time `json:"created_at"`
}

// OTP represents a one-time password.
type OTP struct {
	Email     string    `json:"email"`
	Code      string    `json:"code"`
	ExpiresAt time.Time `json:"expires_at"`
}

// GenerateHandle creates a short unique handle with a type prefix.
func GenerateHandle(prefix string) string {
	b := make([]byte, 4)
	rand.Read(b)
	return fmt.Sprintf("%s_%s", prefix, hex.EncodeToString(b))
}

// GenerateToken creates a random auth token.
func GenerateToken() string {
	b := make([]byte, 32)
	rand.Read(b)
	return hex.EncodeToString(b)
}

// GenerateOTPCode creates a 6-digit OTP code.
func GenerateOTPCode() string {
	b := make([]byte, 4)
	rand.Read(b)
	code := int(b[0])<<24 | int(b[1])<<16 | int(b[2])<<8 | int(b[3])
	if code < 0 {
		code = -code
	}
	return fmt.Sprintf("%06d", code%1000000)
}
