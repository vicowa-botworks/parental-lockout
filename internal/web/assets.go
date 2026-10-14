package web

import (
	"crypto/sha256"
	"embed"
	"encoding/hex"
	"errors"
	"io/fs"
	"path"
	"strings"
)

//go:embed static
var embedded embed.FS

// Assets is the embedded asset tree, rooted at static/ so the names callers use
// are the ones in a URL.
//
// Embedding is not a size decision. The daemon is installed by a script that copies
// one binary and writes one config file; a separate asset directory would be a
// second thing that install.sh can get wrong, and a 404 on the stylesheet makes
// the admin UI look broken in exactly the way that makes an operator distrust it.
func Assets() fs.FS {
	sub, err := fs.Sub(embedded, "static")
	if err != nil {
		// fs.Sub of an embedded directory cannot fail at runtime — the embed
		// directive is checked at compile time. Returning nil would be a silent
		// "no assets", which is a page with no stylesheet; panicking here is the
		// one place a loud failure is cheaper than a quiet one, because it can only
		// happen if the embed directive itself was edited into something wrong.
		panic("web: the embedded assets are not readable: " + err.Error())
	}
	return sub
}

// AssetURL is the hashed path an asset is served under, e.g.
// /static/app.9f2c1a7b44e0d1c3.css.
//
// The hash goes in the URL so the response can be `immutable` for a year. Without
// it, either the CSS is re-fetched on every page view on a machine that may be
// reaching the daemon over a flaky powerline adapter, or a fix to app.css does not
// appear on a parent's phone until their browser feels like revalidating. The
// second one is the bug that gets reported as "you didn't deploy it".
//
// The hash is over the bytes, so it changes exactly when the content does and not
// when the file's mtime does.
func AssetURL(fsys fs.FS, name string) (string, error) {
	b, err := fs.ReadFile(fsys, name)
	if err != nil {
		return "", err
	}
	if len(b) == 0 {
		return "", errors.New("asset " + name + " is empty")
	}
	sum := sha256.Sum256(b)
	ext := path.Ext(name)
	stem := strings.TrimSuffix(name, ext)
	return "/static/" + stem + "." + hex.EncodeToString(sum[:8]) + ext, nil
}

// AssetMap is every embedded asset under its hashed URL, computed once at startup.
//
// The router needs the reverse direction too — a request for
// /static/app.<hash>.css has to find `app.css` — and doing that by hashing every
// file per request is silly when there are two of them and they cannot change
// while the process runs.
func AssetMap(fsys fs.FS) (map[string]string, error) {
	out := map[string]string{}
	err := fs.WalkDir(fsys, ".", func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		url, err := AssetURL(fsys, p)
		if err != nil {
			return err
		}
		out[url] = p
		return nil
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}
