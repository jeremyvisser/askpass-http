package main

// Web-based frontend implementing systemd password agent protocol
//
// Copyright © 2024 Jeremy Visser
//
// References:
// - https://systemd.io/PASSWORD_AGENTS/
// - https://github.com/tazjin/yubikey-fde/
// - man:dracut.modules(7)

import (
	"bytes"
	"context"
	"errors"
	"flag"
	"fmt"
	"html/template"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"gopkg.in/ini.v1"
)

var (
	listen = flag.String("listen", "[::]:8080", "ADDR:PORT to bind to, or FD:n to use for socket activation")
	askDir = flag.String("askdir", "/run/systemd/ask-password", "Directory to watch for password prompts")
	cert   = flag.String("cert", "", "PEM-encoded TLS certificate. If unspecified, uses plain HTTP")
	key    = flag.String("key", "", "PEM-encoded TLS key. If -cert is specified, -key is required")
	idle   = flag.Duration("idle", 0, "Idle timeout after which server automatically shuts down")
)

var ErrMissingKey = errors.New("missing key")
var ErrExpired = errors.New("expired")

const WriteTimeout = 10 * time.Second

var (
	indexTmpl = template.Must(template.New("index").Parse(`<!doctype html>
<title>Askpass</title>
<h1>Askpass</h1>

<ul>
	{{ if not . }}
	<li>
		No ask prompts found. Refresh to try again.
	</li>
	{{ end }}
	{{ range $name, $ap := . }}
	<li>
		<form action="pass" method="post">
			<input type="hidden" name="ask" value="{{ $name }}" />
			<label>
				{{ $ap.Message }}
				<input type="password" name="answer" />
			</label>
			<input type="submit" value="Submit" />
		</form>
	</li>
	{{ end }}
</ul>
`))
)

type Askpass struct {
	Path     string    // /run/systemd/ask-password/<name>
	Message  string    // question to ask the user
	Icon     string    // optional, path to icon
	Socket   string    // socket to write the user-supplied password to
	NotAfter time.Time // ignore files after this date
}

func (a *Askpass) IsExpired() error {
	if a.NotAfter.IsZero() {
		return nil
	}
	if now := time.Now(); now.After(a.NotAfter) {
		return fmt.Errorf("%w: current time (%s) > NotAfter (%s)", ErrExpired, now, a.NotAfter)
	}
	return nil
}

func (a *Askpass) UnmarshalINI(path string) error {
	f, err := ini.Load(path)
	if err != nil {
		return err
	}
	*a = Askpass{
		Path:     path,
		Message:  f.Section("Ask").Key("Message").String(),
		Icon:     f.Section("Ask").Key("Icon").String(),
		Socket:   f.Section("Ask").Key("Socket").String(),
		NotAfter: f.Section("Ask").Key("NotAfter").MustTime(time.Time{}),
	}
	for _, kv := range []struct{ key, val string }{
		{"Message", a.Message},
		{"Socket", a.Socket},
	} {
		if kv.val == "" {
			return fmt.Errorf("%w: %v", ErrMissingKey, kv.key)
		}
	}
	return nil
}

// Answer writes the password answer to the Socket
func (a *Askpass) Answer(s string) error {
	sock, err := net.Dial("unixgram", a.Socket)
	if err != nil {
		return err
	}
	defer sock.Close()
	_ = sock.SetDeadline(time.Now().Add(WriteTimeout))
	var buf bytes.Buffer
	buf.WriteByte('+') // '+' = answer, '-' = cancel
	buf.WriteString(s)
	if n, err := sock.Write(buf.Bytes()); err != nil {
		return err
	} else if n < len(s) {
		return io.ErrShortWrite
	}
	return nil
}

func NewAskpass(name string) (*Askpass, error) {
	var ap Askpass
	path := filepath.Join(*askDir, name)
	if err := ap.UnmarshalINI(path); err != nil {
		return nil, err
	}
	if err := ap.IsExpired(); err != nil {
		return nil, err
	}
	return &ap, nil
}

type Askers map[string]*Askpass

// Find returns the Askpass, or returns nil if not found.
//
// name can safely contain untrusted input, as it is only used to find an
// existing key, and is not passed to the filesystem or reused elsewhere.
func (a Askers) Find(name string) *Askpass {
	ap, ok := a[name]
	if !ok {
		return nil
	}
	return ap
}

// NewAskers enumerates the prompts currently existing.
// To avoid passing untrusted input to the filesystem, no input is accepted.
func NewAskers() Askers {
	// List the askers:
	d, err := os.ReadDir(*askDir)
	if err != nil {
		log.Println(err)
		return nil
	}

	// Parse and prepare output:
	out := make(Askers)
	for _, entry := range d {
		if strings.HasPrefix(entry.Name(), "ask.") && !entry.IsDir() {
			ap, err := NewAskpass(entry.Name())
			if err != nil {
				log.Println(err)
				continue
			}
			out[entry.Name()] = ap
		}
	}
	return out
}

func ServePass(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// Find the requested asker:
	ap := NewAskers().Find(r.FormValue("ask"))
	if ap == nil {
		Error(w, "Not found", http.StatusNotFound)
		return
	}

	// Provide the answer:
	if err := ap.Answer(r.FormValue("answer")); err != nil {
		Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Success:
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func ServeIndex(w http.ResponseWriter, r *http.Request) {
	if err := indexTmpl.Execute(w, NewAskers()); err != nil {
		log.Println(err)
	}
}

func Error(w http.ResponseWriter, error string, code int) {
	log.Println(code, error)
	http.Error(w, error, code)
}

// Listener is similar to net.Listen, except it supports inetd-style sockets
// via the fd:0 syntax (where 0 is the fd number).
func Listener(addr string) (net.Listener, error) {
	if fdstr, ok := strings.CutPrefix(*listen, "fd:"); ok {
		fd, err := strconv.Atoi(fdstr)
		if err != nil {
			return nil, err
		}
		f := os.NewFile(uintptr(fd), *listen)
		defer f.Close()
		return net.FileListener(f)
	} else {
		return net.Listen("tcp", addr)
	}
}

// NewIdleHandler returns a http.Handler that calls shutdownFunc if no
// requests are received within shutdownIdle time.
//
// Once the grace period expires, existing connections are forcibly closed.
// The channel done is closed when shutdown finishes, or the grace period expires,
// whichever comes first.
//
// If shutdownIdle is 0, the idle timeout is disabled and is a no-op.
func NewIdleHandler(shutdownIdle time.Duration, shutdownFunc func(context.Context) error,
	handler http.Handler) (idleHandler http.Handler, done <-chan struct{}) {

	const gracePeriod = 30 * time.Second

	if shutdownIdle > 0 {
		ctx, cancel := context.WithCancel(context.Background())
		t := time.AfterFunc(shutdownIdle, func() {
			log.Printf("Server was idle for %.0f sec. Closing within %.0f sec...",
				shutdownIdle.Seconds(), gracePeriod.Seconds())
			ctx, _ := context.WithTimeout(context.Background(), gracePeriod)
			defer cancel()
			shutdownFunc(ctx)
		})
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			t.Reset(shutdownIdle)
			handler.ServeHTTP(w, r)
		}), ctx.Done()
	}

	return handler, nil
}

func main() {
	flag.Parse()
	http.HandleFunc("/", ServeIndex)
	http.HandleFunc("/pass", ServePass)
	http.HandleFunc("/robots.txt", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, "User-Agent: *\nDisallow: /\n")
		log.Println("/robots.txt was requested. Please do NOT expose this to the internet. *facepalm*")
	})

	lsn, err := Listener(*listen)
	if err != nil {
		log.Fatal(err)
	}
	var srv http.Server
	h, done := NewIdleHandler(*idle, srv.Shutdown, http.DefaultServeMux)
	srv.Handler = h
	if *cert > "" {
		log.Printf("Listening on https://%s", lsn.Addr())
		err = fmt.Errorf("http.Server: ServeTLS: %w", srv.ServeTLS(lsn, *cert, *key))
	} else {
		log.Printf("Listening on http://%s", lsn.Addr())
		err = fmt.Errorf("http.Server: Serve: %w", srv.Serve(lsn))
	}
	if err != nil {
		if errors.Is(err, http.ErrServerClosed) {
			<-done // wait for shutdown to finish
			return // success
		}
		log.Fatal(err)
	}
}
