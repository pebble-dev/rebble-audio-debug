package adm

import (
	"cloud.google.com/go/storage"
	"context"
	"embed"
	"encoding/base64"
	"errors"
	"github.com/gorilla/sessions"
	"google.golang.org/api/iterator"
	"html/template"
	"log"
	"net/http"
	"path"
	"slices"
	"strconv"
	"strings"
	"time"
)

//go:embed templates
var templateFS embed.FS

var templates = map[string]*template.Template{
	"recordings.html": template.Must(template.ParseFS(templateFS, "templates/base.html", "templates/recordings.html")),
}

type RecordingViewer struct {
	store  sessions.Store
	mux    *http.ServeMux
	sc     *storage.Client
	bucket *storage.BucketHandle
}

func NewRecordingViewer(store sessions.Store) *RecordingViewer {
	sc, err := storage.NewClient(context.Background())
	if err != nil {
		log.Fatalf("Failed to create client: %v", err)
	}
	bucket := sc.Bucket("rebble-audio-debug")
	rv := &RecordingViewer{
		mux:    http.NewServeMux(),
		store:  store,
		sc:     sc,
		bucket: bucket,
	}
	rv.mux.HandleFunc("/recordings", rv.handleRecordings)
	return rv
}

type recording struct {
	Date         time.Time
	RelativeTime time.Duration
	URL          string
	Language     string
	Transcript   string
}
type recordingData struct {
	Recordings []recording
}

func (rv *RecordingViewer) handleRecordings(rw http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	// Check if user is authenticated
	session, err := rv.store.Get(r, "session")
	if err != nil {
		http.Error(rw, err.Error(), http.StatusInternalServerError)
		return
	}
	uid, ok := session.Values["uid"]
	if !ok {
		http.Redirect(rw, r, "/auth/login", http.StatusFound)
		return
	}

	rd := recordingData{}

	// List all recordings
	it := rv.bucket.Objects(ctx, &storage.Query{Prefix: "audio/users/" + strconv.Itoa(uid.(int)) + "/recording-"})
	for {
		obj, err := it.Next()
		if errors.Is(err, iterator.Done) {
			break
		}
		if err != nil {
			http.Error(rw, err.Error(), http.StatusInternalServerError)
			return
		}
		url, err := rv.bucket.SignedURL(obj.Name, &storage.SignedURLOptions{
			Scheme:  storage.SigningSchemeV4,
			Method:  "GET",
			Expires: time.Now().Add(30 * time.Minute),
		})
		if err != nil {
			continue
		}
		filename, _ := strings.CutSuffix(path.Base(obj.Name), ".wav")
		when, err := time.Parse("recording-2006-01-02T15:04:05.000000", filename)
		if err != nil {
			when = time.Time{}
		}
		language, ok := obj.Metadata["rebble-language"]
		if !ok {
			language = "(unknown)"
		}
		transcript, ok := obj.Metadata["rebble-transcript"]
		if !ok {
			transcript = "(no transcript)"
		} else {
			if ts, err := base64.StdEncoding.DecodeString(transcript); err != nil {
				transcript = "(error decoding transcript)"
			} else {
				transcript = string(ts)
			}
		}
		// Only show recordings from the last 24 hours. There shouldn't be any older than that anyway.
		if time.Now().Sub(when) >= 24*time.Hour {
			continue
		}
		rd.Recordings = append(rd.Recordings, recording{
			Date:         when,
			RelativeTime: time.Now().Sub(when).Round(time.Second),
			URL:          url,
			Language:     language,
			Transcript:   transcript,
		})
	}
	slices.Reverse(rd.Recordings)

	if err := templates["recordings.html"].ExecuteTemplate(rw, "base.html", rd); err != nil {
		log.Printf("Error executing template: %v\n", err)
		http.Error(rw, err.Error(), http.StatusInternalServerError)
	}
}
