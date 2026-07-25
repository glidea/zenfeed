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

package time

import (
	"context"
	"encoding/json"
	"math/rand/v2"
	"time"
	_ "time/tzdata"

	"github.com/pkg/errors"
	"gopkg.in/yaml.v3"

	runtimeutil "github.com/glidea/zenfeed/pkg/util/runtime"
)

const (
	Day   = 24 * time.Hour
	Week  = 7 * Day
	Month = 30 * Day
	Year  = 365 * Day
)

// SetLocation sets the location for the current application.
func SetLocation(name string) error {
	if name == "" {
		return nil
	}

	loc, err := time.LoadLocation(name)
	if err != nil {
		return errors.Wrap(err, "load location")
	}

	time.Local = loc

	return nil
}

func InRange(t time.Time, start, end time.Time) bool {
	return t.After(start) && t.Before(end)
}

func Format(t time.Time) string {
	return t.Format(time.RFC3339)
}

func Parse(s string) (time.Time, error) {
	return time.Parse(time.RFC3339, s)
}

func MustParse(s string) time.Time {
	return runtimeutil.Must1(Parse(s))
}

func Tick(ctx context.Context, d time.Duration, f func() error) error {
	ticker := time.NewTicker(d)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			if err := f(); err != nil {
				return err
			}
		case <-ctx.Done():
			return nil
		}
	}
}

func Random(max time.Duration) time.Duration {
	return time.Duration(rand.Int63n(int64(max)))
}

type Duration time.Duration

func (d Duration) String() string {
	return time.Duration(d).String()
}

func (d Duration) MarshalJSON() ([]byte, error) {
	return json.Marshal(d.String())
}

func (d *Duration) UnmarshalJSON(b []byte) error {
	var v any
	if err := json.Unmarshal(b, &v); err != nil {
		return err
	}

	switch tv := v.(type) {
	case float64:
		*d = Duration(time.Duration(tv))

		return nil

	case string:
		parsed, err := time.ParseDuration(tv)
		if err != nil {
			return err
		}
		*d = Duration(parsed)

		return nil

	default:
		return errors.Errorf("invalid duration: %v", tv)
	}
}

func (d Duration) MarshalYAML() (interface{}, error) {
	return d.String(), nil
}

func (d *Duration) UnmarshalYAML(value *yaml.Node) error {
	if value.Kind != yaml.ScalarNode {
		return errors.Errorf("invalid duration: expected a scalar node, got %v", value.Kind)
	}

	s := value.Value

	parsed, err := time.ParseDuration(s)
	if err != nil {
		return errors.Errorf("failed to parse duration string '%s' from YAML: %s", s, err.Error())
	}

	*d = Duration(parsed)

	return nil
}
