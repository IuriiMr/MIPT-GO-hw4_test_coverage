package main

// код писать тут

import (
	"encoding/json"
	"encoding/xml"
	_ "fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"os"
	"reflect"
	"strconv"
	"strings"
	"testing"
	"time"
)

type Dataset struct {
	XMLName xml.Name `xml:"root"`
	Users   []Row    `xml:"row"`
}

type Row struct {
	Id     int    `xml:"id"`
	Name   string `xml:"first_name"`
	Age    int    `xml:"age"`
	About  string `xml:"about"`
	Gender string `xml:"gender"`
}

func (r Row) convert() User {
	return User{
		Id:     r.Id,
		Name:   r.Name,
		Age:    r.Age,
		About:  r.About,
		Gender: r.Gender,
	}
}

var dataset Dataset

const (
	accessToken = "Test"
)

func init() {
	file, err := os.Open("dataset.xml")
	if err != nil {
		panic(err)
	}

	fileContents, err := ioutil.ReadAll(file)
	if err != nil {
		panic(err)
	}

	err = xml.Unmarshal([]byte(fileContents), &dataset)
	if err != nil {
		panic(err)
	}
}

func SearchServer(w http.ResponseWriter, r *http.Request) {
	switch r.Header.Get("AccessToken") {
	case "json":
		w.WriteHeader(http.StatusOK)
		io.WriteString(w, `{]`)
		return
	case "internal":
		w.WriteHeader(http.StatusInternalServerError)
		return
	case "request":
		w.WriteHeader(http.StatusBadRequest)
		return
	case "requestBadOrder":
		w.WriteHeader(http.StatusBadRequest)
		io.WriteString(w, `{"Error":"ErrorBadOrderField"}`)
		return
	case "requestBadOrderUnknown":
		w.WriteHeader(http.StatusBadRequest)
		io.WriteString(w, `{"Error":"ErrorBadOrderUnknown"}`)
		return
	case "timeout":
		w.WriteHeader(http.StatusFound)
		time.Sleep(1500 * time.Millisecond)
		return
	case accessToken:
	default:
		w.WriteHeader(http.StatusUnauthorized)
		return
	}

	limit, _ := strconv.Atoi(r.FormValue("limit"))
	offset, _ := strconv.Atoi(r.FormValue("offset"))

	w.WriteHeader(http.StatusOK)

	var users []string
	if limit > 25 {
		limit = 25
	}
	if offset+limit > len(dataset.Users) {
		limit = len(dataset.Users)
	}
	for i := offset; i < limit; i++ {
		user := dataset.Users[i].convert()
		u, err := json.Marshal(user)
		if err != nil {
			panic(err)
		}
		users = append(users, string(u))
	}

	io.WriteString(w, `[`+strings.Join(users, ",")+`]`)
}

func TestFindUsers(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(SearchServer))
	defer ts.Close()

	var users []User
	for i := 30; i < len(dataset.Users); i++ {
		users = append(users, dataset.Users[i].convert())
	}

	tests := map[string]struct {
		client *SearchClient
		req    SearchRequest
		resp   *SearchResponse
		err    bool
	}{
		"ok": {
			client: &SearchClient{URL: ts.URL, AccessToken: accessToken},
			req:    SearchRequest{Limit: 1, Offset: 0},
			resp:   &SearchResponse{Users: []User{dataset.Users[0].convert()}, NextPage: true},
			err:    false,
		},
		"limit > 25": {
			client: &SearchClient{URL: ts.URL, AccessToken: accessToken},
			req:    SearchRequest{Offset: 30, Limit: 26},
			resp:   &SearchResponse{Users: users, NextPage: false},
			err:    false,
		},
		"limit < 0": {
			client: &SearchClient{URL: ts.URL, AccessToken: accessToken},
			req:    SearchRequest{Limit: -1},
			err:    true,
		},
		"offset < 0": {
			client: &SearchClient{URL: ts.URL, AccessToken: accessToken},
			req:    SearchRequest{Offset: -1},
			err:    true,
		},
		"wrong access token": {
			client: &SearchClient{URL: ts.URL, AccessToken: "wrong"},
			err:    true,
		},
		"wrong returning json": {
			client: &SearchClient{URL: ts.URL, AccessToken: "json"},
			req:    SearchRequest{Limit: 1, Offset: 0},
			err:    true,
		},
		"internal error": {
			client: &SearchClient{URL: ts.URL, AccessToken: "internal"},
			req:    SearchRequest{Limit: 1, Offset: 0},
			err:    true,
		},
		"bad request": {
			client: &SearchClient{URL: ts.URL, AccessToken: "request"},
			req:    SearchRequest{Limit: 1, Offset: 0},
			err:    true,
		},
		"bad order": {
			client: &SearchClient{URL: ts.URL, AccessToken: "requestBadOrder"},
			req:    SearchRequest{Limit: 1, Offset: 0},
			err:    true,
		},
		"bad order unknonw": {
			client: &SearchClient{URL: ts.URL, AccessToken: "requestBadOrderUnknown"},
			req:    SearchRequest{Limit: 1, Offset: 0},
			err:    true,
		},
		"timeout": {
			client: &SearchClient{URL: ts.URL, AccessToken: "timeout"},
			req:    SearchRequest{Limit: 1, Offset: 0},
			err:    true,
		},
		"invalid url": {
			client: &SearchClient{URL: "", AccessToken: accessToken},
			req:    SearchRequest{Limit: 1, Offset: 0},
			err:    true,
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			resp, err := tt.client.FindUsers(tt.req)
			if tt.err && err == nil {
				t.Fatalf("error got=%v want=%v", err, tt.err)
			}
			if tt.resp != nil && !reflect.DeepEqual(resp, tt.resp) {
				t.Fatalf("response got=%#v want=%#v", resp, tt.resp)
			}
		})
	}
}
