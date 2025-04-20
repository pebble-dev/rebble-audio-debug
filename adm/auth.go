package adm

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"github.com/gorilla/securecookie"
	"github.com/gorilla/sessions"
	"golang.org/x/oauth2"
	"io"
	"log"
	"net/http"
	"os"
)

type Authenticator struct {
	oauthConfig *oauth2.Config
	store       sessions.Store
	mux         *http.ServeMux
}

type rebbleAuthResponse struct {
	BootOverrides any      `json:"boot_overrides"`
	HasTimeline   bool     `json:"has_timeline"`
	IsSubscribed  bool     `json:"is_subscribed"`
	IsWizard      bool     `json:"is_wizard"`
	Name          string   `json:"name"`
	Scopes        []string `json:"scopes"`
	TimelineTtl   int      `json:"timeline_ttl"`
	Uid           int      `json:"uid"`
}

func NewAuthenticator(store sessions.Store) *Authenticator {

	oauthConfig := &oauth2.Config{
		ClientID:     os.Getenv("OAUTH_CLIENT_ID"),
		ClientSecret: os.Getenv("OAUTH_CLIENT_SECRET"),
		Endpoint: oauth2.Endpoint{
			AuthURL:  os.Getenv("REBBLE_AUTH_URL"),
			TokenURL: os.Getenv("REBBLE_TOKEN_URL"),
		},
		RedirectURL: os.Getenv("OAUTH_CALLBACK_URL"),
		Scopes:      []string{"profile"},
	}
	a := &Authenticator{
		oauthConfig: oauthConfig,
		store:       store,
		mux:         http.NewServeMux(),
	}
	a.mux.HandleFunc("/login", a.handleLogin)
	a.mux.HandleFunc("/callback", a.handleCallback)
	return a
}

func (a *Authenticator) handleLogin(rw http.ResponseWriter, r *http.Request) {
	session, err := a.store.Get(r, "session")
	if err != nil {
		http.Error(rw, err.Error(), http.StatusInternalServerError)
		return
	}
	state := base64.URLEncoding.EncodeToString(securecookie.GenerateRandomKey(32))
	session.Values["oauth_state"] = state
	if err := session.Save(r, rw); err != nil {
		http.Error(rw, err.Error(), http.StatusInternalServerError)
		return
	}
	url := a.oauthConfig.AuthCodeURL(state)
	http.Redirect(rw, r, url, http.StatusFound)
}

func (a *Authenticator) handleCallback(rw http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	session, err := a.store.Get(r, "session")
	if err != nil {
		http.Error(rw, err.Error(), http.StatusInternalServerError)
		return
	}

	state := session.Values["oauth_state"]
	if state != r.URL.Query().Get("state") {
		http.Error(rw, "Invalid state", http.StatusBadRequest)
		return
	}
	delete(session.Values, "oauth_state")
	code := r.URL.Query().Get("code")
	token, err := a.oauthConfig.Exchange(r.Context(), code)
	if err != nil {
		http.Error(rw, err.Error(), http.StatusInternalServerError)
		return
	}

	client := a.oauthConfig.Client(ctx, token)
	resp, err := client.Get(os.Getenv("REBBLE_USER_INFO_URL"))
	if err != nil {
		http.Error(rw, err.Error(), http.StatusInternalServerError)
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		foo, _ := io.ReadAll(resp.Body)
		log.Println("content: " + string(foo))
		http.Error(rw, "Failed to get user info", http.StatusInternalServerError)
		return
	}
	var rebbleResp rebbleAuthResponse
	if err := json.NewDecoder(resp.Body).Decode(&rebbleResp); err != nil {
		http.Error(rw, fmt.Sprintf("decoding auth info failed: %v", err), http.StatusInternalServerError)
		return
	}
	if rebbleResp.Uid == 0 {
		http.Error(rw, "Invalid user id", http.StatusInternalServerError)
		return
	}
	session.Values["oauth_token"] = token.AccessToken
	session.Values["uid"] = rebbleResp.Uid
	if err := session.Save(r, rw); err != nil {
		http.Error(rw, err.Error(), http.StatusInternalServerError)
		return
	}
	http.Redirect(rw, r, "/", http.StatusFound)
}
