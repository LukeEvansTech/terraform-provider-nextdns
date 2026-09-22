package nextdns

import (
	"encoding/json"
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
// performance, security gets tlds, parentalControl gets services/categories)
// and can hide keys from GET responses to simulate a feature the API stops
// returning. Every request is recorded for assertions.
type fakeAPI struct {
	t      *testing.T
	mu     sync.Mutex
	store  map[string]map[string]any // "<profile>/<path>" -> object
	lists  map[string][]any          // "<profile>/<path>" -> list
	hidden map[string]bool           // "<profile>/<path>/<key>" -> hidden from GET
	reqs   []recordedRequest
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

	// /profiles/<id>/<path...>
	parts := strings.SplitN(strings.TrimPrefix(r.URL.Path, "/profiles/"), "/", 2)
	if len(parts) != 2 {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"errors":[{"code":"notFound"}]}`))
		return
	}
	profile, path := parts[0], parts[1]
	key := profile + "/" + path

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
