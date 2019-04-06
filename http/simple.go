package WASABIhttps

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/cloudkucooland/WASABI"
	"github.com/gorilla/mux"
)

func uploadRoute(res http.ResponseWriter, req *http.Request) {
	var err error
	doc := WASABI.SimpleDocument{}
	exp := "14d"

	// Parse form and get content
	req.Body = http.MaxBytesReader(res, req.Body, WASABI.MaxFilesize+1024) // MaxFilesize + 1KB metadata
	contentType := strings.Split(strings.Replace(strings.ToLower(req.Header.Get("Content-Type")), " ", "", -1), ";")[0]

	// Get the document, however the request is formatted
	if req.Method == "POST" && contentType == "application/x-www-form-urlencoded" {
		// Parse form
		err = req.ParseForm()
		if err != nil {
			WASABI.Log.Error(err)
			res.WriteHeader(http.StatusInternalServerError)
			fmt.Fprintf(res, err.Error())
			return
		}
		doc.Content = req.PostFormValue("Q")
	} else if req.Method == "POST" && contentType == "multipart/form-data" {
		// Parse form
		err = req.ParseMultipartForm(WASABI.MaxFilesize + 1024)
		if err != nil {
			WASABI.Log.Error(err)
			res.WriteHeader(http.StatusInternalServerError)
			fmt.Fprintf(res, err.Error())
			return
		}
		// Get document
		doc.Content = req.PostFormValue("Q")
		if doc.Content == "" { // Oh no, it's a file!
			// Get file
			file, _, err := req.FormFile("Q")
			if err != nil && err.Error() == "http: no such file" {
				res.WriteHeader(http.StatusBadRequest)
				fmt.Fprintf(res, "The document can't be empty.\n")
				return
			}
			if err != nil {
				WASABI.Log.Error(err)
				res.WriteHeader(http.StatusInternalServerError)
				fmt.Fprintf(res, err.Error())
				return
			}

			// Read document
			content, err := ioutil.ReadAll(file)
			if err != nil {
				WASABI.Log.Error(err)
				res.WriteHeader(http.StatusInternalServerError)
				fmt.Fprintf(res, err.Error())
				return
			}
			doc.Content = string(content)
		}
	} else { // PUT or POST with non-form
		// Read document
		content, err := ioutil.ReadAll(req.Body)
		if err != nil {
			WASABI.Log.Error(err)
			res.WriteHeader(http.StatusInternalServerError)
			fmt.Fprintf(res, err.Error())
			return
		}
		doc.Content = string(content)
	}

	// Check exact filesize
	if len(doc.Content) > WASABI.MaxFilesize {
		res.WriteHeader(http.StatusRequestEntityTooLarge)
		fmt.Fprintf(res, "Maximum document size exceeded.\n")
		return
	}

	if len(strings.TrimSpace(doc.Content)) < 1 {
		res.WriteHeader(http.StatusBadRequest)
		fmt.Fprintf(res, "The document can't be empty (after whitespace removal).\n")
		return
	}

	/* Read metadata */
	if req.Header.Get("E") != "" {
		exp = req.Header.Get("E")
	} else if req.FormValue("E") != "" {
		exp = req.FormValue("E")
	}
	doc.Expiration, err = parseExpiration(exp)
	if err != nil {
		res.WriteHeader(http.StatusBadRequest)
		fmt.Fprintf(res, "Invalid expiration.\n")
		return
	}

	err = WASABI.Store(&doc)
	if err != nil && err.Error() == "file contains 0x00 bytes" {
		res.WriteHeader(http.StatusBadRequest)
		fmt.Fprintf(res, "You are trying to upload a binary file, which is not supported.\n")
		return
	} else if err != nil {
		WASABI.Log.Error(err)
		res.WriteHeader(http.StatusInternalServerError)
		fmt.Fprintf(res, err.Error())
		return
	}

	fmt.Fprintf(res, config.Root+"/simple/"+doc.ID+"\n")
}

func getRoute(res http.ResponseWriter, req *http.Request) {
	vars := mux.Vars(req)
	id := vars["document"]

	doc, err := WASABI.Request(id)
	if err != nil {
		notFoundRoute(res, req)
	}

	res.Header().Add("Content-Type", "text/plain; charset=utf-8")
	fmt.Fprintf(res, "%s", doc.Content)
}

// ParseExpiration creates a time.Time object from an expiration string, taking the units m, h, d, w into account.
func parseExpiration(expiration string) (time.Time, error) {
	expiration = strings.ToLower(strings.TrimSpace(expiration))
	if expiration == "volatile" {
		return time.Unix(-1, 0), nil
	}

	var multiplier int64

	if strings.HasSuffix(expiration, "h") {
		expiration = strings.TrimSuffix(expiration, "h")
		multiplier = 60
	} else if strings.HasSuffix(expiration, "d") {
		expiration = strings.TrimSuffix(expiration, "d")
		multiplier = 60 * 24
	} else if strings.HasSuffix(expiration, "w") {
		expiration = strings.TrimSuffix(expiration, "w")
		multiplier = 60 * 24 * 7
	} else {
		expiration = strings.TrimSuffix(expiration, "m")
		multiplier = 1
	}

	value, err := strconv.ParseInt(expiration, 10, 0)
	if err != nil {
		return time.Time{}, err
	}

	if multiplier*value == 0 {
		return time.Time{}, nil
	}

	expirationTime := time.Now().Add(time.Duration(multiplier*value) * time.Minute)

	return expirationTime, nil
}
