package auth

import (
	"fmt"
	"os"
	"time"

	"github.com/relentlessworks/convertkit/internal/model"
	"github.com/relentlessworks/convertkit/internal/store"
)

// AuthService handles OTP-based authentication.
type AuthService struct {
	store *store.Store
}

// New creates a new auth service.
func New(s *store.Store) *AuthService {
	return &AuthService{store: s}
}

// RequestOTP generates and stores an OTP for the given email.
// If no SMTP is configured, the OTP is logged to stderr.
func (a *AuthService) RequestOTP(email string) (string, error) {
	// Find or create workspace
	ws, err := a.store.GetWorkspaceByEmail(email)
	if err != nil {
		// Create new workspace
		ws = &model.Workspace{
			Handle:    model.GenerateHandle("ws"),
			Name:      email,
			Email:     email,
			Plan:      "free",
			CreatedAt: time.Now(),
		}
		if err := a.store.CreateWorkspace(ws); err != nil {
			return "", fmt.Errorf("failed to create workspace: %v", err)
		}
	}

	code := model.GenerateOTPCode()
	otp := &model.OTP{
		Email:     email,
		Code:      code,
		ExpiresAt: time.Now().Add(10 * time.Minute),
	}
	if err := a.store.SaveOTP(otp); err != nil {
		return "", fmt.Errorf("failed to save OTP: %v", err)
	}

	// Log OTP to stderr (dev mode — no SMTP configured)
	fmt.Fprintf(os.Stderr, "convertkit: OTP for %s: %s\n", email, code)

	return ws.Handle, nil
}

// VerifyOTP validates an OTP and returns a bearer token.
func (a *AuthService) VerifyOTP(email, code string) (*model.Token, error) {
	otp, err := a.store.GetOTP(email)
	if err != nil {
		return nil, fmt.Errorf("invalid or expired OTP")
	}
	if otp.Code != code {
		return nil, fmt.Errorf("invalid OTP code")
	}

	ws, err := a.store.GetWorkspaceByEmail(email)
	if err != nil {
		return nil, fmt.Errorf("workspace not found")
	}

	token := &model.Token{
		Token:     model.GenerateToken(),
		Email:     email,
		Workspace: ws.Handle,
		CreatedAt: time.Now(),
	}
	if err := a.store.SaveToken(token); err != nil {
		return nil, fmt.Errorf("failed to save token: %v", err)
	}

	a.store.DeleteOTP(email)

	a.store.AddAudit(model.AuditEntry{
		Handle: ws.Handle,
		Action: "auth.verify",
		Detail: fmt.Sprintf("token created for %s", email),
	})

	return token, nil
}

// ValidateToken checks a bearer token and returns the associated workspace handle.
func (a *AuthService) ValidateToken(token string) (string, error) {
	t, err := a.store.GetToken(token)
	if err != nil {
		return "", fmt.Errorf("invalid token")
	}
	return t.Workspace, nil
}
