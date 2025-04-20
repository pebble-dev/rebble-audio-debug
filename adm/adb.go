package adm

import (
	"github.com/gorilla/sessions"
	"net/http"
)

type Adm struct {
	store sessions.Store
	mux   *http.ServeMux
	auth  *Authenticator
	rv    *RecordingViewer
}

func NewAdb(store sessions.Store) *Adm {
	a := &Adm{
		store: store,
		mux:   http.NewServeMux(),
		auth:  NewAuthenticator(store),
		rv:    NewRecordingViewer(store),
	}
	a.mux.Handle("/auth/", http.StripPrefix("/auth", a.auth.mux))
	a.mux.HandleFunc("/heartbeat", a.handleHeartbeat)
	a.mux.HandleFunc("/", a.handleRoot)
	return a
}

func (a *Adm) handleHeartbeat(rw http.ResponseWriter, r *http.Request) {
	_, _ = rw.Write([]byte("adm"))
}

func (a *Adm) handleRoot(rw http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		a.rv.mux.ServeHTTP(rw, r)
		return
	}
	http.Redirect(rw, r, "/recordings", http.StatusFound)
}

func (a *Adm) ListenAndServe(addr string) error {
	return http.ListenAndServe(addr, a.mux)
}
