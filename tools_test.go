// Package toolkit package
package toolkit

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"image"
	"image/png"
	"io"
	"io/ioutil"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"sync"
	"testing"
)

type RoundTripFunc func(req *http.Request) *http.Response

func (f RoundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req), nil
}

func NewTestClient(fn RoundTripFunc) *http.Client {
	return &http.Client{
		Transport: fn,
	}
}

func TestTools_PushJSONToRemote(t *testing.T) {
	client := NewTestClient(func(req *http.Request) *http.Response {
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       ioutil.NopCloser(bytes.NewBufferString("ok")),
			Header:     make(http.Header),
		}
	})

	var testTools Tools
	var foo struct {
		Bar string `json:"bar"`
	}
	foo.Bar = "bar"

	_, _, err := testTools.PushJSONToRemote("http://example.com/some/path", foo, client)
	if err != nil {
		t.Error("failed to call remote url", err)
	}
}

func TestTools_RandomString(t *testing.T) {
	var testTools Tools

	s := testTools.RandomString(10)
	if len(s) != 10 {
		t.Error("wants: 10. got: ", len(s))
		return
	}
}

type uploadTest = struct {
	name          string
	allowedTypes  []string
	renameFile    bool
	errorExpected bool
}

var uploadTests = []struct {
	name          string
	allowedTypes  []string
	renameFile    bool
	errorExpected bool
}{
	{name: "allowed no rename", allowedTypes: []string{"image/jpeg", "image/png", "image/gif"}, renameFile: false, errorExpected: false},
	{name: "allowed rename", allowedTypes: []string{"image/jpeg", "image/png", "image/gif"}, renameFile: true, errorExpected: false},
	{name: "not allowed", allowedTypes: []string{"image/jpeg", "image/gif"}, renameFile: false, errorExpected: true},
}

func TestTools_UploadMultipleFiles(t *testing.T) {
	for _, entry := range uploadTests {
		// TODO: setup pipes to avoid buffering
		pr, pw := io.Pipe()
		writer := multipart.NewWriter(pw)
		wg := sync.WaitGroup{}
		wg.Add(1)

		go func() {
			defer writer.Close()
			defer wg.Done()

			part, err := writer.CreateFormFile("file", "./test_data/img.png")
			if err != nil {
				t.Error(err)
			}

			f, err := os.Open("./test_data/img.png")
			if err != nil {
				t.Error(err)
			}
			defer f.Close()

			img, _, err := image.Decode(f)
			if err != nil {
				t.Error("error decoding image", err)
			}

			err = png.Encode(part, img)
			if err != nil {
				t.Error(err)
			}
		}()

		request := httptest.NewRequest("POST", "/", pr)
		request.Header.Add("Content-Type", writer.FormDataContentType())

		var testTools Tools
		testTools.AllowedFileTypes = entry.allowedTypes

		uploadedFiles, err := testTools.UploadFiles(request, "./test_data/uploads/", entry.renameFile)
		if err != nil && !entry.errorExpected {
			t.Error(err)
		}

		if !entry.errorExpected {
			if _, err = os.Stat(fmt.Sprintf("./test_data/uploads/%s", uploadedFiles[0].NewFileName)); os.IsNotExist(err) {
				t.Errorf("%s: expected file to exist: %s", entry.name, err.Error())
			}

			// clean up
			_ = os.Remove(fmt.Sprintf("./test_data/uploads/%s", uploadedFiles[0].NewFileName))
		}

		if !entry.errorExpected && err != nil {
			t.Errorf("%s: error expected but none recieved", entry.name)
		}

		wg.Wait()
	}
}

func TestTools_UploadSingleFile(t *testing.T) {
	// TODO: setup pipes to avoid buffering
	pr, pw := io.Pipe()
	writer := multipart.NewWriter(pw)
	entry := uploadTest{name: "single", allowedTypes: []string{"image/jpeg", "image/png", "image/gif"}, renameFile: false, errorExpected: false}

	go func() {
		defer writer.Close()

		part, err := writer.CreateFormFile("file", "./test_data/img.png")
		if err != nil {
			t.Error(err)
		}

		f, err := os.Open("./test_data/img.png")
		if err != nil {
			t.Error(err)
		}
		defer f.Close()

		img, _, err := image.Decode(f)
		if err != nil {
			t.Error("error decoding image", err)
		}

		err = png.Encode(part, img)
		if err != nil {
			t.Error(err)
		}
	}()

	request := httptest.NewRequest("POST", "/", pr)
	request.Header.Add("Content-Type", writer.FormDataContentType())

	var testTools Tools
	testTools.AllowedFileTypes = entry.allowedTypes

	uploadedFile, err := testTools.UploadSingleFile(request, "./test_data/uploads/", entry.renameFile)
	if err != nil && !entry.errorExpected {
		t.Error(err)
	}

	if !entry.errorExpected {
		if _, err = os.Stat(fmt.Sprintf("./test_data/uploads/%s", uploadedFile.NewFileName)); os.IsNotExist(err) {
			t.Errorf("%s: expected file to exist: %s", entry.name, err.Error())
		}

		// clean up
		_ = os.Remove(fmt.Sprintf("./test_data/uploads/%s", uploadedFile.NewFileName))
	}

	if !entry.errorExpected && err != nil {
		t.Errorf("%s: error expected but none recieved", entry.name)
	}
}

func TestTools_CreateDir(t *testing.T) {
	var testTools Tools
	err := testTools.CreateDir("./test_data/my_dir")
	if err != nil {
		t.Error(err)
	}
	err = testTools.CreateDir("./test_data/my_dir")
	if err != nil {
		t.Error(err)
	}

	_ = os.Remove("./test_data/my_dir")
}

var slugTests = []struct {
	name          string
	s             string
	expected      string
	errorExpected bool
}{
	{name: "valid string", s: "now is the time 1234", expected: "now-is-the-time-1234", errorExpected: false},
	{name: "empty string", s: "", expected: "", errorExpected: true},
	{name: "complex string", s: "  the TiMe/= is! !now+   ", expected: "the-time-is-now", errorExpected: false},
	{name: "japanese string", s: "こんにちは世界", expected: "", errorExpected: true},
	{name: "japanese string and roman string", s: "hello world こんにちは世界", expected: "hello-world", errorExpected: false},
}

func TestTools_Sluggify(t *testing.T) {
	var testTools Tools

	for _, st := range slugTests {
		slug, err := testTools.Slugify(st.s)
		if err != nil && !st.errorExpected {
			t.Errorf("%s: unexpected error: %s", st.name, err)
		}

		if !st.errorExpected && slug != st.expected {
			t.Errorf("%s: unexpected string output; expected: %s got: %s; %s ", st.name, st.expected, slug, err)
		}
	}
}

func TestTools_DownloadStaticFile(t *testing.T) {
	rr := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/", nil) // var testTools Tools

	var testTool Tools

	testTool.DownloadStaticFile(rr, req, "./test_data", "pic.jpg", "puppy.jpg")

	res := rr.Result()
	defer res.Body.Close()

	if res.Header["Content-Length"][0] != "108096" {
		t.Error("wrong content length of", res.Header["Content-Length"][0])
	}

	if res.Header["Content-Disposition"][0] != "attachment; filename=\"puppy.jpg\"" {
		t.Error("wrong content disposition")
	}

	_, err := ioutil.ReadAll(res.Body)
	if err != nil {
		t.Error(err)
	}
}

var jsonTests = []struct {
	name          string
	json          string
	errorExpected bool
	maxSize       int
	allowUnknown  bool
}{
	{name: "good json", json: `{"foo": "bar"}`, errorExpected: false, maxSize: 1024, allowUnknown: false},
	{name: "badly formatted json", json: `{"foo": }`, errorExpected: true, maxSize: 1024, allowUnknown: false},
	{name: "incorrect type", json: `{"foo": 1}`, errorExpected: true, maxSize: 1024, allowUnknown: false},
	{name: "two json files", json: `{"foo": ""}{"alpha": "beta"}`, errorExpected: true, maxSize: 1024, allowUnknown: false},
	{name: "empty body", json: ``, errorExpected: true, maxSize: 1024, allowUnknown: false},
	{name: "syntax error in json", json: `{"foo": 1"}`, errorExpected: true, maxSize: 1024, allowUnknown: false},
	{name: "unknown field in json", json: `{"fooo": "bar"}`, errorExpected: true, maxSize: 1024, allowUnknown: false},
	{name: "allow fields in json", json: `{"fooo": "bar"}`, errorExpected: false, maxSize: 1024, allowUnknown: true},
	{name: "missing field name", json: `{fooo: "bar"}`, errorExpected: true, maxSize: 1024, allowUnknown: true},
	{name: "file too large", json: `{"foo": "bar"}`, errorExpected: true, maxSize: 5, allowUnknown: false},
	{name: "not json", json: `fuck tests`, errorExpected: true, maxSize: 1024, allowUnknown: false},
}

func TestTools_ReadJSON(t *testing.T) {
	var testTool Tools

	for _, e := range jsonTests {
		testTool.MaxJSONSize = e.maxSize

		testTool.AllowUnknownFields = e.allowUnknown

		var decodedJSON struct {
			Foo string `json:"foo"`
		}

		req, err := http.NewRequest("POST", "/", bytes.NewReader([]byte(e.json)))
		if err != nil {
			t.Log("Error: ", err)
		}

		rr := httptest.NewRecorder()

		err = testTool.ReadJSON(rr, req, &decodedJSON)
		if e.errorExpected && err == nil {
			t.Errorf("%s: error expected, but none recieved", e.name)
		}

		if !e.errorExpected && err != nil {
			t.Errorf("%s: error not expected, but one received: %s", e.name, err.Error())
		}
		req.Body.Close()
	}
}

func TestTools_WriteJSON(t *testing.T) {
	var testTools Tools
	rr := httptest.NewRecorder()
	payload := JSONResponse{
		Error:   false,
		Message: "foo",
	}

	headers := make(http.Header)
	headers.Add("FOO", "BAR")

	err := testTools.WriteJSON(rr, http.StatusOK, payload, headers)
	if err != nil {
		t.Errorf("failed to write JSON: %v", err)
	}
}

func TestTools_ErrorJSON(t *testing.T) {
	var testTools Tools
	rr := httptest.NewRecorder()
	err := testTools.ErrorJSON(rr, errors.New("some error"), http.StatusServiceUnavailable)
	if err != nil {
		t.Error(err)
	}

	var payload JSONResponse

	decoder := json.NewDecoder(rr.Body)
	err = decoder.Decode(&payload)
	if err != nil {
		t.Error("recieved error while decoding JSON", err)
	}

	if !payload.Error {
		t.Error("error set to false in JSON, should be true")
	}

	if rr.Code != http.StatusServiceUnavailable {
		t.Errorf("wrong status code returned, expected 503 but got %d", rr.Code)
	}
}
