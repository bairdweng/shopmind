package config

import (
	"os"
	"path/filepath"
)

const DefaultPort = "8788"

func Root() string {
	if dir := os.Getenv("SHOPMIND_ROOT"); dir != "" {
		return dir
	}
	cwd, err := os.Getwd()
	if err != nil {
		return "."
	}
	return cwd
}

func DataDir() string {
	return filepath.Join(Root(), "data")
}

func VaultPath() string {
	return filepath.Join(DataDir(), "vault.enc")
}

func RunDir() string {
	return filepath.Join(Root(), ".run")
}

func LocalKeysDir() string {
	return filepath.Join(Root(), ".local", "keys")
}

func MasterKeyPath() string {
	return filepath.Join(LocalKeysDir(), "master.key")
}
