//go:build embed

package web

import (
	"encoding/xml"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func init() {
	gin.SetMode(gin.TestMode)
}

func TestServeRobotsTXT(t *testing.T) {
	t.Run("uses_request_origin_for_sitemap", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		req := httptest.NewRequest(http.MethodGet, "/robots.txt", nil)
		req.Host = "example.com"
		req.Header.Set("X-Forwarded-Proto", "https")
		c.Request = req

		serveRobotsTXT(c, requestOrigin(c))

		require.Equal(t, http.StatusOK, w.Code)
		assert.Equal(t, "text/plain; charset=utf-8", w.Header().Get("Content-Type"))
		body := w.Body.String()
		assert.Contains(t, body, "User-agent: *")
		assert.Contains(t, body, "Disallow: /admin/")
		assert.Contains(t, body, "Disallow: /login")
		assert.Contains(t, body, "Sitemap: https://example.com/sitemap.xml")
	})

	t.Run("omits_sitemap_line_without_origin", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest(http.MethodGet, "/robots.txt", nil)

		serveRobotsTXT(c, "")

		require.Equal(t, http.StatusOK, w.Code)
		assert.NotContains(t, w.Body.String(), "Sitemap:")
	})
}

func TestServeSitemapXML(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	req := httptest.NewRequest(http.MethodGet, "/sitemap.xml", nil)
	req.Host = "example.com"
	req.Header.Set("X-Forwarded-Proto", "https")
	c.Request = req

	serveSitemapXML(c, requestOrigin(c))

	require.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "application/xml; charset=utf-8", w.Header().Get("Content-Type"))

	body := w.Body.String()
	assert.Contains(t, body, "<loc>https://example.com/home</loc>")
	assert.Contains(t, body, "<loc>https://example.com/model-plaza</loc>")

	var doc struct {
		URLs []struct {
			Loc string `xml:"loc"`
		} `xml:"url"`
	}
	require.NoError(t, xml.Unmarshal([]byte(body), &doc), "sitemap must be well-formed XML")
	require.Len(t, doc.URLs, len(publicIndexablePaths))
	assert.Equal(t, "https://example.com/home", doc.URLs[0].Loc)
}

func TestServeSitemapXMLEscapesOrigin(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/sitemap.xml", nil)

	serveSitemapXML(c, "https://a&b.example.com")

	require.Equal(t, http.StatusOK, w.Code)
	assert.NotContains(t, w.Body.String(), "https://a&b.example.com")
	assert.Contains(t, w.Body.String(), "https://a&amp;b.example.com")
}

func TestFrontendServer_ServesSEOFiles(t *testing.T) {
	provider := &mockSettingsProvider{settings: map[string]string{"test": "value"}}
	server, err := NewFrontendServer(provider)
	require.NoError(t, err)

	router := gin.New()
	router.Use(server.Middleware())

	cases := []struct {
		path        string
		contentType string
		contains    string
	}{
		{path: "/robots.txt", contentType: "text/plain; charset=utf-8", contains: "User-agent: *"},
		{path: "/sitemap.xml", contentType: "application/xml; charset=utf-8", contains: "/model-plaza"},
	}
	for _, tc := range cases {
		t.Run(tc.path, func(t *testing.T) {
			w := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodGet, tc.path, nil)
			req.Host = "example.com"
			req.Header.Set("X-Forwarded-Proto", "https")
			router.ServeHTTP(w, req)

			require.Equal(t, http.StatusOK, w.Code)
			assert.Equal(t, tc.contentType, w.Header().Get("Content-Type"))
			assert.Contains(t, w.Body.String(), tc.contains)
			assert.Contains(t, w.Body.String(), "https://example.com")
		})
	}
}

func TestServeSEOPathUnknownReturnsNoContent(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/robots.txt", nil)

	serveSEOFile(c, "unknown.txt", "https://example.com")

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Empty(t, w.Body.String())
}

func TestSEOPathsAreStatic(t *testing.T) {
	_, ok := seoStaticPaths["robots.txt"]
	assert.True(t, ok)
	_, ok = seoStaticPaths["sitemap.xml"]
	assert.True(t, ok)
	assert.Equal(t, []string{"/home", "/model-plaza"}, publicIndexablePaths)
}
