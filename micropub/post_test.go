package micropub

import (
	"bytes"
	"context"
	"io"
	"io/ioutil"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"hawx.me/code/assert"
)

func withScope(scope string, handler http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handler.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), "__hawx.me/code/tally-ho:Scopes__", []string{scope})))
	})
}

type fakePostDB struct {
	datas                   []map[string][]interface{}
	replaces, adds, deletes map[string][]map[string][]interface{}
	deleteAlls              map[string][][]string
	deleted                 []string
	undeleted               []string
}

func (b *fakePostDB) Create(data map[string][]interface{}) (string, error) {
	b.datas = append(b.datas, data)

	return "http://example.com/blog/p/1", nil
}

func (b *fakePostDB) Update(
	id string,
	replace, add, delete map[string][]interface{},
	deleteAlls []string,
) error {
	b.replaces[id] = append(b.replaces[id], replace)
	b.adds[id] = append(b.adds[id], add)
	b.deletes[id] = append(b.deletes[id], delete)
	b.deleteAlls[id] = append(b.deleteAlls[id], deleteAlls)

	return nil
}

func (b *fakePostDB) Delete(url string) error {
	b.deleted = append(b.deleted, url)
	return nil
}

func (b *fakePostDB) Undelete(url string) error {
	b.undeleted = append(b.undeleted, url)
	return nil
}

type fakeFileWriter struct {
	data []string
}

func (fw *fakeFileWriter) WriteFile(name, contentType string, r io.Reader) (string, error) {
	data, _ := ioutil.ReadAll(r)
	fw.data = append(fw.data, string(data))

	return "http://example.com/" + name, nil
}

func TestPostEntry(t *testing.T) {
	testCases := map[string]func(string) (*http.Response, error){
		"url-encoded-form": func(u string) (*http.Response, error) {
			return http.PostForm(u, url.Values{
				"h":            {"entry"},
				"content":      {"This is a test"},
				"category[]":   {"test", "ignore"},
				"mp-something": {"what"},
				"url":          {"what"},
			})
		},
		"json": func(u string) (*http.Response, error) {
			return http.Post(u, "application/json", strings.NewReader(`{
  "type": ["h-entry"],
  "properties": {
    "content": ["This is a test"],
    "category": ["test", "ignore"],
    "mp-something": ["what"],
    "url": ["http://what"]
  }
}`))
		},
		"multipart-form": func(u string) (*http.Response, error) {
			var buf bytes.Buffer
			writer := multipart.NewWriter(&buf)

			var werr error
			writeField := func(key, value string) {
				part, err := writer.CreateFormField(key)
				if err != nil {
					werr = err
				}
				io.WriteString(part, value)
			}

			writeField("h", "entry")
			writeField("content", "This is a test")
			writeField("category[]", "test")
			writeField("category[]", "ignore")
			writeField("mp-something", "what")
			writeField("url", "what")
			if err := writer.Close(); err != nil {
				return nil, err
			}
			if werr != nil {
				return nil, werr
			}

			req, err := http.NewRequest("POST", u, &buf)
			if err != nil {
				return nil, err
			}
			req.Header.Set("Content-Type", writer.FormDataContentType())

			return http.DefaultClient.Do(req)
		},
	}

	for name, f := range testCases {
		t.Run(name, func(t *testing.T) {
			assert := assert.New(t)
			blog := &fakePostDB{}

			s := httptest.NewServer(withScope("create", postHandler(blog, nil)))
			defer s.Close()

			resp, err := f(s.URL)

			assert.Nil(err)
			assert.Equal(http.StatusCreated, resp.StatusCode)
			assert.Equal("http://example.com/blog/p/1", resp.Header.Get("Location"))

			if assert.Len(blog.datas, 1) {
				data := blog.datas[0]

				assert.Equal("entry", data["h"][0])
				assert.Equal("This is a test", data["content"][0])
				assert.Equal("test", data["category"][0])
				assert.Equal("ignore", data["category"][1])
				assert.Equal("what", data["mp-something"][0])

				_, ok := data["url"]
				assert.False(ok)
			}
		})
	}
}

func TestPostEntryMissingScope(t *testing.T) {
	testCases := map[string]func(string) (*http.Response, error){
		"url-encoded-form": func(u string) (*http.Response, error) {
			return http.PostForm(u, url.Values{
				"h":            {"entry"},
				"content":      {"This is a test"},
				"category[]":   {"test", "ignore"},
				"mp-something": {"what"},
				"url":          {"what"},
			})
		},
		"json": func(u string) (*http.Response, error) {
			return http.Post(u, "application/json", strings.NewReader(`{
  "type": ["h-entry"],
  "properties": {
    "content": ["This is a test"],
    "category": ["test", "ignore"],
    "mp-something": ["what"],
    "url": ["http://what"]
  }
}`))
		},
		"multipart-form": func(u string) (*http.Response, error) {
			var buf bytes.Buffer
			writer := multipart.NewWriter(&buf)

			var werr error
			writeField := func(key, value string) {
				part, err := writer.CreateFormField(key)
				if err != nil {
					werr = err
				}
				io.WriteString(part, value)
			}

			writeField("h", "entry")
			writeField("content", "This is a test")
			writeField("category[]", "test")
			writeField("category[]", "ignore")
			writeField("mp-something", "what")
			writeField("url", "what")
			if err := writer.Close(); err != nil {
				return nil, err
			}
			if werr != nil {
				return nil, werr
			}

			req, err := http.NewRequest("POST", u, &buf)
			if err != nil {
				return nil, err
			}
			req.Header.Set("Content-Type", writer.FormDataContentType())

			return http.DefaultClient.Do(req)
		},
	}

	for name, f := range testCases {
		t.Run(name, func(t *testing.T) {
			assert := assert.New(t)
			blog := &fakePostDB{}

			s := httptest.NewServer(postHandler(blog, nil))
			defer s.Close()

			resp, err := f(s.URL)

			assert.Nil(err)
			assert.Equal(http.StatusUnauthorized, resp.StatusCode)
			assert.Len(blog.datas, 0)
		})
	}
}

func TestPostEntryMultipartFormWithMedia(t *testing.T) {
	for _, key := range []string{"photo", "video", "audio"} {
		t.Run(key, func(t *testing.T) {
			assert := assert.New(t)
			file := "this is an image"
			db := &fakePostDB{}
			fw := &fakeFileWriter{}

			s := httptest.NewServer(withScope("create", postHandler(db, fw)))
			defer s.Close()

			var buf bytes.Buffer
			writer := multipart.NewWriter(&buf)

			writeField := func(key, value string) {
				part, err := writer.CreateFormField(key)
				assert.Nil(err)
				io.WriteString(part, value)
			}

			writeField("h", "entry")
			writeField("content", "This is a test")
			part, err := writer.CreateFormFile(key, "whatever.png")
			assert.Nil(err)
			io.WriteString(part, file)

			assert.Nil(writer.Close())

			req, err := http.NewRequest("POST", s.URL, &buf)
			assert.Nil(err)
			req.Header.Set("Content-Type", writer.FormDataContentType())

			resp, err := http.DefaultClient.Do(req)

			assert.Nil(err)
			assert.Equal(http.StatusCreated, resp.StatusCode)
			assert.Equal("http://example.com/blog/p/1", resp.Header.Get("Location"))

			if assert.Len(db.datas, 1) {
				data := db.datas[0]

				assert.Equal("entry", data["h"][0])
				assert.Equal("This is a test", data["content"][0])
				assert.Equal("http://example.com/whatever.png", data[key][0])
			}

			if assert.Len(fw.data, 1) {
				assert.Equal(file, fw.data[0])
			}
		})
	}
}

func TestPostEntryMultipartFormWithMediaMissingScope(t *testing.T) {
	for _, key := range []string{"photo", "video", "audio"} {
		t.Run(key, func(t *testing.T) {
			assert := assert.New(t)
			file := "this is an image"
			db := &fakePostDB{}
			fw := &fakeFileWriter{}

			s := httptest.NewServer(postHandler(db, fw))
			defer s.Close()

			var buf bytes.Buffer
			writer := multipart.NewWriter(&buf)

			writeField := func(key, value string) {
				part, err := writer.CreateFormField(key)
				assert.Nil(err)
				io.WriteString(part, value)
			}

			writeField("h", "entry")
			writeField("content", "This is a test")
			part, err := writer.CreateFormFile(key, "whatever.png")
			assert.Nil(err)
			io.WriteString(part, file)

			assert.Nil(writer.Close())

			req, err := http.NewRequest("POST", s.URL, &buf)
			assert.Nil(err)
			req.Header.Set("Content-Type", writer.FormDataContentType())

			resp, err := http.DefaultClient.Do(req)

			assert.Nil(err)
			assert.Equal(http.StatusUnauthorized, resp.StatusCode)
			assert.Len(db.datas, 0)
			assert.Len(fw.data, 0)
		})
	}
}

func TestPostEntryMultipartFormWithMultiplePhotos(t *testing.T) {
	for _, key := range []string{"photo", "video", "audio"} {
		t.Run(key, func(t *testing.T) {

			assert := assert.New(t)
			db := &fakePostDB{}
			fw := &fakeFileWriter{}

			s := httptest.NewServer(withScope("create", postHandler(db, fw)))
			defer s.Close()

			var buf bytes.Buffer
			writer := multipart.NewWriter(&buf)

			writeField := func(key, value string) {
				part, err := writer.CreateFormField(key)
				assert.Nil(err)
				io.WriteString(part, value)
			}

			writeFile := func(key, name, value string) {
				part, err := writer.CreateFormFile(key, name)
				assert.Nil(err)
				io.WriteString(part, value)
			}

			writeField("h", "entry")
			writeField("content", "This is a test")
			writeFile(key+"[]", "1.jpg", "the first file")
			writeFile(key+"[]", "2.jpg", "the second image")

			assert.Nil(writer.Close())

			req, err := http.NewRequest("POST", s.URL, &buf)
			assert.Nil(err)
			req.Header.Set("Content-Type", writer.FormDataContentType())

			resp, err := http.DefaultClient.Do(req)

			assert.Nil(err)
			assert.Equal(http.StatusCreated, resp.StatusCode)
			assert.Equal("http://example.com/blog/p/1", resp.Header.Get("Location"))

			if assert.Len(db.datas, 1) {
				data := db.datas[0]

				assert.Equal("entry", data["h"][0])
				assert.Equal("This is a test", data["content"][0])
				assert.Equal("http://example.com/1.jpg", data[key][0])
				assert.Equal("http://example.com/2.jpg", data[key][1])
			}

			if assert.Len(fw.data, 2) {
				assert.Equal("the first file", fw.data[0])
				assert.Equal("the second image", fw.data[1])
			}
		})
	}
}

func TestPostEntryWithEmptyValues(t *testing.T) {
	testCases := map[string]func(string) (*http.Response, error){
		"url-encoded-form": func(u string) (*http.Response, error) {
			return http.PostForm(u, url.Values{
				"h":          {"entry"},
				"content":    {"This is a test"},
				"category[]": {""},
			})
		},
		"json": func(u string) (*http.Response, error) {
			return http.Post(u, "application/json", strings.NewReader(`{
  "type": ["h-entry"],
  "properties": {
    "content": ["This is a test"],
    "category": []
  }
}`))
		},
		"multipart-form": func(u string) (*http.Response, error) {
			var buf bytes.Buffer
			writer := multipart.NewWriter(&buf)

			var werr error
			writeField := func(key, value string) {
				part, err := writer.CreateFormField(key)
				if err != nil {
					werr = err
				}
				io.WriteString(part, value)
			}

			writeField("h", "entry")
			writeField("content", "This is a test")
			writeField("category[]", "")
			if err := writer.Close(); err != nil {
				return nil, err
			}
			if werr != nil {
				return nil, werr
			}

			req, err := http.NewRequest("POST", u, &buf)
			if err != nil {
				return nil, err
			}
			req.Header.Set("Content-Type", writer.FormDataContentType())

			return http.DefaultClient.Do(req)
		},
	}

	for name, f := range testCases {
		t.Run(name, func(t *testing.T) {
			assert := assert.New(t)
			blog := &fakePostDB{}

			s := httptest.NewServer(withScope("create", postHandler(blog, nil)))
			defer s.Close()

			resp, err := f(s.URL)

			assert.Nil(err)
			assert.Equal(http.StatusCreated, resp.StatusCode)
			assert.Equal("http://example.com/blog/p/1", resp.Header.Get("Location"))

			if assert.Len(blog.datas, 1) {
				data := blog.datas[0]

				_, ok := data["category"]
				assert.False(ok)
			}
		})
	}
}

func TestUpdateEntry(t *testing.T) {
	assert := assert.New(t)
	db := &fakePostDB{
		adds:       map[string][]map[string][]interface{}{},
		deletes:    map[string][]map[string][]interface{}{},
		replaces:   map[string][]map[string][]interface{}{},
		deleteAlls: map[string][][]string{},
	}

	s := httptest.NewServer(withScope("update", postHandler(db, nil)))
	defer s.Close()

	resp, err := http.Post(s.URL, "application/json", strings.NewReader(`{
  "action": "update",
  "url": "https://example.com/blog/p/100",
  "replace": {
    "content": ["hello moon"]
  },
  "add": {
    "syndication": ["http://somewhere.com"]
  },
  "delete": {
    "not-important": ["this"]
  }
}`))

	assert.Nil(err)
	assert.Equal(http.StatusNoContent, resp.StatusCode)

	replace, ok := db.replaces["https://example.com/blog/p/100"]
	if assert.True(ok) && assert.Len(replace, 1) {
		assert.Equal("hello moon", replace[0]["content"][0])
	}

	add, ok := db.adds["https://example.com/blog/p/100"]
	if assert.True(ok) && assert.Len(add, 1) {
		assert.Equal("http://somewhere.com", add[0]["syndication"][0])
	}

	delete, ok := db.deletes["https://example.com/blog/p/100"]
	if assert.True(ok) && assert.Len(delete, 1) {
		assert.Equal("this", delete[0]["not-important"][0])
	}
}

func TestUpdateEntryMissingScope(t *testing.T) {
	assert := assert.New(t)
	db := &fakePostDB{
		adds:       map[string][]map[string][]interface{}{},
		deletes:    map[string][]map[string][]interface{}{},
		replaces:   map[string][]map[string][]interface{}{},
		deleteAlls: map[string][][]string{},
	}

	s := httptest.NewServer(postHandler(db, nil))
	defer s.Close()

	resp, err := http.Post(s.URL, "application/json", strings.NewReader(`{
  "action": "update",
  "url": "https://example.com/blog/p/100",
  "replace": {
    "content": ["hello moon"]
  },
  "add": {
    "syndication": ["http://somewhere.com"]
  },
  "delete": {
    "not-important": ["this"]
  }
}`))

	assert.Nil(err)
	assert.Equal(http.StatusUnauthorized, resp.StatusCode)

	_, ok := db.replaces["https://example.com/blog/p/100"]
	assert.False(ok)

	_, ok = db.adds["https://example.com/blog/p/100"]
	assert.False(ok)

	_, ok = db.deletes["https://example.com/blog/p/100"]
	assert.False(ok)
}

func TestUpdateEntryDelete(t *testing.T) {
	assert := assert.New(t)
	db := &fakePostDB{
		adds:       map[string][]map[string][]interface{}{},
		deletes:    map[string][]map[string][]interface{}{},
		replaces:   map[string][]map[string][]interface{}{},
		deleteAlls: map[string][][]string{},
	}

	s := httptest.NewServer(withScope("update", postHandler(db, nil)))
	defer s.Close()

	resp, err := http.Post(s.URL, "application/json", strings.NewReader(`{
  "action": "update",
  "url": "https://example.com/blog/p/100",
  "replace": {
    "content": ["hello moon"]
  },
  "add": {
    "syndication": ["http://somewhere.com"]
  },
  "delete": ["not-important"]
}`))

	assert.Nil(err)
	assert.Equal(http.StatusNoContent, resp.StatusCode)

	replace, ok := db.replaces["https://example.com/blog/p/100"]
	if assert.True(ok) && assert.Len(replace, 1) {
		assert.Equal("hello moon", replace[0]["content"][0])
	}

	add, ok := db.adds["https://example.com/blog/p/100"]
	if assert.True(ok) && assert.Len(add, 1) {
		assert.Equal("http://somewhere.com", add[0]["syndication"][0])
	}

	delete, ok := db.deleteAlls["https://example.com/blog/p/100"]
	if assert.True(ok) && assert.Len(delete, 1) {
		assert.Equal("not-important", delete[0][0])
	}
}

func TestUpdateEntryInvalidDelete(t *testing.T) {
	s := httptest.NewServer(withScope("update", postHandler(nil, nil)))
	defer s.Close()

	testCases := map[string]string{
		"array with non-string": `[1]`,
		"map with non-array":    `{"this-key": "and-value"}`,
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			resp, err := http.Post(s.URL, "application/json", strings.NewReader(`{
  "action": "update",
  "url": "https://example.com/blog/p/100",
  "delete": `+tc+`
}`))

			assert.Nil(t, err)
			assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
		})
	}
}

func TestDeleteEntry(t *testing.T) {
	testCases := map[string]func(string) (*http.Response, error){
		"url-encoded-form": func(u string) (*http.Response, error) {
			return http.PostForm(u, url.Values{
				"action": {"delete"},
				"url":    {"https://example.com/blog/p/1"},
			})
		},
		"json": func(u string) (*http.Response, error) {
			return http.Post(u, "application/json", strings.NewReader(`{
  "action": "delete",
  "url": "https://example.com/blog/p/1"
}`))
		},
	}

	for name, f := range testCases {
		t.Run(name, func(t *testing.T) {
			assert := assert.New(t)
			db := &fakePostDB{}

			s := httptest.NewServer(withScope("delete", postHandler(db, nil)))
			defer s.Close()

			resp, err := f(s.URL)

			assert.Nil(err)
			assert.Equal(http.StatusNoContent, resp.StatusCode)

			if assert.Len(db.deleted, 1) {
				assert.Equal("https://example.com/blog/p/1", db.deleted[0])
			}
		})
	}
}

func TestDeleteEntryMissingScope(t *testing.T) {
	testCases := map[string]func(string) (*http.Response, error){
		"url-encoded-form": func(u string) (*http.Response, error) {
			return http.PostForm(u, url.Values{
				"action": {"delete"},
				"url":    {"https://example.com/blog/p/1"},
			})
		},
		"json": func(u string) (*http.Response, error) {
			return http.Post(u, "application/json", strings.NewReader(`{
  "action": "delete",
  "url": "https://example.com/blog/p/1"
}`))
		},
	}

	for name, f := range testCases {
		t.Run(name, func(t *testing.T) {
			assert := assert.New(t)
			db := &fakePostDB{}

			s := httptest.NewServer(postHandler(db, nil))
			defer s.Close()

			resp, err := f(s.URL)

			assert.Nil(err)
			assert.Equal(http.StatusUnauthorized, resp.StatusCode)
			assert.Len(db.deleted, 0)
		})
	}
}

func TestUndeleteEntry(t *testing.T) {
	testCases := map[string]func(string) (*http.Response, error){
		"url-encoded-form": func(u string) (*http.Response, error) {
			return http.PostForm(u, url.Values{
				"action": {"undelete"},
				"url":    {"https://example.com/blog/p/1"},
			})
		},
		"json": func(u string) (*http.Response, error) {
			return http.Post(u, "application/json", strings.NewReader(`{
  "action": "undelete",
  "url": "https://example.com/blog/p/1"
}`))
		},
	}

	for name, f := range testCases {
		t.Run(name, func(t *testing.T) {
			assert := assert.New(t)
			db := &fakePostDB{}

			s := httptest.NewServer(withScope("delete", postHandler(db, nil)))
			defer s.Close()

			resp, err := f(s.URL)

			assert.Nil(err)
			assert.Equal(http.StatusNoContent, resp.StatusCode)

			if assert.Len(db.undeleted, 1) {
				assert.Equal("https://example.com/blog/p/1", db.undeleted[0])
			}
		})
	}
}

func TestUndeleteEntryMissingScope(t *testing.T) {
	testCases := map[string]func(string) (*http.Response, error){
		"url-encoded-form": func(u string) (*http.Response, error) {
			return http.PostForm(u, url.Values{
				"action": {"undelete"},
				"url":    {"https://example.com/blog/p/1"},
			})
		},
		"json": func(u string) (*http.Response, error) {
			return http.Post(u, "application/json", strings.NewReader(`{
  "action": "undelete",
  "url": "https://example.com/blog/p/1"
}`))
		},
	}

	for name, f := range testCases {
		t.Run(name, func(t *testing.T) {
			assert := assert.New(t)
			db := &fakePostDB{}

			s := httptest.NewServer(postHandler(db, nil))
			defer s.Close()

			resp, err := f(s.URL)

			assert.Nil(err)
			assert.Equal(http.StatusUnauthorized, resp.StatusCode)
			assert.Len(db.undeleted, 0)
		})
	}
}
