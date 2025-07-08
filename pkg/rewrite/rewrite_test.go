package rewrite

import (
	"context"
	"io"
	"strings"
	"testing"

	. "github.com/onsi/gomega"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/mock"
	"k8s.io/utils/ptr"

	"github.com/glidea/zenfeed/pkg/component"
	"github.com/glidea/zenfeed/pkg/llm"
	"github.com/glidea/zenfeed/pkg/model"
	"github.com/glidea/zenfeed/pkg/storage/object"
	"github.com/glidea/zenfeed/pkg/test"
)

func TestLabels(t *testing.T) {
	RegisterTestingT(t)

	type givenDetail struct {
		config            *Config
		llmMock           func(m *mock.Mock)
		objectStorageMock func(m *mock.Mock)
	}
	type whenDetail struct {
		inputLabels model.Labels
	}
	type thenExpected struct {
		outputLabels model.Labels
		err          error
		isErr        bool
	}

	tests := []test.Case[givenDetail, whenDetail, thenExpected]{
		{
			Scenario: "Drop feed based on transformed content match",
			Given:    "a rule to drop feed if transformed content matches 'spam'",
			When:     "processing labels where transformed content is 'spam'",
			Then:     "should return nil labels indicating drop",
			GivenDetail: givenDetail{
				config: &Config{
					{
						SourceLabel:           model.LabelContent,
						SkipTooShortThreshold: ptr.To(10),
						Transform: &Transform{
							ToText: &ToText{
								Type:   ToTextTypePrompt,
								LLM:    "mock-llm",
								Prompt: "{{ .category }}", // Using a simple template for testing
							},
						},
						Match:  "spam",
						Action: ActionDropFeed,
					},
				},
				llmMock: func(m *mock.Mock) {
					m.On("String", mock.Anything, mock.Anything).Return("spam", nil)
				},
			},
			WhenDetail: whenDetail{
				inputLabels: model.Labels{
					{Key: model.LabelContent, Value: "This is some content that will be transformed to spam."},
					{Key: model.LabelTitle, Value: "Spam Article"},
				},
			},
			ThenExpected: thenExpected{
				outputLabels: nil,
				isErr:        false,
			},
		},
		{
			Scenario: "Create/Update label based on transformed content",
			Given:    "a rule to add a category label based on transformed content",
			When:     "processing labels where transformed content is 'Technology'",
			Then:     "should return labels with the new category label",
			GivenDetail: givenDetail{
				config: &Config{
					{
						SourceLabel:           model.LabelContent,
						SkipTooShortThreshold: ptr.To(10),
						Transform: &Transform{
							ToText: &ToText{
								Type:   ToTextTypePrompt,
								LLM:    "mock-llm",
								Prompt: "{{ .category }}",
							},
						},
						Match:  "Technology",
						Action: ActionCreateOrUpdateLabel,
						Label:  "category",
					},
				},
				llmMock: func(m *mock.Mock) {
					m.On("String", mock.Anything, mock.Anything).Return("Technology", nil)
				},
			},
			WhenDetail: whenDetail{
				inputLabels: model.Labels{
					{Key: model.LabelContent, Value: "Content about AI and programming."},
					{Key: model.LabelTitle, Value: "Tech Article"},
				},
			},
			ThenExpected: thenExpected{
				outputLabels: model.Labels{
					{Key: model.LabelContent, Value: "Content about AI and programming."},
					{Key: model.LabelTitle, Value: "Tech Article"},
					{Key: "category", Value: "Technology"},
				},
				isErr: false,
			},
		},
		{
			Scenario: "No rules match",
			Given:    "a rule that does not match the content",
			When:     "processing labels",
			Then:     "should return the original labels unchanged",
			GivenDetail: givenDetail{
				config: &Config{
					{
						SourceLabel:           model.LabelContent,
						SkipTooShortThreshold: ptr.To(10),
						Match:                 "NonMatchingPattern",
						Action:                ActionDropFeed,
					},
				},
			},
			WhenDetail: whenDetail{
				inputLabels: model.Labels{
					{Key: model.LabelContent, Value: "Some regular content."},
					{Key: model.LabelTitle, Value: "Regular Article"},
				},
			},
			ThenExpected: thenExpected{
				outputLabels: model.Labels{
					{Key: model.LabelContent, Value: "Some regular content."},
					{Key: model.LabelTitle, Value: "Regular Article"},
				},
				isErr: false,
			},
		},
		{
			Scenario: "LLM transformation error",
			Given:    "a rule requiring transformation and LLM returns an error",
			When:     "processing labels",
			Then:     "should return an error",
			GivenDetail: givenDetail{
				config: &Config{
					{
						SourceLabel:           model.LabelContent,
						SkipTooShortThreshold: ptr.To(10),
						Transform: &Transform{
							ToText: &ToText{
								Type:           ToTextTypePrompt,
								LLM:            "mock-llm",
								Prompt:         "{{ .category }}",
								promptRendered: "Analyze the content and categorize it...",
							},
						},
						Match:  ".*",
						Action: ActionCreateOrUpdateLabel,
						Label:  "category",
					},
				},
				llmMock: func(m *mock.Mock) {
					m.On("String", mock.Anything, mock.Anything).Return("", errors.New("LLM failed"))
				},
			},
			WhenDetail: whenDetail{
				inputLabels: model.Labels{
					{Key: model.LabelContent, Value: "Content requiring transformation."},
					{Key: model.LabelTitle, Value: "Transform Error Article"},
				},
			},
			ThenExpected: thenExpected{
				outputLabels: nil,
				err:          errors.New("transform: llm completion: LLM failed"),
				isErr:        true,
			},
		},
		{
			Scenario: "Rule matches but label already exists",
			Given:    "a rule to add a category label and the label already exists",
			When:     "processing labels",
			Then:     "should update the existing label value",
			GivenDetail: givenDetail{
				config: &Config{
					{
						SourceLabel:           model.LabelContent,
						SkipTooShortThreshold: ptr.To(10),
						Transform: &Transform{
							ToText: &ToText{
								Type:           ToTextTypePrompt,
								LLM:            "mock-llm",
								Prompt:         "{{ .category }}",
								promptRendered: "Analyze the content and categorize it...",
							},
						},
						Match:  "Finance",
						Action: ActionCreateOrUpdateLabel,
						Label:  "category",
					},
				},
				llmMock: func(m *mock.Mock) {
					m.On("String", mock.Anything, mock.Anything).Return("Finance", nil)
				},
			},
			WhenDetail: whenDetail{
				inputLabels: model.Labels{
					{Key: model.LabelContent, Value: "Content about stock market."},
					{Key: model.LabelTitle, Value: "Finance Article"},
					{Key: "category", Value: "OldCategory"}, // Existing label
				},
			},
			ThenExpected: thenExpected{
				outputLabels: model.Labels{
					{Key: model.LabelContent, Value: "Content about stock market."},
					{Key: model.LabelTitle, Value: "Finance Article"},
					{Key: "category", Value: "Finance"}, // Updated label
				},
				isErr: false,
			},
		},
		{
			Scenario: "Successfully generate podcast from content",
			Given:    "a rule to convert content to a podcast with all dependencies mocked to succeed",
			When:     "processing labels with content to be converted to a podcast",
			Then:     "should return labels with a new podcast_url label",
			GivenDetail: givenDetail{
				config: &Config{
					{
						SourceLabel: model.LabelContent,
						Transform: &Transform{
							ToPodcast: &ToPodcast{
								LLM:      "mock-llm-transcript",
								TTSLLM:   "mock-llm-tts",
								Speakers: []Speaker{{Name: "narrator", Voice: "alloy"}},
							},
						},
						Action: ActionCreateOrUpdateLabel,
						Label:  "podcast_url",
					},
				},
				llmMock: func(m *mock.Mock) {
					m.On("String", mock.Anything, mock.Anything).Return("This is the podcast script.", nil).Once()
					m.On("WAV", mock.Anything, mock.Anything, mock.AnythingOfType("[]llm.Speaker")).
						Return(io.NopCloser(strings.NewReader("fake audio data")), nil).Once()
				},
				objectStorageMock: func(m *mock.Mock) {
					m.On("Put", mock.Anything, mock.AnythingOfType("string"), mock.Anything, "audio/wav").
						Return("http://storage.example.com/podcast.wav", nil).Once()
					m.On("Get", mock.Anything, mock.AnythingOfType("string")).Return("", object.ErrNotFound).Once()
				},
			},
			WhenDetail: whenDetail{
				inputLabels: model.Labels{
					{Key: model.LabelContent, Value: "This is a long article to be converted into a podcast."},
				},
			},
			ThenExpected: thenExpected{
				outputLabels: model.Labels{
					{Key: model.LabelContent, Value: "This is a long article to be converted into a podcast."},
					{Key: "podcast_url", Value: "http://storage.example.com/podcast.wav"},
				},
				isErr: false,
			},
		},
		{
			Scenario: "Fail podcast generation due to transcription LLM error",
			Given:    "a rule to convert content to a podcast, but the transcription LLM is mocked to fail",
			When:     "processing labels",
			Then:     "should return an error related to transcription failure",
			GivenDetail: givenDetail{
				config: &Config{
					{
						SourceLabel: model.LabelContent,
						Transform: &Transform{
							ToPodcast: &ToPodcast{LLM: "mock-llm-transcript", Speakers: []Speaker{{Name: "narrator", Voice: "alloy"}}},
						},
						Action: ActionCreateOrUpdateLabel, Label: "podcast_url",
					},
				},
				llmMock: func(m *mock.Mock) {
					m.On("String", mock.Anything, mock.Anything).Return("", errors.New("transcript failed")).Once()
				},
			},
			WhenDetail: whenDetail{inputLabels: model.Labels{{Key: model.LabelContent, Value: "article"}}},
			ThenExpected: thenExpected{
				outputLabels: nil,
				err:          errors.New("transform: generate podcast transcript: llm completion: transcript failed"),
				isErr:        true,
			},
		},
		{
			Scenario: "Fail podcast generation due to TTS LLM error",
			Given:    "a rule to convert content to a podcast, but the TTS LLM is mocked to fail",
			When:     "processing labels",
			Then:     "should return an error related to TTS failure",
			GivenDetail: givenDetail{
				config: &Config{
					{
						SourceLabel: model.LabelContent,
						Transform: &Transform{
							ToPodcast: &ToPodcast{LLM: "mock-llm-transcript", TTSLLM: "mock-llm-tts", Speakers: []Speaker{{Name: "narrator", Voice: "alloy"}}},
						},
						Action: ActionCreateOrUpdateLabel, Label: "podcast_url",
					},
				},
				llmMock: func(m *mock.Mock) {
					m.On("String", mock.Anything, mock.Anything).Return("script", nil).Once()
					m.On("WAV", mock.Anything, mock.Anything, mock.Anything).Return(nil, errors.New("tts failed")).Once()
				},
				objectStorageMock: func(m *mock.Mock) {
					m.On("Get", mock.Anything, mock.AnythingOfType("string")).Return("", object.ErrNotFound).Once()
				},
			},
			WhenDetail: whenDetail{inputLabels: model.Labels{{Key: model.LabelContent, Value: "article"}}},
			ThenExpected: thenExpected{
				outputLabels: nil,
				err:          errors.New("transform: generate podcast audio: calling tts llm: tts failed"),
				isErr:        true,
			},
		},
		{
			Scenario: "Fail podcast generation due to object storage error",
			Given:    "a rule to convert content to a podcast, but object storage is mocked to fail",
			When:     "processing labels",
			Then:     "should return an error related to storage failure",
			GivenDetail: givenDetail{
				config: &Config{
					{
						SourceLabel: model.LabelContent,
						Transform: &Transform{
							ToPodcast: &ToPodcast{LLM: "mock-llm-transcript", TTSLLM: "mock-llm-tts", Speakers: []Speaker{{Name: "narrator", Voice: "alloy"}}},
						},
						Action: ActionCreateOrUpdateLabel, Label: "podcast_url",
					},
				},
				llmMock: func(m *mock.Mock) {
					m.On("String", mock.Anything, mock.Anything).Return("script", nil).Once()
					m.On("WAV", mock.Anything, mock.Anything, mock.Anything).Return(io.NopCloser(strings.NewReader("fake audio")), nil).Once()
				},
				objectStorageMock: func(m *mock.Mock) {
					m.On("Put", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return("", errors.New("storage failed")).Once()
					m.On("Get", mock.Anything, mock.AnythingOfType("string")).Return("", object.ErrNotFound).Once()
				},
			},
			WhenDetail: whenDetail{inputLabels: model.Labels{{Key: model.LabelContent, Value: "article"}}},
			ThenExpected: thenExpected{
				outputLabels: nil,
				err:          errors.New("transform: store podcast audio: storage failed"),
				isErr:        true,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.Scenario, func(t *testing.T) {
			// Given.
			var mockLLMFactory llm.Factory
			var mockLLMInstance *mock.Mock
			llmMockOption := component.MockOption(func(m *mock.Mock) {
				mockLLMInstance = m
				if tt.GivenDetail.llmMock != nil {
					tt.GivenDetail.llmMock(m)
				}
			})
			mockLLMFactory, err := llm.NewFactory("", nil, llm.FactoryDependencies{}, llmMockOption)
			Expect(err).NotTo(HaveOccurred())

			var mockObjectStorage object.Storage
			var mockObjectStorageInstance *mock.Mock
			objectStorageMockOption := component.MockOption(func(m *mock.Mock) {
				mockObjectStorageInstance = m
				if tt.GivenDetail.objectStorageMock != nil {
					tt.GivenDetail.objectStorageMock(m)
				}
			})
			mockObjectStorageFactory := object.NewFactory(objectStorageMockOption)
			mockObjectStorage, err = mockObjectStorageFactory.New("test", nil, object.Dependencies{})
			Expect(err).NotTo(HaveOccurred())

			// Manually validate config to compile regex and render templates.
			// In real usage, this happens in `new` or `Reload`.
			for i := range *tt.GivenDetail.config {
				err := (*tt.GivenDetail.config)[i].Validate()
				Expect(err).NotTo(HaveOccurred(), "Rule validation should not fail in test setup")
			}

			// Instantiate the rewriter with the mock factory
			rewriterInstance := &rewriter{
				Base: component.New(&component.BaseConfig[Config, Dependencies]{
					Name:     "TestRewriter",
					Instance: "test",
					Config:   tt.GivenDetail.config,
					Dependencies: Dependencies{
						LLMFactory:    mockLLMFactory, // Pass the mock factory
						ObjectStorage: mockObjectStorage,
					},
				}),
			}

			// Clone input labels to avoid modification by reference affecting assertions.
			inputLabelsCopy := make(model.Labels, len(tt.WhenDetail.inputLabels))
			copy(inputLabelsCopy, tt.WhenDetail.inputLabels)

			// When.
			outputLabels, err := rewriterInstance.Labels(context.Background(), inputLabelsCopy)

			// Then.
			if tt.ThenExpected.isErr {
				Expect(err).To(HaveOccurred())
				// Use MatchError for potentially wrapped errors.
				Expect(err).To(MatchError(ContainSubstring(tt.ThenExpected.err.Error())))
				Expect(outputLabels).To(BeNil())
			} else {
				Expect(err).NotTo(HaveOccurred())
				// Ensure output labels are sorted for consistent comparison.
				if outputLabels != nil {
					outputLabels.EnsureSorted()
				}
				tt.ThenExpected.outputLabels.EnsureSorted()
				Expect(outputLabels).To(Equal(tt.ThenExpected.outputLabels))
			}

			// Verify mock calls if stubs were provided.
			if tt.GivenDetail.llmMock != nil && mockLLMInstance != nil {
				mockLLMInstance.AssertExpectations(t)
			}
			if tt.GivenDetail.objectStorageMock != nil && mockObjectStorageInstance != nil {
				mockObjectStorageInstance.AssertExpectations(t)
			}
		})
	}
}
