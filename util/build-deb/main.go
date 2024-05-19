package main

import (
	"log"
	"os"
	"path"
	"time"

	"github.com/google/rpmpack"
)

var (
	metadata = rpmpack.RPMMetaData{
		Name:        "askpass-http",
		Summary:     "Askpass HTTP server",
		Description: "Lets you unlock your disk from the moon. Roaming charges may apply.",
		Version:     "0",
		Release:     "0~" + time.Now().Format("20060102"),
		Licence:     "MIT",
		Requires: rpmpack.Relations{
			&rpmpack.Relation{Name: "systemd"},
			&rpmpack.Relation{Name: "dracut"},
		},
	}

	files = []rpmpack.RPMFile{
		{
			Name:  "/usr/bin/askpass-http",
			Mode:  0755,
			Owner: "root",
			Group: "root",
		},
		{
			Name:  "/usr/lib/systemd/system/askpass-http.path",
			Mode:  0644,
			Owner: "root",
			Group: "root",
		},
		{
			Name:  "/usr/lib/systemd/system/askpass-http.socket",
			Mode:  0644,
			Owner: "root",
			Group: "root",
		},
		{
			Name:  "/usr/lib/systemd/system/askpass-http.service",
			Mode:  0644,
			Owner: "root",
			Group: "root",
		},
		{
			Name:  "/usr/lib/dracut/modules.d/98askpasshttp/module-setup.sh",
			Mode:  0755,
			Owner: "root",
			Group: "root",
		},
	}
)

const (
	rpmFile = "askpass-http.rpm"

	posttrans = `
systemctl daemon-reload
if [[ $1 -ge 1 ]]; then
	dracut -f
fi
`

	preun = `
if [[ $1 == 0 ]]; then
    systemctl disable --now \
        askpass-http.path \
        askpass-http.socket \
        askpass-http.service
    dracut -f
fi`
)

func main() {
	rpm, err := rpmpack.NewRPM(metadata)
	if err != nil {
		log.Fatal(err)
	}

	for _, f := range files {
		if f.Body == nil {
			// Load body from file, trying in order:
			//   full/path/to/file
			//   ./file
			fname := f.Name
			if fname[0] == '/' {
				fname = fname[1:]
			}
			f.Body, err = os.ReadFile(fname)
			if err != nil {
				_, fname := path.Split(fname)
				var err2 error
				if f.Body, err2 = os.ReadFile(fname); err2 != nil {
					log.Fatal(err)
				}
			}
		}
		rpm.AddFile(f)
	}

	rpm.AddPosttrans(posttrans)
	rpm.AddPreun(preun)

	out, err := os.Create(rpmFile)
	if err != nil {
		log.Fatal(err)
	}
	if err := rpm.Write(out); err != nil {
		log.Fatal(err)
	}
}
