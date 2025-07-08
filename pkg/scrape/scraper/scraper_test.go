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

package scraper

import (
	"testing"
	"time"

	. "github.com/onsi/gomega"

	"github.com/glidea/zenfeed/pkg/test"
	timeutil "github.com/glidea/zenfeed/pkg/util/time"
)

func TestConfig_Validate(t *testing.T) {
	RegisterTestingT(t)

	// --- Test types ---
	type givenDetail struct {
		config *Config
	}
	type whenDetail struct{} // Validation is the action
	type thenExpected struct {
		expectedConfig *Config // Expected state after validation
		isErr          bool
		wantErrMsg     string
	}

	// --- Test cases ---
	tests := []test.Case[givenDetail, whenDetail, thenExpected]{
		{
			Scenario: "Default values",
			Given:    "a config with zero values for Past and Interval and non-empty Name",
			When:     "validating the config",
			Then:     "should set default Past and Interval, and no error",
			GivenDetail: givenDetail{
				config: &Config{Name: "test"}, // Name is required
			},
			WhenDetail: whenDetail{},
			ThenExpected: thenExpected{
				expectedConfig: &Config{
					Name:     "test",
					Past:     timeutil.Day, // Default Past
					Interval: time.Hour,    // Default/Minimum Interval
				},
				isErr: false,
			},
		},
		{
			Scenario: "Past exceeds maximum",
			Given:    "a config with Past exceeding the maximum limit",
			When:     "validating the config",
			Then:     "should cap Past to the maximum value",
			GivenDetail: givenDetail{
				config: &Config{Name: "test", Past: maxPast + time.Hour},
			},
			WhenDetail: whenDetail{},
			ThenExpected: thenExpected{
				expectedConfig: &Config{
					Name:     "test",
					Past:     maxPast,   // Capped Past
					Interval: time.Hour, // Default Interval
				},
				isErr: false,
			},
		},
		{
			Scenario: "Interval below minimum",
			Given:    "a config with Interval below the minimum limit",
			When:     "validating the config",
			Then:     "should set Interval to the minimum value",
			GivenDetail: givenDetail{
				config: &Config{Name: "test", Interval: time.Second},
			},
			WhenDetail: whenDetail{},
			ThenExpected: thenExpected{
				expectedConfig: &Config{
					Name:     "test",
					Past:     timeutil.Day,     // Default Past
					Interval: 10 * time.Minute, // Minimum Interval
				},
				isErr: false,
			},
		},
		{
			Scenario: "Valid values",
			Given:    "a config with valid Past and Interval",
			When:     "validating the config",
			Then:     "should keep the original values",
			GivenDetail: givenDetail{
				config: &Config{
					Name:     "test",
					Past:     4 * time.Hour,
					Interval: 30 * time.Minute,
				},
			},
			WhenDetail: whenDetail{},
			ThenExpected: thenExpected{
				expectedConfig: &Config{
					Name:     "test",
					Past:     4 * time.Hour,
					Interval: 30 * time.Minute,
				},
				isErr: false,
			},
		},
		{
			Scenario: "Missing Name",
			Given:    "a config with an empty Name",
			When:     "validating the config",
			Then:     "should return an error",
			GivenDetail: givenDetail{
				config: &Config{}, // Empty Name
			},
			WhenDetail: whenDetail{},
			ThenExpected: thenExpected{
				isErr:      true,
				wantErrMsg: "name cannot be empty",
			},
		},
	}

	// --- Run tests ---
	for _, tt := range tests {
		t.Run(tt.Scenario, func(t *testing.T) {
			// --- Given ---
			config := tt.GivenDetail.config // Use the config from the test case

			// --- When ---
			err := config.Validate()

			// --- Then ---
			if tt.ThenExpected.isErr {
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring(tt.ThenExpected.wantErrMsg))
			} else {
				Expect(err).NotTo(HaveOccurred())
				// Compare the validated config with the expected one
				Expect(config).To(Equal(tt.ThenExpected.expectedConfig))
			}
		})
	}
}

func TestNew(t *testing.T) {
	RegisterTestingT(t)

	// --- Test types ---
	type givenDetail struct {
		instance     string
		config       *Config
		dependencies Dependencies // Keep dependencies empty for now, focus on config validation
	}
	type whenDetail struct{} // Creation is the action
	type thenExpected struct {
		isErr        bool
		wantErrMsg   string
		validateFunc func(t *testing.T, s Scraper) // Optional validation
	}

	// --- Test cases ---
	validRSSConfig := &ScrapeSourceRSS{URL: "http://valid.com/feed"}
	validBaseConfig := &Config{
		Name:     "test-scraper",
		Interval: 15 * time.Minute, // Valid interval
		RSS:      validRSSConfig,   // Need a valid source for newReader
	}

	tests := []test.Case[givenDetail, whenDetail, thenExpected]{
		{
			Scenario: "Valid Configuration",
			Given:    "a valid config and dependencies",
			When:     "creating a new scraper",
			Then:     "should create scraper successfully",
			GivenDetail: givenDetail{
				instance:     "scraper-1",
				config:       validBaseConfig,
				dependencies: Dependencies{}, // Empty deps are okay for New itself
			},
			WhenDetail: whenDetail{},
			ThenExpected: thenExpected{
				isErr: false,
				validateFunc: func(t *testing.T, s Scraper) {
					Expect(s).NotTo(BeNil())
					Expect(s.Name()).To(Equal("Scraper")) // From Base component
					Expect(s.Instance()).To(Equal("scraper-1"))
					Expect(s.Config()).To(Equal(validBaseConfig)) // Check if config is stored

					// Check internal state if needed (e.g., source type)
					concreteScraper, ok := s.(*scraper)
					Expect(ok).To(BeTrue())
					Expect(concreteScraper.source).NotTo(BeNil())
					_, isRSSReader := concreteScraper.source.(*rssReader)
					Expect(isRSSReader).To(BeTrue())
				},
			},
		},
		{
			Scenario: "Invalid Configuration - Validation Fail",
			Given:    "a config that fails validation (e.g., missing name)",
			When:     "creating a new scraper",
			Then:     "should return a validation error",
			GivenDetail: givenDetail{
				instance: "scraper-invalid",
				config: &Config{ // Missing Name, invalid interval
					Interval: time.Second,
					RSS:      validRSSConfig,
				},
				dependencies: Dependencies{},
			},
			WhenDetail: whenDetail{},
			ThenExpected: thenExpected{
				isErr:      true,
				wantErrMsg: "invalid scraper config: name cannot be empty", // Specific validation error
			},
		},
		{
			Scenario: "Invalid Configuration - Source Creation Fail",
			Given:    "a config that passes validation but has invalid source details",
			When:     "creating a new scraper",
			Then:     "should return an error from source creation",
			GivenDetail: givenDetail{
				instance: "scraper-bad-source",
				config: &Config{
					Name:     "test-bad-source",
					Interval: 15 * time.Minute,
					RSS:      &ScrapeSourceRSS{URL: "invalid-url-format"}, // Invalid RSS URL
				},
				dependencies: Dependencies{},
			},
			WhenDetail: whenDetail{},
			ThenExpected: thenExpected{
				isErr:      true,
				wantErrMsg: "invalid RSS config: URL must be a valid HTTP/HTTPS URL", // Error from newRSSReader via newReader
			},
		},
		{
			Scenario: "Invalid Configuration - No Source Configured",
			Given:    "a config that passes validation but lacks any source config (RSS is nil)",
			When:     "creating a new scraper",
			Then:     "should return an error indicating unsupported source",
			GivenDetail: givenDetail{
				instance: "scraper-no-source",
				config: &Config{
					Name:     "test-no-source",
					Interval: 15 * time.Minute,
					RSS:      nil, // No source configured
				},
				dependencies: Dependencies{},
			},
			WhenDetail: whenDetail{},
			ThenExpected: thenExpected{
				isErr:      true,
				wantErrMsg: "source not supported", // Error from newReader
			},
		},
	}

	// --- Run tests ---
	factory := NewFactory() // Use the real factory
	for _, tt := range tests {
		t.Run(tt.Scenario, func(t *testing.T) {
			// --- Given & When ---
			s, err := factory.New(tt.GivenDetail.instance, tt.GivenDetail.config, tt.GivenDetail.dependencies)

			// --- Then ---
			if tt.ThenExpected.isErr {
				Expect(err).To(HaveOccurred())
				// Use MatchError for wrapped errors if necessary, but ContainSubstring is often sufficient
				Expect(err.Error()).To(ContainSubstring(tt.ThenExpected.wantErrMsg))
				Expect(s).To(BeNil())
			} else {
				Expect(err).NotTo(HaveOccurred())
				Expect(s).NotTo(BeNil())
				if tt.ThenExpected.validateFunc != nil {
					tt.ThenExpected.validateFunc(t, s)
				}
			}
		})
	}
}
