package gemquick

import (
	"encoding/json"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"net/http"
	"path"
	"path/filepath"
)

func (g *Gemquick) ReadJson(w http.ResponseWriter, r *http.Request, data interface{}) error {
	maxBytes := 1048576 // 1MB
	r.Body = http.MaxBytesReader(w, r.Body, int64(maxBytes))
	dec := json.NewDecoder(r.Body)
	err := dec.Decode(&data)
	if err != nil {
		return err
	}

	err = dec.Decode(&struct{}{})
	if err != io.EOF {
		return errors.New("request body must only contain a single JSON object")
	}

	return nil
}

func (g *Gemquick) WriteJson(w http.ResponseWriter, status int, data interface{}, headers ...http.Header) error {
	out, err := json.MarshalIndent(data, "", "\t")
	if err != nil {
		return err
	}

	w = setHeaders(w, status, headers, "application/json")
	_, err = w.Write(out)
	if err != nil {
		return err
	}

	return nil
}

func (g *Gemquick) WriteXML(w http.ResponseWriter, status int, data interface{}, headers ...http.Header) error {
	out, err := xml.MarshalIndent(data, "", "\t")
	if err != nil {
		return err
	}

	w = setHeaders(w, status, headers, "application/xml")
	_, err = w.Write(out)
	if err != nil {
		return err
	}

	return nil
}

func (g *Gemquick) DownloadFile(w http.ResponseWriter, r *http.Request, pathToFile, filename string) error {
	fp := path.Join(pathToFile, filename)
	fileToServe := filepath.Clean(fp)

	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%s", filename))
	w.Header().Set("Content-Type", fmt.Sprintf("attachment; file=%s", filename))
	http.ServeFile(w, r, fileToServe)

	return nil
}

func (g *Gemquick) Error404(w http.ResponseWriter, r *http.Request) {
	g.ErrorStatus(w, http.StatusNotFound)
}

func (g *Gemquick) Error500(w http.ResponseWriter, r *http.Request) {
	g.ErrorStatus(w, http.StatusInternalServerError)
}

func (g *Gemquick) ErrorUnauthorized(w http.ResponseWriter, r *http.Request) {
	g.ErrorStatus(w, http.StatusUnauthorized)
}

func (g *Gemquick) ErrorForbidden(w http.ResponseWriter, r *http.Request) {
	g.ErrorStatus(w, http.StatusForbidden)
}

func (g *Gemquick) ErrorStatus(w http.ResponseWriter, status int) {
	http.Error(w, http.StatusText(status), status)
}

func setHeaders(w http.ResponseWriter, status int, headers []http.Header, contentType string) http.ResponseWriter {
	if len(headers) > 0 {
		for key, value := range headers[0] {
			w.Header()[key] = value
		}
	}
	w.Header().Set("Content-Type", contentType)
	w.WriteHeader(status)

	return w
}
