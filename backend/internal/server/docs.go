package server

import (
	"crypto/sha256"
	"encoding/base64"
	"net/http"

	"github.com/gin-gonic/gin"
)

const docsInitializer = `window.onload = function () {
  SwaggerUIBundle({url: "/openapi.yaml", dom_id: "#swagger-ui", deepLinking: true, persistAuthorization: false, validatorUrl: null});
};`

const docsHTML = `<!doctype html>
<html lang="pt-BR">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <title>Oficina API</title>
  <link rel="stylesheet" href="https://cdn.jsdelivr.net/npm/swagger-ui-dist@5.33.1/swagger-ui.css">
</head>
<body>
  <header><h1>Oficina API</h1><p>Documentação interativa da oficina. A autorização permanece somente na memória desta página.</p></header>
  <div id="swagger-ui"></div>
  <script src="https://cdn.jsdelivr.net/npm/swagger-ui-dist@5.33.1/swagger-ui-bundle.js"></script>
  <script>` + docsInitializer + `</script>
</body>
</html>`

func serveDocs(c *gin.Context) {
	hash := sha256.Sum256([]byte(docsInitializer))
	c.Header("Content-Security-Policy", "default-src 'none'; script-src https://cdn.jsdelivr.net 'sha256-"+base64.StdEncoding.EncodeToString(hash[:])+"'; style-src https://cdn.jsdelivr.net 'unsafe-inline'; img-src 'self' data:; font-src https://cdn.jsdelivr.net; connect-src 'self'; base-uri 'none'; frame-ancestors 'none'")
	c.Data(http.StatusOK, "text/html; charset=utf-8", []byte(docsHTML))
}
