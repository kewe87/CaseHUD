// Copyright (C) 2026 kewe87
// SPDX-License-Identifier: GPL-3.0-only

package aiconnect

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/kewe87/casehud/internal/fileacl"
)

// checkPlain refuses links and junctions: what we write must be the file itself.
func checkPlain(p string) error {
	fi, err := os.Lstat(p)
	if err != nil {
		return err
	}
	if fi.Mode()&(os.ModeSymlink|os.ModeIrregular) != 0 {
		return &InvalidError{Path: p, Code: CodeSymlink, Reason: "it is a link, not a regular file or folder"}
	}
	return nil
}

// checkChain runs checkPlain on every folder from base (trusted, not checked)
// down to p, and on p itself when it exists: a link or junction anywhere on
// the way could send a write elsewhere. A part that does not exist is fine.
func checkChain(base, p string) error {
	rel, err := filepath.Rel(base, p)
	if err != nil || rel == "." || strings.HasPrefix(rel, "..") {
		return checkPlain(filepath.Dir(p))
	}
	cur := base
	for _, part := range strings.Split(rel, string(filepath.Separator)) {
		cur = filepath.Join(cur, part)
		if err := checkPlain(cur); err != nil {
			if errors.Is(err, os.ErrNotExist) {
				return nil
			}
			return err
		}
	}
	return nil
}

// readConfig reads a config file after the checks of checkChain, up to
// MaxConfigSize. The opened file must be the one that was checked.
func readConfig(base, p string) ([]byte, error) {
	if err := checkChain(base, p); err != nil {
		return nil, err
	}
	fi, err := os.Lstat(p)
	if err != nil {
		return nil, err
	}
	if !fi.Mode().IsRegular() {
		return nil, &InvalidError{Path: p, Code: CodeUnsupported, Reason: "it is not a regular file"}
	}
	if fi.Size() > MaxConfigSize {
		return nil, &InvalidError{Path: p, Code: CodeUnsupported, Reason: "it is larger than 1 MB"}
	}
	f, err := os.Open(p)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	if of, err := f.Stat(); err != nil || !os.SameFile(fi, of) {
		return nil, &InvalidError{Path: p, Code: CodeSymlink, Reason: "the file changed while it was opened"}
	}
	b, err := io.ReadAll(io.LimitReader(f, MaxConfigSize+1))
	if err != nil {
		return nil, err
	}
	if len(b) > MaxConfigSize {
		return nil, &InvalidError{Path: p, Code: CodeUnsupported, Reason: "it is larger than 1 MB"}
	}
	return bytes.TrimPrefix(b, []byte("\xef\xbb\xbf")), nil
}

// maxBackups is how many backups of one config file are kept. A backup holds
// everything the file held, secrets of other servers included.
const maxBackups = 5

// pruneBackups removes the oldest backups of p beyond maxBackups.
func pruneBackups(p string) {
	dir, prefix := filepath.Dir(p), filepath.Base(p)+".casehud-backup-"
	ents, err := os.ReadDir(dir)
	if err != nil {
		return
	}
	type bk struct {
		name string
		mod  time.Time
	}
	var all []bk
	for _, e := range ents {
		if !e.IsDir() && strings.HasPrefix(e.Name(), prefix) {
			if fi, err := e.Info(); err == nil && fi.Mode().IsRegular() {
				all = append(all, bk{e.Name(), fi.ModTime()})
			}
		}
	}
	if len(all) <= maxBackups {
		return
	}
	sort.Slice(all, func(i, j int) bool {
		if !all[i].mod.Equal(all[j].mod) {
			return all[i].mod.Before(all[j].mod)
		}
		return all[i].name < all[j].name
	})
	for _, b := range all[:len(all)-maxBackups] {
		os.Remove(filepath.Join(dir, b.name))
	}
}

// writeConfig backs up the existing file, then replaces it atomically.
func (e Env) writeConfig(base, p string, data []byte, exists bool) (string, error) {
	dir := filepath.Dir(p)
	if err := checkChain(base, p); err != nil {
		return "", err
	}
	if len(data) > MaxConfigSize {
		return "", &InvalidError{Path: p, Code: CodeUnsupported, Reason: "the result would be larger than 1 MB"}
	}
	backup := ""
	if exists {
		old, err := os.ReadFile(p)
		if err != nil {
			return "", err
		}
		stamp := e.now().Format("20060102-150405")
		backup = p + ".casehud-backup-" + stamp
		for i := 2; ; i++ { // never overwrite an earlier backup
			if _, err := os.Lstat(backup); errors.Is(err, os.ErrNotExist) {
				break
			}
			backup = fmt.Sprintf("%s.casehud-backup-%s-%d", p, stamp, i)
		}
		if err := os.WriteFile(backup, old, 0o600); err != nil {
			return "", fmt.Errorf("the backup could not be written: %w", err)
		}
		// The backup is as private as the file; if its rights cannot be
		// copied, it is made private to the user.
		if err := fileacl.CopyDACL(p, backup); err != nil {
			if rerr := fileacl.Restrict(backup); rerr != nil {
				os.Remove(backup)
				return "", fmt.Errorf("the backup could not be protected: %w", rerr)
			}
		}
	} else if _, err := os.Lstat(p); err == nil {
		return "", &InvalidError{Path: p, Code: CodeWriteFailed, Reason: "the file appeared meanwhile"}
	}
	tmp, err := os.CreateTemp(dir, ".casehud-*.tmp")
	if err != nil {
		return "", err
	}
	name := tmp.Name()
	_, werr := tmp.Write(data)
	if serr := tmp.Sync(); werr == nil {
		werr = serr
	}
	if cerr := tmp.Close(); werr == nil {
		werr = cerr
	}
	if werr != nil {
		os.Remove(name)
		return "", werr
	}
	if exists { // the new file keeps the rights of the old one
		if err := fileacl.CopyDACL(p, name); err != nil {
			os.Remove(name)
			return "", fmt.Errorf("the rights of the file could not be kept: %w", err)
		}
	}
	// Check the way again just before the swap: a link set meanwhile is caught.
	if err := checkChain(base, p); err != nil {
		os.Remove(name)
		return "", err
	}
	if err := os.Rename(name, p); err != nil {
		os.Remove(name)
		return "", err
	}
	pruneBackups(p)
	return backup, nil
}

// object is a JSON object that keeps the order of its keys and the raw
// text of its values, so only the changed key differs afterwards.
type object []kv

type kv struct {
	key string
	val json.RawMessage
}

// errDuplicateKey: an object holds one key twice. Readers disagree about which
// value counts, so such a file is left alone.
var errDuplicateKey = errors.New("duplicate key")

func parseObject(b []byte) (object, error) {
	dec := json.NewDecoder(bytes.NewReader(b))
	t, err := dec.Token()
	if err != nil {
		return nil, err
	}
	if d, ok := t.(json.Delim); !ok || d != '{' {
		return nil, errors.New("not an object")
	}
	var o object
	for dec.More() {
		kt, err := dec.Token()
		if err != nil {
			return nil, err
		}
		key, _ := kt.(string)
		if _, dup := o.get(key); dup {
			return nil, errDuplicateKey
		}
		var raw json.RawMessage
		if err := dec.Decode(&raw); err != nil {
			return nil, err
		}
		o = append(o, kv{key, raw})
	}
	if _, err := dec.Token(); err != nil {
		return nil, err
	}
	if _, err := dec.Token(); err != io.EOF {
		return nil, errors.New("data after the object")
	}
	return o, nil
}

func (o object) get(k string) (json.RawMessage, bool) {
	for _, e := range o {
		if e.key == k {
			return e.val, true
		}
	}
	return nil, false
}

func (o *object) set(k string, v json.RawMessage) {
	for i := range *o {
		if (*o)[i].key == k {
			(*o)[i].val = v
			return
		}
	}
	*o = append(*o, kv{k, v})
}

func (o *object) del(k string) {
	out := (*o)[:0]
	for _, e := range *o {
		if e.key != k {
			out = append(out, e)
		}
	}
	*o = out
}

func (o object) marshal() json.RawMessage {
	var b bytes.Buffer
	b.WriteByte('{')
	for i, e := range o {
		if i > 0 {
			b.WriteByte(',')
		}
		k, _ := json.Marshal(e.key)
		b.Write(k)
		b.WriteByte(':')
		b.Write(e.val)
	}
	b.WriteByte('}')
	return b.Bytes()
}

// indent writes JSON with two spaces and a final newline.
func indent(raw []byte) ([]byte, error) {
	var b bytes.Buffer
	if err := json.Indent(&b, raw, "", "  "); err != nil {
		return nil, err
	}
	b.WriteByte('\n')
	return b.Bytes(), nil
}
