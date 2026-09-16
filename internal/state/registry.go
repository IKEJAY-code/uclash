package state

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"

	"gopkg.in/yaml.v3"
)

// Profile is one subscription or imported local YAML.
type Profile struct {
	Name string `yaml:"name"`
	// URL is the subscription URL actually fetched (may carry clash flags).
	URL       string `yaml:"url,omitempty"`
	Origin    string `yaml:"origin,omitempty"` // original user-supplied URL or file path
	File      string `yaml:"file"`             // basename inside profiles dir
	Source    string `yaml:"source"`           // sub | file
	Converted bool   `yaml:"converted,omitempty"`
	UpdatedAt string `yaml:"updated-at,omitempty"`
}

type Registry struct {
	Active   string    `yaml:"active"`
	Profiles []Profile `yaml:"profiles"`
}

func LoadRegistry(path string) (*Registry, error) {
	r := &Registry{}
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return r, nil
		}
		return nil, err
	}
	if err := yaml.Unmarshal(data, r); err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}
	return r, nil
}

func (r *Registry) Save(path string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	data, err := yaml.Marshal(r)
	if err != nil {
		return err
	}
	return writeAtomic(path, data, 0o600)
}

func (r *Registry) Find(name string) *Profile {
	for i := range r.Profiles {
		if r.Profiles[i].Name == name {
			return &r.Profiles[i]
		}
	}
	return nil
}

func (r *Registry) Remove(name string) bool {
	for i := range r.Profiles {
		if r.Profiles[i].Name == name {
			r.Profiles = append(r.Profiles[:i], r.Profiles[i+1:]...)
			if r.Active == name {
				r.Active = ""
			}
			return true
		}
	}
	return false
}

func (r *Registry) Sort() {
	sort.Slice(r.Profiles, func(i, j int) bool { return r.Profiles[i].Name < r.Profiles[j].Name })
}

func Now() string { return time.Now().Format(time.RFC3339) }

func Stale(updatedAt string, hours int) bool {
	if updatedAt == "" {
		return true
	}
	t, err := time.Parse(time.RFC3339, updatedAt)
	if err != nil {
		return true
	}
	return time.Since(t) > time.Duration(hours)*time.Hour
}
