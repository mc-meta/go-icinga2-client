package icinga2

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"gopkg.in/jmcvetta/napping.v3"
	"net/http"
	"net/url"
	"strings"
	"sync"
)

type Client interface {
	GetHost(string) (Host, error)
	CreateHost(Host) error
	ListHosts() ([]Host, error)
	DeleteHost(string) error
	UpdateHost(Host) error

	GetHostGroup(string) (HostGroup, error)
	CreateHostGroup(HostGroup) error
	ListHostGroups() ([]HostGroup, error)
	DeleteHostGroup(string) error
	UpdateHostGroup(HostGroup) error

	ListDowntimes(string) ([]Downtime, error)

	GetService(string) (Service, error)
	CreateService(Service) error
	ListServices(string) ([]Service, error)
	DeleteService(string) error
	UpdateService(Service) error

	ProcessCheckResult(Service, Action) error
}

type WebClient struct {
	napping           napping.Session
	URL               string
	Username          string
	Password          string
	Debug             bool
	InsecureTLS       bool
	DisableKeepAlives bool
	Zone              string
	RootCAs           *x509.CertPool
}

type MockClient struct {
	Hostgroups map[string]HostGroup
	Hosts      map[string]Host
	Services   map[string]Service
	Actions    map[string][]Action
	mutex      sync.Mutex
}

type Vars map[string]interface{}

type Checkable interface {
	GetCheckCommand() string
	GetVars() Vars
	GetNotes() string
	GetNotesURL() string
}

type Object interface {
	GetVars() Vars
}

func New(s WebClient) (*WebClient, error) {
	rootCAs := s.RootCAs
	if rootCAs == nil {
		rootCAs, _ = x509.SystemCertPool()
	}
	transport := &http.Transport{
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: s.InsecureTLS,
			RootCAs:            rootCAs,
		},
                DisableKeepAlives: s.DisableKeepAlives,
	}
	client := &http.Client{Transport: transport}

	s.napping = napping.Session{
		Log:      s.Debug,
		Client:   client,
		Userinfo: url.UserPassword(s.Username, s.Password),
	}

	s.URL = strings.TrimRight(s.URL, "/")

	return &s, nil
}

func NewMockClient() (c *MockClient) {
	c = new(MockClient)
	c.Hostgroups = make(map[string]HostGroup)
	c.Hosts = make(map[string]Host)
	c.Services = make(map[string]Service)
	c.Actions = make(map[string][]Action)
	c.mutex = sync.Mutex{}
	return
}

type Results struct {
	Results []struct {
		Code   float64  `json:"code"`
		Errors []string `json:"errors,omitempty"`
		Status string   `json:"status,omitempty"`
		Name   string   `json:"name,omitempty"`
		Type   string   `json:"type,omitempty"`
	} `json:"results"`
}

func (s *WebClient) CreateObject(path string, create interface{}) error {
	var results, errmsg Results

	resp, err := s.napping.Put(s.URL+"/v1/objects"+path, create, &results, &errmsg)

	return s.handleResults("create", path, resp, &results, &errmsg, err)
}

func (s *WebClient) UpdateObject(path string, create interface{}) error {
	var results, errmsg Results

	resp, err := s.napping.Post(s.URL+"/v1/objects"+path, create, &results, &errmsg)
	return s.handleResults("update", path, resp, &results, &errmsg, err)
}

func (s *WebClient) handleResults(typ, path string, resp *napping.Response, results, errmsg *Results, oerr error) error {
	var resultReport string

	if oerr != nil {
		return oerr
	}

	for _, r := range results.Results {
		if r.Code >= 400.0 {
			resultReport += r.Status + " " + strings.Join(r.Errors, " ") + " "
		}
	}

	for _, r := range errmsg.Results {
		if r.Code >= 400.0 {
			resultReport += r.Status + " " + strings.Join(r.Errors, " ") + " "
		}
	}

	if resp.HttpResponse().StatusCode >= 400 {
		return fmt.Errorf("%s %s : %s - %s", typ, path, resp.HttpResponse().Status, resultReport)
	}

	if resultReport != "" {
		return fmt.Errorf("%s %s : %s\n", typ, path, resultReport)
	}

	return oerr

}
