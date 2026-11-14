package web

import "github.com/kewe87/casehud/internal/fileacl"

func restrictFile(path string) error { return fileacl.Restrict(path) }
