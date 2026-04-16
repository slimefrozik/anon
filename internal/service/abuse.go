package service

import (
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/slimefrozik/anon/internal/model"
)

var blocklistPatterns = []string{
	"<redacted>",
}

var maxPostLength = 1000
var maxCommentLength = 300

type AbuseService struct{}

func NewAbuseService() *AbuseService {
	return &AbuseService{}
}

func (s *AbuseService) ValidatePost(post *model.CreatePostRequest) error {
	if post.ContentType != "text" && post.ContentType != "image" {
		return fmt.Errorf("invalid content type")
	}
	if post.ContentType == "text" && utf8.RuneCountInString(post.TextContent) > maxPostLength {
		return fmt.Errorf("text too long")
	}
	if s.matchesBlocklist(post.TextContent) {
		return fmt.Errorf("content rejected")
	}
	return nil
}

func (s *AbuseService) ValidateComment(text string) error {
	if utf8.RuneCountInString(text) > maxCommentLength {
		return fmt.Errorf("comment too long")
	}
	if s.matchesBlocklist(text) {
		return fmt.Errorf("content rejected")
	}
	return nil
}

func (s *AbuseService) ShouldShadowBan(text string) bool {
	return s.matchesBlocklist(text)
}

func (s *AbuseService) matchesBlocklist(text string) bool {
	lower := strings.ToLower(text)
	for _, pattern := range blocklistPatterns {
		if strings.Contains(lower, pattern) {
			return true
		}
	}
	return false
}
