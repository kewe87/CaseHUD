package web

import (
	"crypto/subtle"
	"encoding/json"
	"log"
	"os"
	"path/filepath"
)

// APIFileName is the discovery file in the data folder.
const APIFileName = "api.json"

// APIFile tells other programs how to reach the running app's v1 API.
type APIFile struct {
	APIVersion int    `json:"apiVersion"`
	Version    string `json:"version"`
	Port       int    `json:"port"`
	Token      string `json:"token"`
	PID        int    `json:"pid"`
	Exe        string `json:"exe"`
}

// WriteAPIFile writes the discovery file atomically (temporary file in the
// same folder, then rename). The file is private to the user: 0600, and on
// Windows an explicit DACL for the user, SYSTEM and Administrators.
func WriteAPIFile(dir string, f APIFile) error {
	b, err := json.MarshalIndent(f, "", "  ")
	if err != nil {
		return err
	}
	tmp, err := os.CreateTemp(dir, APIFileName+".*.tmp") // created 0600
	if err != nil {
		return err
	}
	name := tmp.Name()
	// Restrict the (still empty) file before the token is written into it.
	if err := restrictFile(name); err != nil {
		tmp.Close()
		os.Remove(name)
		return err
	}
	if _, err := tmp.Write(append(b, '\n')); err != nil {
		tmp.Close()
		os.Remove(name)
		return err
	}
	if err := tmp.Close(); err != nil {
		os.Remove(name)
		return err
	}
	if err := os.Chmod(name, 0o600); err != nil {
		os.Remove(name)
		return err
	}
	if err := restrictFile(name); err != nil { // Windows: explicit DACL
		os.Remove(name)
		return err
	}
	if err := os.Rename(name, filepath.Join(dir, APIFileName)); err != nil {
		os.Remove(name)
		return err
	}
	return nil
}

// ReadAPIFile reads the discovery file from the data folder.
func ReadAPIFile(dir string) (APIFile, error) {
	var f APIFile
	b, err := os.ReadFile(filepath.Join(dir, APIFileName))
	if err != nil {
		return f, err
	}
	err = json.Unmarshal(b, &f)
	return f, err
}

// CleanAPIFiles removes temporary files a crashed start left behind.
func CleanAPIFiles(dir string) {
	old, _ := filepath.Glob(filepath.Join(dir, APIFileName+".*.tmp"))
	for _, f := range old {
		os.Remove(f)
	}
}

// RemoveAPIFile deletes the discovery file, but only if it still names this
// process, so a newer instance's file is never removed.
func RemoveAPIFile(dir string, pid int) {
	p := filepath.Join(dir, APIFileName)
	b, err := os.ReadFile(p)
	if err != nil {
		return
	}
	var f APIFile
	if json.Unmarshal(b, &f) == nil && f.PID != pid {
		return
	}
	os.Remove(p)
}

// APIInfo is what the discovery file should say about this server. Call it
// after Listen.
func (s *Server) APIInfo(pid int, exe string) APIFile {
	return APIFile{APIVersion: 1, Version: s.Version, Port: s.port(), Token: s.clientTok(), PID: pid, Exe: exe}
}

func (s *Server) clientTok() string {
	s.apiMu.RLock()
	defer s.apiMu.RUnlock()
	return s.clientToken
}

// clientOK compares a presented token with the client token; false while the
// API is off (the token is then empty).
func (s *Server) clientOK(tok string) bool {
	_, ok := s.clientAuth(tok)
	return ok
}

// clientAuth is clientOK that also returns a channel which is closed when
// this token is revoked (API switched off), so long requests can end.
func (s *Server) clientAuth(tok string) (<-chan struct{}, bool) {
	s.apiMu.RLock()
	defer s.apiMu.RUnlock()
	ok := s.clientToken != "" && subtle.ConstantTimeCompare([]byte(tok), []byte(s.clientToken)) == 1
	return s.clientGen, ok
}

// SyncAPI makes the running server match the "API" setting: switched on, it
// makes sure a client token exists and writes api.json; switched off, it
// forgets the token (so it stops working at once) and removes api.json.
// Call it after Listen and after every settings change.
func (s *Server) SyncAPI() {
	s.syncMu.Lock()
	defer s.syncMu.Unlock()
	on := s.Engine.APIEnabled()
	s.apiMu.Lock()
	if !on {
		s.clientToken = ""
		if s.clientGen != nil {
			close(s.clientGen) // ends open event streams of the client
			s.clientGen = nil
		}
	} else if s.clientToken == "" {
		tok, err := randomToken()
		if err != nil {
			s.apiMu.Unlock()
			log.Println("api token:", err)
			return
		}
		s.clientToken = tok
	}
	if on && s.clientGen == nil {
		s.clientGen = make(chan struct{})
	}
	s.apiMu.Unlock()
	if s.APIFileDir == "" {
		return
	}
	if !on {
		RemoveAPIFile(s.APIFileDir, os.Getpid())
		return
	}
	exe, err := os.Executable()
	if err != nil {
		log.Println("api.json:", err)
		return
	}
	CleanAPIFiles(s.APIFileDir)
	if err := WriteAPIFile(s.APIFileDir, s.APIInfo(os.Getpid(), exe)); err != nil {
		log.Println("api.json:", err)
	}
}

// RestrictFile limits path to the current user, SYSTEM and Administrators
// (a protected DACL on Windows; 0600 elsewhere). Call it before secrets are
// written into the file, e.g. for instance.url or config.json.
func RestrictFile(path string) error {
	if err := os.Chmod(path, 0o600); err != nil {
		return err
	}
	return restrictFile(path)
}
