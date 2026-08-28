package store

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"sync"
	"time"

	"github.com/relentlessworks/convertkit/internal/model"
)

// Store is a JSON file-backed data store.
type Store struct {
	mu   sync.RWMutex
	path string
	data *dbData
}

type dbData struct {
	Workspaces map[string]*model.Workspace `json:"workspaces"`
	Tokens     map[string]*model.Token     `json:"tokens"`
	OTPs       map[string]*model.OTP       `json:"otps"`
	// Conversions keyed by workspace handle
	Conversions map[string][]*model.ConversionRecord `json:"conversions"`
	AuditLog    []model.AuditEntry                   `json:"audit_log"`
}

// New creates or opens a store at the given path.
func New(path string) (*Store, error) {
	s := &Store{
		path: path,
		data: &dbData{
			Workspaces:  make(map[string]*model.Workspace),
			Tokens:      make(map[string]*model.Token),
			OTPs:        make(map[string]*model.OTP),
			Conversions: make(map[string][]*model.ConversionRecord),
			AuditLog:    []model.AuditEntry{},
		},
	}

	if err := s.load(); err != nil {
		return nil, err
	}
	return s, nil
}

func (s *Store) load() error {
	data, err := os.ReadFile(s.path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	if len(data) == 0 {
		return nil
	}
	return json.Unmarshal(data, s.data)
}

func (s *Store) save() error {
	data, err := json.MarshalIndent(s.data, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(s.path, data, 0644)
}

// --- Workspace ---

func (s *Store) CreateWorkspace(ws *model.Workspace) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.data.Workspaces[ws.Handle] = ws
	return s.save()
}

func (s *Store) GetWorkspaceByHandle(handle string) (*model.Workspace, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	ws, ok := s.data.Workspaces[handle]
	if !ok {
		return nil, fmt.Errorf("workspace not found")
	}
	return ws, nil
}

func (s *Store) GetWorkspaceByEmail(email string) (*model.Workspace, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, ws := range s.data.Workspaces {
		if ws.Email == email {
			return ws, nil
		}
	}
	return nil, fmt.Errorf("workspace not found")
}

// --- OTP ---

func (s *Store) SaveOTP(otp *model.OTP) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.data.OTPs[otp.Email] = otp
	return s.save()
}

func (s *Store) GetOTP(email string) (*model.OTP, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	otp, ok := s.data.OTPs[email]
	if !ok {
		return nil, fmt.Errorf("no OTP found")
	}
	if time.Now().After(otp.ExpiresAt) {
		return nil, fmt.Errorf("OTP expired")
	}
	return otp, nil
}

func (s *Store) DeleteOTP(email string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.data.OTPs, email)
	s.save()
}

// --- Token ---

func (s *Store) SaveToken(t *model.Token) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.data.Tokens[t.Token] = t
	return s.save()
}

func (s *Store) GetToken(token string) (*model.Token, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	t, ok := s.data.Tokens[token]
	if !ok {
		return nil, fmt.Errorf("invalid token")
	}
	return t, nil
}

// --- Conversions ---

func (s *Store) SaveConversion(wsHandle string, rec *model.ConversionRecord) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.data.Conversions[wsHandle] = append(s.data.Conversions[wsHandle], rec)
	// Keep only last 100
	if len(s.data.Conversions[wsHandle]) > 100 {
		s.data.Conversions[wsHandle] = s.data.Conversions[wsHandle][len(s.data.Conversions[wsHandle])-100:]
	}
	return s.save()
}

func (s *Store) ListConversions(wsHandle string, limit int) ([]*model.ConversionRecord, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	records := s.data.Conversions[wsHandle]
	// Return in reverse order (newest first)
	result := make([]*model.ConversionRecord, 0, len(records))
	for i := len(records) - 1; i >= 0; i-- {
		result = append(result, records[i])
		if limit > 0 && len(result) >= limit {
			break
		}
	}
	return result, nil
}

func (s *Store) GetConversion(wsHandle, handle string) (*model.ConversionRecord, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, rec := range s.data.Conversions[wsHandle] {
		if rec.Handle == handle {
			return rec, nil
		}
	}
	return nil, fmt.Errorf("conversion not found")
}

// --- Audit ---

func (s *Store) AddAudit(entry model.AuditEntry) {
	s.mu.Lock()
	defer s.mu.Unlock()
	entry.ID = len(s.data.AuditLog) + 1
	entry.Timestamp = time.Now()
	s.data.AuditLog = append(s.data.AuditLog, entry)
	// Keep last 1000
	if len(s.data.AuditLog) > 1000 {
		s.data.AuditLog = s.data.AuditLog[len(s.data.AuditLog)-1000:]
	}
	s.save()
}

func (s *Store) ListAudit(wsHandle string, limit int) ([]model.AuditEntry, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var result []model.AuditEntry
	for i := len(s.data.AuditLog) - 1; i >= 0; i-- {
		if s.data.AuditLog[i].Handle == wsHandle {
			result = append(result, s.data.AuditLog[i])
			if limit > 0 && len(result) >= limit {
				break
			}
		}
	}
	// Sort by timestamp descending
	sort.Slice(result, func(i, j int) bool {
		return result[i].Timestamp.After(result[j].Timestamp)
	})
	return result, nil
}
