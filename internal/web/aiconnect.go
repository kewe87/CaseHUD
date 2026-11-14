// Copyright (C) 2026 kewe87
// SPDX-License-Identifier: GPL-3.0-only

package web

import (
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"os"
	"path/filepath"

	"github.com/kewe87/casehud/internal/access"
	"github.com/kewe87/casehud/internal/aiconnect"
	"github.com/kewe87/casehud/internal/app"
	"github.com/kewe87/casehud/internal/mcp"
)

// aiClientName is the name a connected app has in the list of programs.
func aiClientName(appName string) string { return "MCP (" + appName + ")" }

// aiAppView is an AI app in the settings.
type aiAppView struct {
	aiconnect.Status
	// Scope and System are the rights of the token set up for the app.
	Scope  access.Scope `json:"scope,omitempty"`
	System bool         `json:"system,omitempty"`
}

type aiView struct {
	Available bool        `json:"available"`
	Apps      []aiAppView `json:"apps"`
	MCP       bool        `json:"mcp"`
	API       bool        `json:"api"`
	Address   string      `json:"address"`
	// Hint names the first app found while nothing is connected (shown once
	// until dismissed).
	Hint string `json:"hint,omitempty"`
}

func (s *Server) aiView(r *http.Request) aiView {
	v := aiView{Apps: []aiAppView{}, MCP: s.Engine.MCPEnabled(), API: s.Engine.APIEnabled(), Address: "http://" + r.Host + "/api/v1/"}
	if s.AI == nil {
		return v
	}
	v.Available = true
	connected, first := false, ""
	for _, st := range s.AI.Detect() {
		a := aiAppView{Status: st}
		if st.State == aiconnect.Connected {
			connected = true
			for _, c := range s.acc().List() {
				if c.Interface == access.MCP && c.App == st.ID {
					a.Scope, a.System = c.Scope, c.System
				}
			}
		}
		if st.State == aiconnect.Found && first == "" {
			first = st.Name
		}
		v.Apps = append(v.Apps, a)
	}
	if !connected && first != "" && !s.Engine.AIHintDismissed() {
		v.Hint = first
	}
	return v
}

// aiMessages are the texts for the window, by code. They name no file or folder:
// the details go to the log.
var aiMessages = map[string]string{
	aiconnect.CodeInvalidJSON:   "The configuration file is not valid JSON (comments are not supported). It was not changed.",
	aiconnect.CodeDuplicateKeys: "The configuration file contains a key twice. It was not changed.",
	aiconnect.CodeSymlink:       "The configuration file or one of its folders is a link or junction. It was not changed.",
	aiconnect.CodeNotFound:      "The app or its configuration was not found.",
	aiconnect.CodeWriteFailed:   "The configuration could not be written.",
	aiconnect.CodeCLIFailed:     "The claude command failed.",
	aiconnect.CodeUnsupported:   "This is not possible for this app.",
	aiconnect.CodeConflict:      "Another entry named \"casehud\" exists.",
}

// aiFail answers with the status that fits the error of the connector and a
// JSON body {code, message, error}; code is stable, message has no path.
func aiFail(w http.ResponseWriter, err error) {
	var inv *aiconnect.InvalidError
	status := http.StatusBadGateway
	switch {
	case errors.Is(err, aiconnect.ErrConflict):
		status = http.StatusConflict
	case errors.Is(err, aiconnect.ErrNotFound), errors.Is(err, aiconnect.ErrUnknownApp):
		status = http.StatusNotFound
	case errors.As(err, &inv):
		status = http.StatusUnprocessableEntity
	}
	code := aiconnect.CodeOf(err)
	msg := aiMessages[code]
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(map[string]string{"code": code, "message": msg, "error": msg})
}

// revokeAI revokes the tokens of an app (by its id) except keep.
func (s *Server) revokeAI(appID, keep string) {
	for _, c := range s.acc().List() {
		if c.Interface == access.MCP && c.App == appID && c.ID != keep {
			_ = s.acc().Revoke(c.ID)
		}
	}
}

// enableInterface switches the API or MCP interface on (a program that was
// just set up would not work otherwise).
func (s *Server) enableInterface(mcpOn, apiOn bool) error {
	st := s.Engine.Settings()
	if (!mcpOn || st.MCP) && (!apiOn || st.API) {
		return nil
	}
	if err := s.Engine.PatchSettings(func(st *app.Settings) {
		st.MCP = st.MCP || mcpOn
		st.API = st.API || apiOn
	}); err != nil {
		return err
	}
	s.SyncAPI()
	return nil
}

// aiRoutes are the routes of the window for the AI apps and for tokens of
// other programs. They answer to the token of the window only: a program can
// never connect an app or create a token for itself. The app comes from a
// fixed list, never from a path in the request.
func (s *Server) aiRoutes(api *http.ServeMux) {
	api.HandleFunc("GET /api/ai", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, s.aiView(r))
	})
	api.HandleFunc("POST /api/ai/hint/dismiss", func(w http.ResponseWriter, r *http.Request) {
		if err := s.Engine.DismissAIHint(); err != nil {
			fail(w, http.StatusInternalServerError, err)
			return
		}
		writeJSON(w, s.aiView(r))
	})
	api.HandleFunc("POST /api/ai/{id}/connect", func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Scope   access.Scope `json:"scope"`
			Replace bool         `json:"replace"`
		}
		if !decodeGUI(w, r, &body) {
			return
		}
		name, ok := aiconnect.Name(r.PathValue("id"))
		if !ok || s.AI == nil {
			aiFail(w, aiconnect.ErrUnknownApp)
			return
		}
		if !body.Scope.Valid() { // "system" is never offered here
			fail(w, http.StatusBadRequest, access.ErrBadScope)
			return
		}
		id := r.PathValue("id")
		s.aiMu.Lock()
		defer s.aiMu.Unlock()
		var newID string
		pre := func() (func(), error) {
			cid, tok, err := s.acc().MintApp(id, aiClientName(name), body.Scope, false, access.MCP, true)
			if err != nil {
				return nil, err
			}
			newID = cid
			undo := func() { _ = s.acc().Revoke(cid) }
			file, _ := mcp.TokenFileFor(id)
			b, _ := json.MarshalIndent(map[string]string{"token": tok, "name": aiClientName(name)}, "", "  ")
			if err := WriteSecretFile(filepath.Join(s.DataDir, file), append(b, '\n')); err != nil {
				undo()
				return nil, err
			}
			return func() { undo(); _ = os.Remove(filepath.Join(s.DataDir, file)) }, nil
		}
		res, err := s.AI.Connect(r.Context(), id, body.Replace, pre)
		if err != nil {
			log.Printf("ai: connecting %s failed: %v", name, err)
			aiFail(w, err)
			return
		}
		s.revokeAI(id, newID)
		if err := s.enableInterface(true, false); err != nil {
			log.Printf("ai: the MCP interface could not be switched on: %v", err)
		}
		log.Printf("ai: %s connected with %s access", name, body.Scope)
		writeJSON(w, map[string]any{"result": res, "view": s.aiView(r)})
	})
	api.HandleFunc("DELETE /api/ai/{id}", func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		name, ok := aiconnect.Name(id)
		if !ok || s.AI == nil {
			aiFail(w, aiconnect.ErrUnknownApp)
			return
		}
		s.aiMu.Lock()
		defer s.aiMu.Unlock()
		res, err := s.AI.Disconnect(r.Context(), id)
		if err != nil {
			log.Printf("ai: disconnecting %s failed: %v", name, err)
			aiFail(w, err)
			return
		}
		s.revokeAI(id, "")
		if file, ok := mcp.TokenFileFor(id); ok {
			_ = os.Remove(filepath.Join(s.DataDir, file))
		}
		log.Printf("ai: %s disconnected", name)
		writeJSON(w, map[string]any{"result": res, "view": s.aiView(r)})
	})
	api.HandleFunc("POST /api/access/tokens", func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Name   string       `json:"name"`
			Scope  access.Scope `json:"scope"`
			System bool         `json:"system"`
		}
		if !decodeGUI(w, r, &body) {
			return
		}
		id, tok, err := s.acc().Mint(body.Name, body.Scope, body.System, access.API, true)
		if err != nil {
			accessFail(w, err)
			return
		}
		if err := s.enableInterface(false, true); err != nil {
			log.Printf("ai: the API could not be switched on: %v", err)
		}
		log.Printf("access: token %q created (scope %s, system: %t)", s.clientName(id), body.Scope, body.System && body.Scope == access.Control)
		addr := "http://" + r.Host + "/api/v1/"
		writeJSON(w, map[string]any{"id": id, "token": tok, "address": addr, "openapi": addr + "openapi.json", "access": s.accessView()})
	})
}
