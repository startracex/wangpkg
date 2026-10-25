package search

import (
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/startracex/wangpkg/pkg/httputil"
	"github.com/startracex/wangpkg/pkg/npmini"
)

type Response struct {
	Objects []Object  `json:"objects"`
	Total   int       `json:"total"`
	Time    time.Time `json:"time"`
}

type Object struct {
	Downloads  map[string]int `json:"downloads"`
	Dependents int            `json:"dependents"`
	Updated    time.Time      `json:"updated"`
	Package    Package        `json:"package"`
}

type Package struct {
	Date        time.Time         `json:"date"`
	Name        string            `json:"name"`
	Version     string            `json:"version"`
	Description string            `json:"description"`
	Keywords    []string          `json:"keywords"`
	Links       map[string]string `json:"links"`
	Publisher   People            `json:"publisher"`
	Maintainers []People          `json:"maintainers"`
}

type People struct {
	Username string `json:"username"`
	Email    string `json:"email"`
}

func (p People) String() string {
	if p.Email != "" {
		return p.Username + " <" + p.Email + ">"
	}
	return p.Username
}

type Parameters struct {
	Texts []string
	Size  int
	From  int
}

// Search queries the search endpoint of the default registry of ni.
func Search(p Parameters, ni *npmini.NpmINI, rt http.RoundTripper) (*Response, error) {
	reg, err := ni.GetRegistry("")
	if err != nil {
		return nil, fmt.Errorf("resolve registry: %w", err)
	}
	u := reg.JoinPath("/-/v1/search")
	u.RawQuery = p.Query()
	req, err := http.NewRequest(http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, fmt.Errorf("build search request: %w", err)
	}
	return httputil.DecodeJSON[*Response](rt.RoundTrip(req))
}

func (p Parameters) Query() string {
	query := make(url.Values)
	query.Set("text", strings.Join(p.Texts, " "))
	if p.Size > 0 {
		query.Set("size", strconv.Itoa(p.Size))
	}
	if p.From > 0 {
		query.Set("from", strconv.Itoa(p.From))
	}
	return query.Encode()
}
