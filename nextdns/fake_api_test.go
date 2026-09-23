package nextdns

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
)

// fakeAPI is an in-memory stand-in for api.nextdns.io that speaks the subset
// of the API the provider uses. It stores raw JSON per (profile, sub-path),
// applies PATCH as a shallow merge and PUT as a replace, composes parent
// objects from their sub-paths on GET (settings gets logs/blockPage/
// performance, security gets tlds, parentalControl gets services/categories,
// privacy gets blocklists/natives) and can hide keys from GET responses to
// simulate a feature the API stops returning. Profiles are created with
// POST /profiles and rewrites are POSTed and DELETEd by id, as the real API
// does. Every request is recorded for assertions.
type fakeAPI struct {
	t      *testing.T
	mu     sync.Mutex
	store  map[string]map[string]any // "<profile>/<path>" -> object
	lists  map[string][]any          // "<profile>/<path>" -> list
	hidden map[string]bool           // "<profile>/<path>/<key>" -> hidden from GET
	reqs   []recordedRequest
	nextID int
	Server *httptest.Server
}

type recordedRequest struct {
	Method string
	Path   string
	Body   map[string]any
}

// composed maps a parent path to the sub-paths whose bodies are embedded in
// its GET response, under the key the API uses.
var composed = map[string]map[string]string{
	"settings":        {"logs": "settings/logs", "blockPage": "settings/blockPage", "performance": "settings/performance"},
	"security":        {"tlds": "security/tlds"},
	"parentalControl": {"services": "parentalControl/services", "categories": "parentalControl/categories"},
	"privacy":         {"blocklists": "privacy/blocklists", "natives": "privacy/natives"},
}

func newFakeAPI(t *testing.T) *fakeAPI {
	t.Helper()
	f := &fakeAPI{
		t:      t,
		store:  map[string]map[string]any{},
		lists:  map[string][]any{},
		hidden: map[string]bool{},
	}
	f.Server = httptest.NewServer(http.HandlerFunc(f.handle))
	t.Cleanup(f.Server.Close)
	return f
}

// seed sets the stored object for a profile sub-path from a JSON literal.
func (f *fakeAPI) seed(profile, path, body string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	var v any
	if err := json.Unmarshal([]byte(body), &v); err != nil {
		f.t.Fatalf("seed %s: %v", path, err)
	}
	switch tv := v.(type) {
	case map[string]any:
		if _, ok := f.store[profile+"/"+path]; !ok {
			f.store[profile+"/"+path] = map[string]any{}
		}
		for k, val := range tv { // merge, so a second seed overrides keys rather than the object
			f.store[profile+"/"+path][k] = val
		}
	case []any:
		f.lists[profile+"/"+path] = tv
	default:
		f.t.Fatalf("seed %s: unsupported JSON %T", path, v)
	}
}

// hide stops a key from being returned by GET on a profile sub-path.
func (f *fakeAPI) hide(profile, path, key string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.hidden[profile+"/"+path+"/"+key] = true
}

// get returns a stored value (nil, false when absent).
func (f *fakeAPI) get(profile, path, key string) (any, bool) {
	f.mu.Lock()
	defer f.mu.Unlock()
	obj, ok := f.store[profile+"/"+path]
	if !ok {
		return nil, false
	}
	v, ok := obj[key]
	return v, ok
}

// list returns a copy of a stored list.
func (f *fakeAPI) list(profile, path string) []any {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]any(nil), f.lists[profile+"/"+path]...)
}

// requests returns the recorded requests matching method and path suffix.
func (f *fakeAPI) requests(method, pathSuffix string) []recordedRequest {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := []recordedRequest{}
	for _, r := range f.reqs {
		if r.Method == method && strings.HasSuffix(r.Path, pathSuffix) {
			out = append(out, r)
		}
	}
	return out
}

func (f *fakeAPI) handle(w http.ResponseWriter, r *http.Request) {
	f.mu.Lock()
	defer f.mu.Unlock()

	if r.Header.Get("X-Api-Key") == "" {
		w.WriteHeader(http.StatusForbidden)
		_, _ = w.Write([]byte(`{"errors":[{"code":"unauthorized"}]}`))
		return
	}

	var body map[string]any
	var list []any
	if r.Method != http.MethodGet {
		raw, _ := io.ReadAll(r.Body)
		if len(raw) > 0 {
			if raw[0] == '[' {
				_ = json.Unmarshal(raw, &list)
			} else {
				_ = json.Unmarshal(raw, &body)
			}
		}
	}
	f.reqs = append(f.reqs, recordedRequest{Method: r.Method, Path: r.URL.Path, Body: body})

	if r.URL.Path == "/profiles" && r.Method == http.MethodPost {
		f.nextID++
		id := fmt.Sprintf("fake%02d", f.nextID)
		f.store[id+"/"] = map[string]any{"name": body["name"]}
		writeData(w, map[string]any{"id": id})
		return
	}

	// /profiles/<id>[/<path...>]
	parts := strings.SplitN(strings.TrimPrefix(r.URL.Path, "/profiles/"), "/", 2)
	if len(parts) == 1 {
		f.handleProfile(w, r.Method, parts[0], body)
		return
	}
	profile, path := parts[0], parts[1]
	key := profile + "/" + path

	if f.handleRewrites(w, r.Method, profile, path, body) {
		return
	}

	switch r.Method {
	case http.MethodGet:
		if l, ok := f.lists[key]; ok {
			out, _ := json.Marshal(map[string]any{"data": l})
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write(out)
			return
		}
		obj := map[string]any{}
		for k, v := range f.store[key] {
			if !f.hidden[key+"/"+k] {
				obj[k] = v
			}
		}
		for embedKey, sub := range composed[path] {
			if l, ok := f.lists[profile+"/"+sub]; ok {
				obj[embedKey] = l
			} else if o, ok := f.store[profile+"/"+sub]; ok {
				obj[embedKey] = o
			}
		}
		out, _ := json.Marshal(map[string]any{"data": obj})
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(out)
	case http.MethodPatch:
		if _, ok := f.store[key]; !ok {
			f.store[key] = map[string]any{}
		}
		for k, v := range body {
			if _, embedded := composed[path][k]; embedded {
				continue // sub-objects live at their own path
			}
			f.store[key][k] = v
		}
		w.WriteHeader(http.StatusNoContent)
	case http.MethodPut:
		f.lists[key] = list
		if list == nil {
			f.lists[key] = []any{}
		}
		w.WriteHeader(http.StatusNoContent)
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

// handleProfile serves GET/PATCH/DELETE on /profiles/<id> for profiles
// created through POST /profiles. The caller holds f.mu.
func (f *fakeAPI) handleProfile(w http.ResponseWriter, method, id string, body map[string]any) {
	obj, ok := f.store[id+"/"]
	if !ok {
		notFound(w)
		return
	}
	switch method {
	case http.MethodGet:
		writeData(w, obj)
	case http.MethodPatch:
		for k, v := range body {
			obj[k] = v
		}
		w.WriteHeader(http.StatusNoContent)
	case http.MethodDelete:
		delete(f.store, id+"/")
		w.WriteHeader(http.StatusNoContent)
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

// handleRewrites serves POST .../rewrites and DELETE .../rewrites/<id>,
// which the real API addresses by rewrite id rather than by replacing the
// list. GET falls through to the generic list handling. The caller holds
// f.mu. It reports whether it wrote a response.
func (f *fakeAPI) handleRewrites(w http.ResponseWriter, method, profile, path string, body map[string]any) bool {
	key := profile + "/rewrites"
	switch {
	case path == "rewrites" && method == http.MethodPost:
		f.nextID++
		body["id"] = fmt.Sprintf("rw%02d", f.nextID)
		body["type"] = "A"
		f.lists[key] = append(f.lists[key], body)
		writeData(w, body)
		return true
	case strings.HasPrefix(path, "rewrites/") && method == http.MethodDelete:
		id := strings.TrimPrefix(path, "rewrites/")
		kept := []any{}
		for _, e := range f.lists[key] {
			if m, ok := e.(map[string]any); ok && m["id"] == id {
				continue
			}
			kept = append(kept, e)
		}
		f.lists[key] = kept
		w.WriteHeader(http.StatusNoContent)
		return true
	}
	return false
}

func writeData(w http.ResponseWriter, v any) {
	out, _ := json.Marshal(map[string]any{"data": v})
	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write(out)
}

func notFound(w http.ResponseWriter) {
	w.WriteHeader(http.StatusNotFound)
	_, _ = w.Write([]byte(`{"errors":[{"code":"notFound"}]}`))
}
