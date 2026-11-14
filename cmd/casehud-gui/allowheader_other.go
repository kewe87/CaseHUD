// Copyright (C) 2026 kewe87
// SPDX-License-Identifier: GPL-3.0-only

//go:build !windows

package main

import (
	"errors"
	"io/fs"
)

func isReparse(fs.FileInfo) bool { return false }

func sensorsDir() (string, error) { return "", errors.New("only needed on Windows") }

func makeAdminOwner(string) error { return nil }

func isSharingError(error) bool { return false }
