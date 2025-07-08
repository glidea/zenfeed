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

package object

import (
	"context"
	"io"
	"net/url"
	"reflect"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
	"github.com/pkg/errors"

	"github.com/glidea/zenfeed/pkg/component"
	"github.com/glidea/zenfeed/pkg/config"
	"github.com/glidea/zenfeed/pkg/telemetry"
	"github.com/glidea/zenfeed/pkg/telemetry/log"
	telemetrymodel "github.com/glidea/zenfeed/pkg/telemetry/model"
)

// --- Interface code block ---
type Storage interface {
	component.Component
	config.Watcher
	Put(ctx context.Context, key string, body io.Reader, contentType string) (url string, err error)
	Get(ctx context.Context, key string) (url string, err error)
}

var ErrNotFound = errors.New("not found")

type Config struct {
	Endpoint        string
	AccessKeyID     string
	SecretAccessKey string
	Bucket          string
	BucketURL       string
}

func (c *Config) Validate() error {
	if c.Endpoint == "" {
		return errors.New("endpoint is required")
	}
	if c.AccessKeyID == "" {
		return errors.New("access key id is required")
	}
	if c.SecretAccessKey == "" {
		return errors.New("secret access key is required")
	}
	if c.Bucket == "" {
		return errors.New("bucket is required")
	}
	if c.BucketURL == "" {
		return errors.New("bucket url is required")
	}

	return nil
}

func (c *Config) From(app *config.App) *Config {
	*c = Config{
		Endpoint:        app.Storage.Object.Endpoint,
		AccessKeyID:     app.Storage.Object.AccessKeyID,
		SecretAccessKey: app.Storage.Object.SecretAccessKey,
		Bucket:          app.Storage.Object.Bucket,
		BucketURL:       app.Storage.Object.BucketURL,
	}

	return c
}

type Dependencies struct{}

// --- Factory code block ---
type Factory component.Factory[Storage, config.App, Dependencies]

func NewFactory(mockOn ...component.MockOption) Factory {
	if len(mockOn) > 0 {
		return component.FactoryFunc[Storage, config.App, Dependencies](
			func(instance string, config *config.App, dependencies Dependencies) (Storage, error) {
				m := &mockStorage{}
				component.MockOptions(mockOn).Apply(&m.Mock)

				return m, nil
			},
		)
	}

	return component.FactoryFunc[Storage, config.App, Dependencies](new)
}

func new(instance string, app *config.App, dependencies Dependencies) (Storage, error) {
	config := &Config{}
	config.From(app)
	if err := config.Validate(); err != nil {
		return nil, errors.Wrap(err, "validate config")
	}

	client, err := minio.New(config.Endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(config.AccessKeyID, config.SecretAccessKey, ""),
		Secure: true,
	})
	if err != nil {
		return nil, errors.Wrap(err, "new minio client")
	}

	u, err := url.Parse(config.BucketURL)
	if err != nil {
		return nil, errors.Wrap(err, "parse public url")
	}

	return &s3{
		Base: component.New(&component.BaseConfig[Config, Dependencies]{
			Name:         "ObjectStorage",
			Instance:     instance,
			Config:       config,
			Dependencies: dependencies,
		}),
		client:    client,
		bucketURL: u,
	}, nil
}

// --- Implementation code block ---
type s3 struct {
	*component.Base[Config, Dependencies]

	client    *minio.Client
	bucketURL *url.URL
}

func (s *s3) Put(ctx context.Context, key string, body io.Reader, contentType string) (publicURL string, err error) {
	ctx = telemetry.StartWith(ctx, append(s.TelemetryLabels(), telemetrymodel.KeyOperation, "Put")...)
	defer func() { telemetry.End(ctx, err) }()
	bucket := s.Config().Bucket

	if _, err := s.client.PutObject(ctx, bucket, key, body, -1, minio.PutObjectOptions{
		ContentType: contentType,
	}); err != nil {
		return "", errors.Wrap(err, "put object")
	}

	return s.bucketURL.JoinPath(key).String(), nil
}

func (s *s3) Get(ctx context.Context, key string) (publicURL string, err error) {
	ctx = telemetry.StartWith(ctx, append(s.TelemetryLabels(), telemetrymodel.KeyOperation, "Get")...)
	defer func() { telemetry.End(ctx, err) }()
	bucket := s.Config().Bucket

	if _, err := s.client.StatObject(ctx, bucket, key, minio.StatObjectOptions{}); err != nil {
		errResponse := minio.ToErrorResponse(err)
		if errResponse.Code == minio.NoSuchKey {
			return "", ErrNotFound
		}

		return "", errors.Wrap(err, "stat object")
	}

	return s.bucketURL.JoinPath(key).String(), nil
}

func (s *s3) Reload(app *config.App) (err error) {
	ctx := telemetry.StartWith(s.Context(), append(s.TelemetryLabels(), telemetrymodel.KeyOperation, "Reload")...)
	defer func() { telemetry.End(ctx, err) }()

	newConfig := &Config{}
	newConfig.From(app)
	if err := newConfig.Validate(); err != nil {
		return errors.Wrap(err, "validate config")
	}

	if reflect.DeepEqual(s.Config(), newConfig) {
		log.Debug(ctx, "object storage config not changed")

		return nil
	}

	client, err := minio.New(newConfig.Endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(newConfig.AccessKeyID, newConfig.SecretAccessKey, ""),
		Secure: true,
	})
	if err != nil {
		return errors.Wrap(err, "new minio client")
	}

	u, err := url.Parse(newConfig.BucketURL)
	if err != nil {
		return errors.Wrap(err, "parse public url")
	}

	s.client = client
	s.bucketURL = u
	s.SetConfig(newConfig)

	log.Info(ctx, "object storage reloaded")

	return nil
}

// --- Mock code block ---
type mockStorage struct {
	component.Mock
}

func (m *mockStorage) Put(ctx context.Context, key string, body io.Reader, contentType string) (string, error) {
	args := m.Called(ctx, key, body, contentType)

	return args.String(0), args.Error(1)
}

func (m *mockStorage) Get(ctx context.Context, key string) (string, error) {
	args := m.Called(ctx, key)

	return args.String(0), args.Error(1)
}

func (m *mockStorage) Reload(app *config.App) error {
	args := m.Called(app)

	return args.Error(0)
}
