//go:build embed

package web

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

// seoStaticPaths 是后端按请求 Origin 动态生成的 SEO 文件。
// 它们不在前端 dist 中，避免把占位域名写进静态资源；
// 多域名部署时每个 Origin 都能拿到指向自身的 robots/sitemap。
var seoStaticPaths = map[string]bool{
	"robots.txt":  true,
	"sitemap.xml": true,
}

// publicIndexablePaths 是当前允许搜索引擎收录的公共页面。
// 与前端路由 meta.indexable 保持一致：新增可收录页面时两处同步。
var publicIndexablePaths = []string{
	"/home",
	"/model-plaza",
}

// serveSEOFile 分发 robots.txt / sitemap.xml。
func serveSEOFile(c *gin.Context, cleanPath, origin string) {
	switch cleanPath {
	case "robots.txt":
		serveRobotsTXT(c, origin)
	case "sitemap.xml":
		serveSitemapXML(c, origin)
	}
}

func serveRobotsTXT(c *gin.Context, origin string) {
	var b strings.Builder
	b.WriteString("User-agent: *\n")
	b.WriteString("Allow: /\n")
	for _, disallow := range []string{
		"/login",
		"/register",
		"/email-verify",
		"/setup",
		"/auth/",
		"/dashboard",
		"/admin/",
		"/api/",
		"/v1/",
		"/v1beta/",
		"/backend-api/",
		"/antigravity/",
		"/responses",
		"/images/",
		"/videos/",
	} {
		b.WriteString("Disallow: " + disallow + "\n")
	}
	if origin != "" {
		fmt.Fprintf(&b, "Sitemap: %s/sitemap.xml\n", origin)
	}

	c.Header("Content-Type", "text/plain; charset=utf-8")
	c.Header("Cache-Control", "public, max-age=3600")
	c.String(http.StatusOK, b.String())
}

func serveSitemapXML(c *gin.Context, origin string) {
	var buf bytes.Buffer
	buf.WriteString(`<?xml version="1.0" encoding="UTF-8"?>` + "\n")
	buf.WriteString(`<urlset xmlns="http://www.sitemaps.org/schemas/sitemap/0.9">` + "\n")
	for _, path := range publicIndexablePaths {
		buf.WriteString("  <url><loc>")
		_ = xml.EscapeText(&buf, []byte(origin+path))
		buf.WriteString("</loc></url>\n")
	}
	buf.WriteString("</urlset>\n")

	c.Header("Content-Type", "application/xml; charset=utf-8")
	c.Header("Cache-Control", "public, max-age=3600")
	c.String(http.StatusOK, buf.String())
}
