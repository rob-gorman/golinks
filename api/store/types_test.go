package store

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGoLink_Validate(t *testing.T) {
	t.Parallel()
	
	tests := []struct {
		name  string
		link  GoLink
		valid bool
	}{
		{
			name: "valid",
			link: GoLink{
				Short: "valid-short-1",
				Url:   "https://example.com/valid",
				Desc:  "A valid link",
			},
			valid: true,
		},
		{
			name: "invalid-url",
			link: GoLink{
				Short: "invalid-url",
				Url:   "invalid-url",
				Desc:  "An invalid URL link",
			},
		},
		{
			name: "no scheme invalid",
			link: GoLink{
				Short: "no-scheme",
				Url:   "example.com",
				Desc:  "An invalid URL link",
			},
		},
		{
			name: "invalid-short",
			link: GoLink{
				Short: "",
				Url:   "https://example.com/valid",
				Desc:  "A valid link",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.link.Validate()
			if tt.valid {
				assert.NoError(t, err)
			} else {
				assert.Error(t, err)
			}
		})
	}
}

func TestLinkUpdate_Validate(t *testing.T) {
	t.Parallel()

	strPtr := func(s string) *string { return &s }
	tests := []struct {
		name  string
		link  LinkUpdate
		valid bool
	}{
		{
			name: "valid-all",
			link: LinkUpdate{
				Short: strPtr("valid-short-1"),
				Url:   strPtr("https://example.com/valid"),
				Desc:  strPtr("A valid link"),
			},
			valid: true,
		},
		{
			name: "valid-partial-url",
			link: LinkUpdate{
				Url: strPtr("https://example.com/valid"),
			},
			valid: true,
		},
		{
			name: "valid-partial-short",
			link: LinkUpdate{
				Short: strPtr("valid-short-1"),
			},
			valid: true,
		},
		{
			name: "invalid-url",
			link: LinkUpdate{
				Short: strPtr("valid-short-1"),
				Url:   strPtr("example.com"),
				Desc:  strPtr("An invalid URL link"),
			},
		},
		{
			name: "invalid-short",
			link: LinkUpdate{
				Short: strPtr(""),
				Url:   strPtr("https://example.com/valid"),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.link.Validate()
			if tt.valid {
				assert.NoError(t, err)
			} else {
				assert.Error(t, err)
			}
		})
	}
}
