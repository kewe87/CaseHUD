package app

import (
	"context"
	"errors"
	"log"
	"os"
	"sync"

	"github.com/kewe87/casehud/internal/community"
	leds "github.com/kewe87/casehud/internal/rgb"
	"github.com/kewe87/casehud/internal/sketch"
)

// builtinComponentNames are the components the LED module ships with; the
// library may add parts but never replace these.
var builtinComponentNames = func() map[string]bool {
	m := map[string]bool{}
	for _, c := range leds.Components() {
		m[c.Name] = true
	}
	return m
}()

// libraryState holds the library client of the engine.
type libraryState struct {
	once sync.Once
	lib  *community.Library
	err  error

	catMu sync.Mutex
	cat   *sketch.Catalog // parts and cases for the case sketch (library_catalog.go)
}

// Library returns the client for the component, case and lighting preset
// library. Its cache lives in the data directory; the network is only used by
// RefreshLibrary.
func (e *Engine) Library() (*community.Library, error) {
	s := &e.library
	s.once.Do(func() {
		s.lib, s.err = community.NewLibrary(e.CommunityURL(), e.Store.path("library"))
		if s.lib != nil {
			s.lib.Reserved = func(id string) bool { return builtinComponentNames[id] }
		}
	})
	return s.lib, s.err
}

// loadLibraryComponents registers the cached library parts with the LED
// module. It reads local files only.
func (e *Engine) loadLibraryComponents() {
	e.invalidateSketchCatalog()
	lib, err := e.Library()
	if err != nil {
		return
	}
	// The cached files are checked first (size, coordinates, names); only
	// what passes is handed to the LED module, through a staging folder.
	stage, skipped, err := stageLibrary(lib.Dir)
	if err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			log.Printf("library: components not loaded: %v", err)
		}
		return
	}
	defer os.RemoveAll(stage)
	if len(skipped) > 0 {
		log.Printf("library: %d components refused: %v", len(skipped), errors.Join(skipped...))
	}
	n, err := leds.LoadLibrary(stage)
	switch {
	case err == nil, errors.Is(err, os.ErrNotExist) && n == 0:
	case n == 0:
		log.Printf("library: components not loaded: %v", err)
	default:
		log.Printf("library: %d components loaded, some skipped: %v", n, err)
	}
}

// RefreshLibrary downloads the library (the user asked for it), keeps the
// copy in the data directory and hands the components to the LED module. A
// failed download leaves the old copy in place.
func (e *Engine) RefreshLibrary(ctx context.Context) (community.Summary, error) {
	lib, err := e.Library()
	if err != nil {
		return community.Summary{}, err
	}
	defer e.invalidatePresets() // presets may have changed, even after a partial refresh
	s, err := lib.Refresh(ctx)
	if err != nil {
		return lib.Status(), err
	}
	e.loadLibraryComponents()
	return s, nil
}

// LibraryStatus describes the cached library (no network).
func (e *Engine) LibraryStatus() community.Summary {
	lib, err := e.Library()
	if err != nil {
		return community.Summary{}
	}
	return lib.Status()
}
