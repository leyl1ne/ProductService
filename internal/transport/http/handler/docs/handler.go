package docs

import (
	"embed"
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
)

//go:embed api.yaml
var specFS embed.FS

type DocsHandler struct {
	spec []byte
}

func NewDocsHandler() (*DocsHandler, error) {
	const op = "handler.docs.NewDocsHandler"

	spec, err := specFS.ReadFile("api.yaml")
	if err != nil {
		return nil, fmt.Errorf("%s: failed to read api file: %w", op, err)
	}

	return &DocsHandler{
		spec: spec,
	}, nil
}

func (h *DocsHandler) Spec() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Data(
			http.StatusOK,
			"application/yaml",
			h.spec,
		)
	}
}

func (h *DocsHandler) UI() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Data(
			http.StatusOK,
			"test/html; charset=utf-8",
			swaggerUIHTML,
		)
	}
}

func (h *DocsHandler) Redirect() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Redirect(http.StatusMovedPermanently, "/swagger")
	}
}

var swaggerUIHTML = []byte(`
<!doctype html>
<html>
<head>
  <meta charset="utf-8">
  <title>AgroLink API</title>

  <link rel="stylesheet"
        href="https://unpkg.com/swagger-ui-dist@5/swagger-ui.css">

  <style>
    body {
      margin: 0;
      background: #f6f7fb;
    }

    #swagger-ui {
      max-width: 1400px;
      margin: 0 auto;
    }
  </style>
</head>

<body>
  <div id="swagger-ui"></div>

  <script src="https://unpkg.com/swagger-ui-dist@5/swagger-ui-bundle.js"></script>

  <script>
    window.onload = function () {
      SwaggerUIBundle({
        url: "/swagger/api.yaml",
        dom_id: "#swagger-ui",
        deepLinking: true,
        presets: [
          SwaggerUIBundle.presets.apis
        ],
        layout: "BaseLayout"
      });
    };
  </script>
</body>
</html>
`)
