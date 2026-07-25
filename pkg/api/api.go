// Copyright (C) 2025 wangyusong
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program. If not, see <https://www.gnu.org/licenses/>.

package api

import (
	"context"
	"encoding/json"
	"io"
	"math/rand/v2"
	"net/http"
	"reflect"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/pkg/errors"

	"github.com/glidea/zenfeed/pkg/component"
	"github.com/glidea/zenfeed/pkg/config"
	"github.com/glidea/zenfeed/pkg/llm"
	"github.com/glidea/zenfeed/pkg/model"
	"github.com/glidea/zenfeed/pkg/storage/feed"
	"github.com/glidea/zenfeed/pkg/storage/feed/block"
	telemetry "github.com/glidea/zenfeed/pkg/telemetry"
	telemetrymodel "github.com/glidea/zenfeed/pkg/telemetry/model"
	jsonschema "github.com/glidea/zenfeed/pkg/util/json_schema"
)

// --- Interface code block ---
type API interface {
	component.Component
	config.Watcher

	QueryAppConfigSchema(
		ctx context.Context,
		req *QueryAppConfigSchemaRequest,
	) (resp *QueryAppConfigSchemaResponse, err error)
	QueryAppConfig(ctx context.Context, req *QueryAppConfigRequest) (resp *QueryAppConfigResponse, err error)
	ApplyAppConfig(ctx context.Context, req *ApplyAppConfigRequest) (resp *ApplyAppConfigResponse, err error)

	QueryRSSHubCategories(
		ctx context.Context,
		req *QueryRSSHubCategoriesRequest,
	) (resp *QueryRSSHubCategoriesResponse, err error)
	QueryRSSHubWebsites(
		ctx context.Context,
		req *QueryRSSHubWebsitesRequest,
	) (resp *QueryRSSHubWebsitesResponse, err error)
	QueryRSSHubRoutes(ctx context.Context, req *QueryRSSHubRoutesRequest) (resp *QueryRSSHubRoutesResponse, err error)

	Write(ctx context.Context, req *WriteRequest) (resp *WriteResponse, err error) // WARN: beta!!!
	Query(ctx context.Context, req *QueryRequest) (resp *QueryResponse, err error)
}

type Config struct {
	RSSHubEndpoint string
	LLM            string
}

func (c *Config) Validate() error {
	c.RSSHubEndpoint = strings.TrimSuffix(c.RSSHubEndpoint, "/")

	return nil
}

func (c *Config) From(app *config.App) *Config {
	c.RSSHubEndpoint = app.Scrape.RSSHubEndpoint
	c.LLM = app.API.LLM

	return c
}

type Dependencies struct {
	ConfigManager config.Manager
	FeedStorage   feed.Storage
	LLMFactory    llm.Factory
}

type QueryAppConfigSchemaRequest struct{}

type QueryAppConfigSchemaResponse map[string]any

type QueryAppConfigRequest struct{}

type QueryAppConfigResponse struct {
	config.App `yaml:",inline" json:",inline"`
}

type ApplyAppConfigRequest struct {
	config.App `yaml:",inline" json:",inline"`
}

type ApplyAppConfigResponse struct{}

type QueryRSSHubCategoriesRequest struct{}

type QueryRSSHubCategoriesResponse struct {
	Categories []string `json:"categories,omitempty"`
}

type QueryRSSHubWebsitesRequest struct {
	Category string `json:"category,omitempty"`
}

type QueryRSSHubWebsitesResponse struct {
	Websites []RSSHubWebsite `json:"websites,omitempty"`
}

type RSSHubWebsite struct {
	ID          string   `json:"id,omitempty"`
	Name        string   `json:"name,omitempty"`
	Description string   `json:"description,omitempty"`
	Categories  []string `json:"categories,omitempty"`
}

type QueryRSSHubRoutesRequest struct {
	WebsiteID string `json:"website_id,omitempty"`
}

type QueryRSSHubRoutesResponse struct {
	Routes []RSSHubRoute `json:"routes,omitempty"`
}

type RSSHubRoute struct {
	Name        string         `json:"name,omitempty"`
	Description string         `json:"description,omitempty"`
	Path        any            `json:"path,omitempty"`
	Example     string         `json:"example,omitempty"`
	Parameters  map[string]any `json:"parameters,omitempty"`
	Features    map[string]any `json:"features,omitempty"`
}

type WriteRequest struct { // Beta.
	Feeds []*model.Feed `json:"feeds"`
}

type WriteResponse struct{} // TODO: data may lost (if crash just now) after Response returned.

type QueryRequest struct {
	Query        string    `json:"query,omitempty"`
	Threshold    float32   `json:"threshold,omitempty"`
	LabelFilters []string  `json:"label_filters,omitempty"`
	Summarize    bool      `json:"summarize,omitempty"`
	Limit        int       `json:"limit,omitempty"`
	Start        time.Time `json:"start,omitempty"`
	End          time.Time `json:"end,omitempty"`
}

func (r *QueryRequest) Validate() error { //nolint:cyclop
	if r.Query != "" && utf8.RuneCountInString(r.Query) > 64 {
		return errors.New("query must be at most 64 characters")
	}
	if r.Threshold == 0 {
		r.Threshold = 0.5
	}
	if r.Threshold < 0 || r.Threshold > 1 {
		return errors.New("threshold must be between 0 and 1")
	}
	if r.Limit < 1 {
		r.Limit = 10
	}
	if r.Limit > 500 {
		r.Limit = 500
	}
	if r.Start.IsZero() {
		r.Start = time.Now().Add(-24 * time.Hour)
	}
	if r.End.IsZero() {
		r.End = time.Now()
	}
	if !r.End.After(r.Start) {
		return errors.New("end must be after start")
	}

	return nil
}

type QueryRequestSemanticFilter struct {
	Query     string  `json:"query,omitempty"`
	Threshold float32 `json:"threshold,omitempty"`
}

type QueryResponse struct {
	Summary string          `json:"summary,omitempty"`
	Feeds   []*block.FeedVO `json:"feeds"`
	Count   int             `json:"count"`
}

type Error struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

func (e Error) Error() string {
	return e.Message
}

func newError(code int, err error) Error {
	return Error{
		Code:    code,
		Message: err.Error(),
	}
}

var (
	ErrBadRequest = func(err error) Error { return newError(http.StatusBadRequest, err) }
	ErrNotFound   = func(err error) Error { return newError(http.StatusNotFound, err) }
	ErrInternal   = func(err error) Error { return newError(http.StatusInternalServerError, err) }
)

// --- Factory code block ---
type Factory component.Factory[API, config.App, Dependencies]

func NewFactory(mockOn ...component.MockOption) Factory {
	if len(mockOn) > 0 {
		return component.FactoryFunc[API, config.App, Dependencies](
			func(instance string, app *config.App, dependencies Dependencies) (API, error) {
				m := &mockAPI{}
				component.MockOptions(mockOn).Apply(&m.Mock)

				return m, nil
			},
		)
	}

	return component.FactoryFunc[API, config.App, Dependencies](new)
}

func new(instance string, app *config.App, dependencies Dependencies) (API, error) {
	config := &Config{}
	config.From(app)
	if err := config.Validate(); err != nil {
		return nil, errors.Wrap(err, "validate config")
	}

	api := &api{
		Base: component.New(&component.BaseConfig[Config, Dependencies]{
			Name:         "API",
			Instance:     instance,
			Config:       config,
			Dependencies: dependencies,
		}),
		hc: &http.Client{},
	}

	return api, nil
}

// --- Implementation code block ---
type api struct {
	*component.Base[Config, Dependencies]

	hc *http.Client
}

func (a *api) Reload(app *config.App) error {
	newConfig := &Config{}
	newConfig.From(app)
	if err := newConfig.Validate(); err != nil {
		return errors.Wrap(err, "validate config")
	}
	a.SetConfig(newConfig)

	return nil
}

func (a *api) QueryAppConfigSchema(
	ctx context.Context,
	req *QueryAppConfigSchemaRequest,
) (resp *QueryAppConfigSchemaResponse, err error) {
	schema, err := jsonschema.ForType(reflect.TypeOf(config.App{}))
	if err != nil {
		return nil, ErrInternal(errors.Wrap(err, "query app config schema"))
	}

	return (*QueryAppConfigSchemaResponse)(&schema), nil
}

func (a *api) QueryAppConfig(
	ctx context.Context,
	req *QueryAppConfigRequest,
) (resp *QueryAppConfigResponse, err error) {
	c := a.Dependencies().ConfigManager.AppConfig()

	return &QueryAppConfigResponse{App: *c}, nil
}

func (a *api) ApplyAppConfig(
	ctx context.Context,
	req *ApplyAppConfigRequest,
) (resp *ApplyAppConfigResponse, err error) {
	if err := a.Dependencies().ConfigManager.SaveAppConfig(&req.App); err != nil {
		return nil, ErrBadRequest(errors.Wrap(err, "save app config"))
	}

	return &ApplyAppConfigResponse{}, nil
}

func (a *api) QueryRSSHubCategories(
	ctx context.Context,
	req *QueryRSSHubCategoriesRequest,
) (resp *QueryRSSHubCategoriesResponse, err error) {
	url := a.Config().RSSHubEndpoint + "/api/namespace"

	// New request.
	forwardReq, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, ErrInternal(errors.Wrap(err, "new request"))
	}

	// Do request.
	forwardRespIO, err := a.hc.Do(forwardReq)
	if err != nil {
		return nil, ErrInternal(errors.Wrap(err, "query rss hub websites"))
	}
	defer func() { _ = forwardRespIO.Body.Close() }()

	// Parse response.
	var forwardResp map[string]RSSHubWebsite
	if err := json.NewDecoder(forwardRespIO.Body).Decode(&forwardResp); err != nil {
		return nil, ErrInternal(errors.Wrap(err, "parse response"))
	}

	// Convert to response.
	categories := make(map[string]struct{}, len(forwardResp))
	for _, website := range forwardResp {
		for _, category := range website.Categories {
			categories[category] = struct{}{}
		}
	}
	result := make([]string, 0, len(categories))
	for category := range categories {
		result = append(result, category)
	}
	resp = &QueryRSSHubCategoriesResponse{Categories: result}

	return resp, nil
}

func (a *api) QueryRSSHubWebsites(
	ctx context.Context, req *QueryRSSHubWebsitesRequest,
) (resp *QueryRSSHubWebsitesResponse, err error) {
	if req.Category == "" {
		return nil, ErrBadRequest(errors.New("category is required"))
	}

	url := a.Config().RSSHubEndpoint + "/api/category/" + req.Category

	// New request.
	forwardReq, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, ErrInternal(errors.Wrap(err, "new request"))
	}

	// Do request.
	forwardRespIO, err := a.hc.Do(forwardReq)
	if err != nil {
		return nil, ErrInternal(errors.Wrap(err, "query rss hub routes"))
	}
	defer func() { _ = forwardRespIO.Body.Close() }()

	// Parse response.
	body, err := io.ReadAll(forwardRespIO.Body)
	if err != nil {
		return nil, ErrInternal(errors.Wrap(err, "read response"))
	}
	if len(body) == 0 {
		// Hack for RSSHub...
		// Consider cache category ids for validate by self to remove this shit code.
		return nil, ErrBadRequest(errors.New("category id is invalid"))
	}
	var forwardResp map[string]RSSHubWebsite
	if err := json.Unmarshal(body, &forwardResp); err != nil {
		return nil, ErrInternal(errors.Wrap(err, "parse response"))
	}

	// Convert to response.
	resp = &QueryRSSHubWebsitesResponse{Websites: make([]RSSHubWebsite, 0, len(forwardResp))}
	for id, website := range forwardResp {
		website.ID = id
		website.Description = website.Name + " - " + website.Description
		website.Name = "" // Avoid AI confusion of ID and Name.
		resp.Websites = append(resp.Websites, website)
	}

	return resp, nil
}

func (a *api) QueryRSSHubRoutes(
	ctx context.Context,
	req *QueryRSSHubRoutesRequest,
) (resp *QueryRSSHubRoutesResponse, err error) {
	if req.WebsiteID == "" {
		return nil, ErrBadRequest(errors.New("website id is required"))
	}

	url := a.Config().RSSHubEndpoint + "/api/namespace/" + req.WebsiteID

	// New request.
	forwardReq, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, ErrInternal(errors.Wrap(err, "new request"))
	}

	// Do request.
	forwardRespIO, err := a.hc.Do(forwardReq)
	if err != nil {
		return nil, ErrInternal(errors.Wrap(err, "query rss hub routes"))
	}
	defer func() { _ = forwardRespIO.Body.Close() }()

	// Parse response.
	body, err := io.ReadAll(forwardRespIO.Body)
	if err != nil {
		return nil, ErrInternal(errors.Wrap(err, "read response"))
	}
	if len(body) == 0 {
		return nil, ErrBadRequest(errors.New("website id is invalid"))
	}

	var forwardResp struct {
		Routes map[string]RSSHubRoute `json:"routes"`
	}
	if err := json.Unmarshal(body, &forwardResp); err != nil {
		return nil, ErrInternal(errors.Wrap(err, "parse response"))
	}

	// Convert to response.
	resp = &QueryRSSHubRoutesResponse{Routes: make([]RSSHubRoute, 0, len(forwardResp.Routes))}
	for _, route := range forwardResp.Routes {
		resp.Routes = append(resp.Routes, route)
	}

	return resp, nil
}

func (a *api) Write(ctx context.Context, req *WriteRequest) (resp *WriteResponse, err error) {
	ctx = telemetry.StartWith(ctx, append(a.TelemetryLabels(), telemetrymodel.KeyOperation, "Write")...)
	defer func() { telemetry.End(ctx, err) }()

	for _, feed := range req.Feeds {
		feed.ID = rand.Uint64()
		feed.Labels.Put(model.LabelType, "api", false)
	}
	if err := a.Dependencies().FeedStorage.Append(ctx, req.Feeds...); err != nil {
		return nil, ErrInternal(errors.Wrap(err, "append"))
	}

	return &WriteResponse{}, nil
}

func (a *api) Query(ctx context.Context, req *QueryRequest) (resp *QueryResponse, err error) {
	ctx = telemetry.StartWith(ctx, append(a.TelemetryLabels(), telemetrymodel.KeyOperation, "Query")...)
	defer func() { telemetry.End(ctx, err) }()

	// Validate request.
	if err := req.Validate(); err != nil {
		return nil, ErrBadRequest(errors.Wrap(err, "validate"))
	}

	// Forward to storage.
	feeds, err := a.Dependencies().FeedStorage.Query(ctx, block.QueryOptions{
		Query:        req.Query,
		Threshold:    req.Threshold,
		LabelFilters: req.LabelFilters,
		Limit:        req.Limit,
		Start:        req.Start,
		End:          req.End,
	})
	if err != nil {
		return nil, ErrInternal(errors.Wrap(err, "query"))
	}
	if len(feeds) == 0 {
		return &QueryResponse{Feeds: []*block.FeedVO{}}, nil
	}

	// Summarize feeds.
	var summary string
	if req.Summarize {
		var sb strings.Builder
		for _, feed := range feeds {
			sb.WriteString(feed.Labels.Get(model.LabelContent) + "\n")
		}

		q := []string{
			"You are a helpful assistant that summarizes the following feeds.",
			sb.String(),
		}
		if req.Query != "" {
			q = append(q, "And my specific question & requirements are: "+req.Query)
			q = append(q, "Respond in query's original language.")
		}

		summary, err = a.Dependencies().LLMFactory.Get(a.Config().LLM).String(ctx, q)
		if err != nil {
			summary = err.Error()
		}
	}

	// Convert to response.
	for _, feed := range feeds {
		feed.Time = feed.Time.In(time.Local)
	}

	return &QueryResponse{
		Summary: summary,
		Feeds:   feeds,
		Count:   len(feeds),
	}, nil
}

type mockAPI struct {
	component.Mock
}

func (m *mockAPI) Reload(app *config.App) error {
	return m.Called(app).Error(0)
}

func (m *mockAPI) QueryAppConfigSchema(
	ctx context.Context,
	req *QueryAppConfigSchemaRequest,
) (resp *QueryAppConfigSchemaResponse, err error) {
	args := m.Called(ctx, req)

	return args.Get(0).(*QueryAppConfigSchemaResponse), args.Error(1)
}

func (m *mockAPI) QueryAppConfig(
	ctx context.Context,
	req *QueryAppConfigRequest,
) (resp *QueryAppConfigResponse, err error) {
	args := m.Called(ctx, req)

	return args.Get(0).(*QueryAppConfigResponse), args.Error(1)
}

func (m *mockAPI) ApplyAppConfig(
	ctx context.Context,
	req *ApplyAppConfigRequest,
) (resp *ApplyAppConfigResponse, err error) {
	args := m.Called(ctx, req)

	return args.Get(0).(*ApplyAppConfigResponse), args.Error(1)
}

func (m *mockAPI) QueryRSSHubCategories(
	ctx context.Context,
	req *QueryRSSHubCategoriesRequest,
) (resp *QueryRSSHubCategoriesResponse, err error) {
	args := m.Called(ctx, req)

	return args.Get(0).(*QueryRSSHubCategoriesResponse), args.Error(1)
}

func (m *mockAPI) QueryRSSHubWebsites(
	ctx context.Context,
	req *QueryRSSHubWebsitesRequest,
) (resp *QueryRSSHubWebsitesResponse, err error) {
	args := m.Called(ctx, req)

	return args.Get(0).(*QueryRSSHubWebsitesResponse), args.Error(1)
}

func (m *mockAPI) QueryRSSHubRoutes(
	ctx context.Context,
	req *QueryRSSHubRoutesRequest,
) (resp *QueryRSSHubRoutesResponse, err error) {
	args := m.Called(ctx, req)

	return args.Get(0).(*QueryRSSHubRoutesResponse), args.Error(1)
}

func (m *mockAPI) Query(ctx context.Context, req *QueryRequest) (resp *QueryResponse, err error) {
	args := m.Called(ctx, req)

	return args.Get(0).(*QueryResponse), args.Error(1)
}

func (m *mockAPI) Write(ctx context.Context, req *WriteRequest) (resp *WriteResponse, err error) {
	args := m.Called(ctx, req)

	return args.Get(0).(*WriteResponse), args.Error(1)
}
