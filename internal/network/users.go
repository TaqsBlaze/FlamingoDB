package network

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"sync"
)

// Policy defines what operations a user is allowed to perform.
type Policy struct {
	CanSelect bool `json:"can_select"` // SELECT queries
	CanInsert bool `json:"can_insert"` // INSERT statements
	CanUpdate bool `json:"can_update"` // UPDATE statements
	CanDelete bool `json:"can_delete"` // DELETE statements
	CanCreate bool `json:"can_create"` // CREATE TABLE
	CanDrop   bool `json:"can_drop"`   // DROP TABLE
}

// NamedPolicy represents a reusable policy with a name.
type NamedPolicy struct {
	Name      string `json:"name"`
	CanSelect bool   `json:"can_select"`
	CanInsert bool   `json:"can_insert"`
	CanUpdate bool   `json:"can_update"`
	CanDelete bool   `json:"can_delete"`
	CanCreate bool   `json:"can_create"`
	CanDrop   bool   `json:"can_drop"`
}

// DBUser represents a database user with credentials, policy name, and resolved policy.
type DBUser struct {
	Username   string `json:"username"`
	Password   string `json:"password"`
	IsAdmin    bool   `json:"is_admin"`
	PolicyName string `json:"policy_name"` // e.g. "Read-Only", "Read-Write", etc.
	Policy     Policy `json:"policy"`      // Resolved policy permissions
}

// AdminPolicy returns a Policy with all permissions enabled.
func AdminPolicy() Policy {
	return Policy{
		CanSelect: true,
		CanInsert: true,
		CanUpdate: true,
		CanDelete: true,
		CanCreate: true,
		CanDrop:   true,
	}
}

// UserStore manages database users, persisting them to a JSON file.
type UserStore struct {
	mu       sync.RWMutex
	users    map[string]*DBUser
	filePath string
}

// NewUserStore loads an existing user store or creates a new one with the admin seed.
func NewUserStore(filePath string, adminUsername, adminPassword string) (*UserStore, error) {
	us := &UserStore{
		users:    make(map[string]*DBUser),
		filePath: filePath,
	}

	needsSave := false

	// Try loading from disk
	if data, err := os.ReadFile(filePath); err == nil {
		var users []*DBUser
		if err := json.Unmarshal(data, &users); err == nil {
			for _, u := range users {
				if u.Username != "" {
					if !isHashed(u.Password) && u.Password != "" {
						u.Password = hashPassword(u.Password)
						needsSave = true
					}
					us.users[u.Username] = u
				}
			}
		}
	}

	// Ensure admin user always exists (do not overwrite if already stored)
	if _, exists := us.users[adminUsername]; !exists {
		us.users[adminUsername] = &DBUser{
			Username:   adminUsername,
			Password:   hashPassword(adminPassword),
			IsAdmin:    true,
			PolicyName: "Admin",
			Policy:     AdminPolicy(),
		}
		needsSave = true
	}

	if needsSave {
		_ = us.save()
	}

	return us, nil
}

// Authenticate checks credentials and returns the user if valid.
func (us *UserStore) Authenticate(username, password string) (*DBUser, bool) {
	us.mu.RLock()
	defer us.mu.RUnlock()
	u, ok := us.users[username]
	if !ok || u.Password != hashPassword(password) {
		return nil, false
	}
	return u, true
}

// GetUser returns a user by name.
func (us *UserStore) GetUser(username string) (*DBUser, bool) {
	us.mu.RLock()
	defer us.mu.RUnlock()
	u, ok := us.users[username]
	return u, ok
}

// ListUsers returns all users (passwords redacted).
func (us *UserStore) ListUsers() []*DBUser {
	us.mu.RLock()
	defer us.mu.RUnlock()
	list := make([]*DBUser, 0, len(us.users))
	for _, u := range us.users {
		safe := *u
		safe.Password = "" // never expose passwords
		list = append(list, &safe)
	}
	return list
}

// CreateUser adds a new user. Returns error if username already exists.
func (us *UserStore) CreateUser(username, password string, isAdmin bool, policyName string, policy Policy) error {
	us.mu.Lock()
	defer us.mu.Unlock()
	if _, exists := us.users[username]; exists {
		return &userError{"user already exists: " + username}
	}
	if isAdmin {
		policyName = "Admin"
		policy = AdminPolicy()
	}
	us.users[username] = &DBUser{
		Username:   username,
		Password:   hashPassword(password),
		IsAdmin:    isAdmin,
		PolicyName: policyName,
		Policy:     policy,
	}
	return us.save()
}

// DeleteUser removes a user. Prevents deleting the last admin.
func (us *UserStore) DeleteUser(username string) error {
	us.mu.Lock()
	defer us.mu.Unlock()
	u, ok := us.users[username]
	if !ok {
		return &userError{"user not found: " + username}
	}
	if u.IsAdmin {
		// Count admins
		count := 0
		for _, v := range us.users {
			if v.IsAdmin {
				count++
			}
		}
		if count <= 1 {
			return &userError{"cannot delete the last admin user"}
		}
	}
	delete(us.users, username)
	return us.save()
}

// UpdatePolicy replaces a user's policy.
func (us *UserStore) UpdatePolicy(username string, policy Policy, isAdmin bool, policyName string) error {
	us.mu.Lock()
	defer us.mu.Unlock()
	u, ok := us.users[username]
	if !ok {
		return &userError{"user not found: " + username}
	}
	u.IsAdmin = isAdmin
	u.PolicyName = policyName
	if isAdmin {
		u.PolicyName = "Admin"
		u.Policy = AdminPolicy()
	} else {
		u.Policy = policy
	}
	return us.save()
}

// UpdatePassword changes a user's password.
func (us *UserStore) UpdatePassword(username, newPassword string) error {
	us.mu.Lock()
	defer us.mu.Unlock()
	u, ok := us.users[username]
	if !ok {
		return &userError{"user not found: " + username}
	}
	u.Password = hashPassword(newPassword)
	return us.save()
}

// save serializes users to disk (must be called with lock held).
func (us *UserStore) save() error {
	list := make([]*DBUser, 0, len(us.users))
	for _, u := range us.users {
		list = append(list, u)
	}
	data, err := json.MarshalIndent(list, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(us.filePath, data, 0600)
}

// PolicyStore manages named policies, persisting them to a JSON file.
type PolicyStore struct {
	mu       sync.RWMutex
	policies map[string]*NamedPolicy
	filePath string
}

// NewPolicyStore loads an existing policy store or creates a new one with default policies.
func NewPolicyStore(filePath string) (*PolicyStore, error) {
	ps := &PolicyStore{
		policies: make(map[string]*NamedPolicy),
		filePath: filePath,
	}

	// Try loading from disk
	if data, err := os.ReadFile(filePath); err == nil {
		var list []*NamedPolicy
		if err := json.Unmarshal(data, &list); err == nil {
			for _, p := range list {
				ps.policies[p.Name] = p
			}
		}
	}

	// Seed default policies if empty
	if len(ps.policies) == 0 {
		ps.policies["Read-Only"] = &NamedPolicy{
			Name:      "Read-Only",
			CanSelect: true,
		}
		ps.policies["Read-Write"] = &NamedPolicy{
			Name:      "Read-Write",
			CanSelect: true,
			CanInsert: true,
			CanUpdate: true,
			CanDelete: true,
		}
		ps.policies["Schema-Admin"] = &NamedPolicy{
			Name:      "Schema-Admin",
			CanSelect: true,
			CanInsert: true,
			CanUpdate: true,
			CanDelete: true,
			CanCreate: true,
			CanDrop:   true,
		}
		_ = ps.save()
	}

	return ps, nil
}

// List returns all named policies.
func (ps *PolicyStore) List() []*NamedPolicy {
	ps.mu.RLock()
	defer ps.mu.RUnlock()
	list := make([]*NamedPolicy, 0, len(ps.policies))
	for _, p := range ps.policies {
		list = append(list, p)
	}
	return list
}

// Get returns a named policy by name.
func (ps *PolicyStore) Get(name string) (*NamedPolicy, bool) {
	ps.mu.RLock()
	defer ps.mu.RUnlock()
	p, ok := ps.policies[name]
	return p, ok
}

// Set adds or updates a named policy.
func (ps *PolicyStore) Set(name string, p *NamedPolicy) error {
	ps.mu.Lock()
	defer ps.mu.Unlock()
	if name == "" {
		return &userError{"policy name cannot be empty"}
	}
	p.Name = name
	ps.policies[name] = p
	return ps.save()
}

// Delete removes a named policy.
func (ps *PolicyStore) Delete(name string) error {
	ps.mu.Lock()
	defer ps.mu.Unlock()
	if _, ok := ps.policies[name]; !ok {
		return &userError{"policy not found: " + name}
	}
	delete(ps.policies, name)
	return ps.save()
}

// save serializes policies to disk (must be called with lock held).
func (ps *PolicyStore) save() error {
	list := make([]*NamedPolicy, 0, len(ps.policies))
	for _, p := range ps.policies {
		list = append(list, p)
	}
	data, err := json.MarshalIndent(list, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(ps.filePath, data, 0600)
}

type userError struct{ msg string }

func (e *userError) Error() string { return e.msg }

func hashPassword(password string) string {
	if password == "" {
		return ""
	}
	hash := sha256.Sum256([]byte(password))
	return hex.EncodeToString(hash[:])
}

func isHashed(password string) bool {
	if len(password) != 64 {
		return false
	}
	for i := 0; i < len(password); i++ {
		c := password[i]
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F')) {
			return false
		}
	}
	return true
}
