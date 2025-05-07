package llm_reviewer

import (
	"embed"
	"encoding/json"
	"fmt"
	"github.com/apache/answer-plugins/util"
	"github.com/apache/answer/plugin"
	my18N "github.com/cyoung96/llm-reviewer/i18n"
	"github.com/segmentfault/pacman/i18n"
	"strings"
)

//go:embed  info.yaml
var Info embed.FS

type Reviewer struct {
	Config *ReviewerConfig
}

const (
	OptionNone  = "none"
	OptionAll   = "all"
	OptionFirst = "first"
)

type ReviewerConfig struct {
	PostAllNeedReview      bool   `json:"review_post_all"`
	PostNeedReview         bool   `json:"review_post"`
	PostReviewOption       string `json:"review_post_option"`
	PostReviewKeywords     string `json:"review_post_keywords"`
	PostDisallowedKeywords string `json:"disallowed_keywords"`
}

func init() {
	plugin.Register(&Reviewer{
		Config: &ReviewerConfig{},
	})
}

func (r *Reviewer) Info() plugin.Info {
	info := &util.Info{}
	info.GetInfo(Info)

	return plugin.Info{
		Name:        plugin.MakeTranslator(my18N.InfoName),
		SlugName:    info.SlugName,
		Description: plugin.MakeTranslator(my18N.InfoDescription),
		Author:      info.Author,
		Version:     info.Version,
		Link:        info.Link,
	}
}

func (r *Reviewer) Review(content *plugin.ReviewContent) (result *plugin.ReviewResult) {
	result = &plugin.ReviewResult{Approved: true, ReviewStatus: plugin.ReviewStatusApproved}

	//// If the author is admin, no need to review
	//if content.Author.Role > 1 {
	//	return result
	//}

	if r.Config.PostReviewOption == OptionAll || (r.Config.PostReviewOption == "" && r.Config.PostAllNeedReview) {
		result = &plugin.ReviewResult{
			Approved:     false,
			ReviewStatus: plugin.ReviewStatusNeedReview,
			Reason:       plugin.TranslateWithData(i18n.Language(content.Language), my18N.CommentNeedReview, nil),
		}
		return result
	}

	if (r.Config.PostReviewOption == OptionFirst || (r.Config.PostReviewOption == "" && r.Config.PostNeedReview)) && content.Author.ApprovedQuestionAmount+content.Author.ApprovedAnswerAmount == 0 {
		result = &plugin.ReviewResult{
			Approved:     false,
			ReviewStatus: plugin.ReviewStatusNeedReview,
			Reason:       plugin.TranslateWithData(i18n.Language(content.Language), my18N.CommentNeedReview, nil),
		}
		return result
	}

	keywords := strings.Split(r.Config.PostReviewKeywords, "\n")
	disallowedKeywords := strings.Split(r.Config.PostDisallowedKeywords, "\n")

	// Check if the post contains the keywords that need review
	for _, keyword := range keywords {
		keyword = strings.TrimSpace(keyword)
		if len(keyword) == 0 {
			continue
		}
		keyword = strings.ToLower(keyword)
		if strings.Contains(strings.ToLower(content.Title), keyword) ||
			strings.Contains(strings.ToLower(content.Content), keyword) ||
			strings.Contains(content.IP, keyword) ||
			strings.Contains(content.UserAgent, keyword) ||
			r.checkTags(content.Tags, keyword) {
			return &plugin.ReviewResult{
				Approved:     false,
				ReviewStatus: plugin.ReviewStatusNeedReview,
				Reason:       fmt.Sprintf(plugin.TranslateWithData(i18n.Language(content.Language), my18N.CommentMatchWordReview, nil), keyword),
			}
		}
	}

	// If the post contains the disallowed keywords
	for _, disallowedKeyword := range disallowedKeywords {
		disallowedKeyword = strings.TrimSpace(disallowedKeyword)
		if len(disallowedKeyword) == 0 {
			continue
		}
		disallowedKeyword = strings.ToLower(disallowedKeyword)
		if strings.Contains(strings.ToLower(content.Title), disallowedKeyword) ||
			strings.Contains(strings.ToLower(content.Content), disallowedKeyword) ||
			strings.Contains(content.IP, disallowedKeyword) ||
			strings.Contains(content.UserAgent, disallowedKeyword) ||
			r.checkTags(content.Tags, disallowedKeyword) {
			return &plugin.ReviewResult{
				Approved:     false,
				ReviewStatus: plugin.ReviewStatusDeleteDirectly,
				Reason:       fmt.Sprintf(plugin.TranslateWithData(i18n.Language(content.Language), my18N.CommentMatchWordReview, nil), disallowedKeyword),
			}
		}
	}

	return result
}

func (r *Reviewer) ConfigFields() []plugin.ConfigField {
	defaultOption := OptionNone
	if r.Config.PostReviewOption == "" {
		if r.Config.PostAllNeedReview {
			defaultOption = OptionAll
		} else if r.Config.PostNeedReview {
			defaultOption = OptionFirst
		}
	} else {
		defaultOption = r.Config.PostReviewOption
	}

	return []plugin.ConfigField{
		{
			Name:      "review_post_option",
			Type:      plugin.ConfigTypeSelect,
			Title:     plugin.MakeTranslator(my18N.ConfigReviewPostTitle),
			Required:  false,
			UIOptions: plugin.ConfigFieldUIOptions{},
			Value:     defaultOption,
			Options: []plugin.ConfigFieldOption{
				{
					Value: OptionNone,
					Label: plugin.MakeTranslator(my18N.ConfigSelectOption),
				},
				{
					Value: OptionAll,
					Label: plugin.MakeTranslator(my18N.ConfigReviewPostLabelAll),
				},
				{
					Value: OptionFirst,
					Label: plugin.MakeTranslator(my18N.ConfigReviewPostLabelFirst),
				},
			},
		},
		{
			Name:        "review_post_keywords",
			Type:        plugin.ConfigTypeTextarea,
			Title:       plugin.MakeTranslator(my18N.ConfigReviewPostKeywordsTitle),
			Description: plugin.MakeTranslator(my18N.ConfigReviewPostKeywordsDescription),
			Value:       r.Config.PostReviewKeywords,
		},
		{
			Name:        "disallowed_keywords",
			Type:        plugin.ConfigTypeTextarea,
			Title:       plugin.MakeTranslator(my18N.ConfigDisallowedKeywordsTitle),
			Description: plugin.MakeTranslator(my18N.ConfigDisallowedKeywordsDescription),
			Value:       r.Config.PostDisallowedKeywords,
		},
	}
}

func (r *Reviewer) ConfigReceiver(config []byte) error {
	c := &ReviewerConfig{}
	_ = json.Unmarshal(config, c)

	if c.PostReviewOption == OptionAll {
		c.PostAllNeedReview = true
		c.PostNeedReview = false
	} else if c.PostReviewOption == OptionFirst {
		c.PostAllNeedReview = false
		c.PostNeedReview = true
	} else if c.PostReviewOption == OptionNone {
		c.PostAllNeedReview = false
		c.PostNeedReview = false
	}

	r.Config = c
	return nil
}

func (r *Reviewer) checkTags(tags []string, keyword string) bool {
	for _, tag := range tags {
		if strings.Contains(strings.ToLower(tag), keyword) {
			return true
		}
	}
	return false
}
