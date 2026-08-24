package handlers

import (
	"net/http"
	"os"
	"path/filepath"

	"github.com/gin-gonic/gin"
)

// ServeOpenAPISpec serves the OpenAPI YAML spec file.
func ServeOpenAPISpec(specPath string) gin.HandlerFunc {
	return func(c *gin.Context) {
		absPath := specPath
		if !filepath.IsAbs(absPath) {
			wd, _ := os.Getwd()
			absPath = filepath.Join(wd, absPath)
		}
		c.File(absPath)
	}
}

// ServeSwaggerUI serves the Swagger UI page that loads the OpenAPI spec.
func ServeSwaggerUI() gin.HandlerFunc {
	return func(c *gin.Context) {
		html := `<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="UTF-8">
  <title>RootCauseway API Documentation</title>
  <link rel="stylesheet" href="https://cdn.jsdelivr.net/npm/swagger-ui-dist@5/swagger-ui.css">
  <style>body { margin: 0; }</style>
</head>
<body>
  <div id="swagger-ui"></div>
  <script src="https://cdn.jsdelivr.net/npm/swagger-ui-dist@5/swagger-ui-bundle.js"></script>
  <script>
    SwaggerUIBundle({
      url: '/api/docs/openapi.yaml',
      dom_id: '#swagger-ui',
      deepLinking: true,
      presets: [SwaggerUIBundle.presets.apis],
      layout: 'BaseLayout'
    });
  </script>
</body>
</html>`
		c.Data(http.StatusOK, "text/html; charset=utf-8", []byte(html))
	}
}
