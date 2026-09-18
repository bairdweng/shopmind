package vault

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"golang.org/x/crypto/argon2"

	"shopmind/internal/config"
)

const (
	fileVersion = 1
	kdfName     = "argon2id"
	kdfTime     = 2
	kdfMemory   = 32 * 1024
	kdfThreads  = 1
	keyLen      = 32
	saltLen     = 16
)

var (
	ErrNotInitialized = errors.New("保险库未初始化")
	ErrAlreadyInit    = errors.New("保险库已存在")
	ErrLocked         = errors.New("未解锁")
	ErrBadPassword    = errors.New("口令错误")
	ErrEmptyPassword  = errors.New("口令不能为空")
	ErrNoLocalKey     = errors.New("本机密钥不存在")
)

type Payload struct {
	CursorAPIKey   string `json:"cursor_api_key,omitempty"`
	CursorAgentBin string `json:"cursor_agent_bin,omitempty"`
}

type Public struct {
	CursorAPIKeySet  bool   `json:"cursor_api_key_set"`
	CursorAPIKeyHint string `json:"cursor_api_key_hint"`
	CursorAgentBin   string `json:"cursor_agent_bin"`
}

type Patch struct {
	CursorAPIKey   string
	CursorAgentBin string
	UpdateAPIKey   bool
	ClearAPIKey    bool
}

type fileFormat struct {
	V       int    `json:"v"`
	KDF     string `json:"kdf"`
	Time    uint32 `json:"time"`
	Memory  uint32 `json:"memory"`
	Threads uint8  `json:"threads"`
	Salt    string `json:"salt"`
	Nonce   string `json:"nonce"`
	CT      string `json:"ct"`
}

type Store struct {
	mu       sync.RWMutex
	key      []byte
	params   fileFormat
	payload  Payload
	unlocked bool
}

func New() *Store {
	return &Store{}
}

func (s *Store) Initialized() bool {
	_, err := os.Stat(config.VaultPath())
	return err == nil
}

func (s *Store) Unlocked() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.unlocked
}

func (s *Store) Payload() (Payload, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if !s.unlocked {
		return Payload{}, ErrLocked
	}
	return s.payload, nil
}

func (s *Store) Public() (Public, error) {
	p, err := s.Payload()
	if err != nil {
		return Public{}, err
	}
	return Public{
		CursorAPIKeySet:  strings.TrimSpace(p.CursorAPIKey) != "",
		CursorAPIKeyHint: maskKey(p.CursorAPIKey),
		CursorAgentBin:   p.CursorAgentBin,
	}, nil
}

func (s *Store) Init(password string) error {
	password = strings.TrimSpace(password)
	if password == "" {
		return ErrEmptyPassword
	}
	if s.Initialized() {
		return ErrAlreadyInit
	}
	salt := make([]byte, saltLen)
	if _, err := rand.Read(salt); err != nil {
		return err
	}
	key := argon2.IDKey([]byte(password), salt, kdfTime, kdfMemory, kdfThreads, keyLen)
	s.mu.Lock()
	defer s.mu.Unlock()
	s.key = key
	s.params = fileFormat{
		V:       fileVersion,
		KDF:     kdfName,
		Time:    kdfTime,
		Memory:  kdfMemory,
		Threads: kdfThreads,
		Salt:    base64.StdEncoding.EncodeToString(salt),
	}
	s.payload = Payload{}
	s.unlocked = true
	if err := s.persistLocked(); err != nil {
		return err
	}
	return s.writeLocalKeyLocked()
}

func (s *Store) Unlock(password string) error {
	password = strings.TrimSpace(password)
	if password == "" {
		return ErrEmptyPassword
	}
	raw, err := os.ReadFile(config.VaultPath())
	if err != nil {
		if os.IsNotExist(err) {
			return ErrNotInitialized
		}
		return err
	}
	var f fileFormat
	if err := json.Unmarshal(raw, &f); err != nil {
		return fmt.Errorf("保险库损坏: %w", err)
	}
	if f.V != fileVersion || f.KDF != kdfName {
		return fmt.Errorf("不支持的保险库格式")
	}
	salt, err := base64.StdEncoding.DecodeString(f.Salt)
	if err != nil {
		return fmt.Errorf("保险库损坏: %w", err)
	}
	nonce, err := base64.StdEncoding.DecodeString(f.Nonce)
	if err != nil {
		return fmt.Errorf("保险库损坏: %w", err)
	}
	ct, err := base64.StdEncoding.DecodeString(f.CT)
	if err != nil {
		return fmt.Errorf("保险库损坏: %w", err)
	}
	key := argon2.IDKey([]byte(password), salt, f.Time, f.Memory, f.Threads, keyLen)
	plain, err := decrypt(key, nonce, ct)
	if err != nil {
		return ErrBadPassword
	}
	var payload Payload
	if err := json.Unmarshal(plain, &payload); err != nil {
		return fmt.Errorf("保险库损坏: %w", err)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.key = key
	s.params = f
	s.payload = payload
	s.unlocked = true
	return s.writeLocalKeyLocked()
}

// UnlockFromLocalKey 用 .local/keys/master.key 解锁，密钥文件不存在时返回 ErrNoLocalKey。
func (s *Store) UnlockFromLocalKey() error {
	key, err := os.ReadFile(config.MasterKeyPath())
	if err != nil {
		if os.IsNotExist(err) {
			return ErrNoLocalKey
		}
		return err
	}
	if len(key) != keyLen {
		return fmt.Errorf("本机密钥损坏")
	}
	raw, err := os.ReadFile(config.VaultPath())
	if err != nil {
		if os.IsNotExist(err) {
			return ErrNotInitialized
		}
		return err
	}
	var f fileFormat
	if err := json.Unmarshal(raw, &f); err != nil {
		return fmt.Errorf("保险库损坏: %w", err)
	}
	nonce, err := base64.StdEncoding.DecodeString(f.Nonce)
	if err != nil {
		return fmt.Errorf("保险库损坏: %w", err)
	}
	ct, err := base64.StdEncoding.DecodeString(f.CT)
	if err != nil {
		return fmt.Errorf("保险库损坏: %w", err)
	}
	plain, err := decrypt(key, nonce, ct)
	if err != nil {
		return ErrBadPassword
	}
	var payload Payload
	if err := json.Unmarshal(plain, &payload); err != nil {
		return fmt.Errorf("保险库损坏: %w", err)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.key = append([]byte(nil), key...)
	s.params = f
	s.payload = payload
	s.unlocked = true
	return nil
}

func (s *Store) Apply(patch Patch) (Public, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.unlocked {
		return Public{}, ErrLocked
	}
	s.payload.CursorAgentBin = strings.TrimSpace(patch.CursorAgentBin)
	if patch.ClearAPIKey {
		s.payload.CursorAPIKey = ""
	} else if patch.UpdateAPIKey {
		s.payload.CursorAPIKey = strings.TrimSpace(patch.CursorAPIKey)
	}
	if err := s.persistLocked(); err != nil {
		return Public{}, err
	}
	return Public{
		CursorAPIKeySet:  strings.TrimSpace(s.payload.CursorAPIKey) != "",
		CursorAPIKeyHint: maskKey(s.payload.CursorAPIKey),
		CursorAgentBin:   s.payload.CursorAgentBin,
	}, nil
}

func (s *Store) persistLocked() error {
	if err := os.MkdirAll(filepath.Dir(config.VaultPath()), 0o755); err != nil {
		return err
	}
	plain, err := json.Marshal(s.payload)
	if err != nil {
		return err
	}
	nonce, ct, err := encrypt(s.key, plain)
	if err != nil {
		return err
	}
	f := s.params
	f.V = fileVersion
	f.KDF = kdfName
	f.Nonce = base64.StdEncoding.EncodeToString(nonce)
	f.CT = base64.StdEncoding.EncodeToString(ct)
	data, err := json.MarshalIndent(f, "", "  ")
	if err != nil {
		return err
	}
	tmp := config.VaultPath() + ".tmp"
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, config.VaultPath())
}

func (s *Store) writeLocalKeyLocked() error {
	if err := os.MkdirAll(config.LocalKeysDir(), 0o700); err != nil {
		return err
	}
	tmp := config.MasterKeyPath() + ".tmp"
	if err := os.WriteFile(tmp, s.key, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, config.MasterKeyPath())
}

func encrypt(key, plain []byte) (nonce, ct []byte, err error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, nil, err
	}
	nonce = make([]byte, gcm.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return nil, nil, err
	}
	ct = gcm.Seal(nil, nonce, plain, nil)
	return nonce, ct, nil
}

func decrypt(key, nonce, ct []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	return gcm.Open(nil, nonce, ct, nil)
}

func maskKey(key string) string {
	key = strings.TrimSpace(key)
	if key == "" {
		return ""
	}
	if len(key) <= 4 {
		return "****"
	}
	return "********" + key[len(key)-4:]
}
