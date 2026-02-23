package skills

import (
	"bufio"
	"bytes"
	"context"
	"embed"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"path/filepath"
	"strings"
	"time"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"gopkg.in/yaml.v3"
)

//go:embed files/*.md
var skillFS embed.FS

type skillMeta struct {
	File        string   `json:"file"`
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Tags        []string `json:"tags"`
}

type frontmatter struct {
	Name        string   `yaml:"name"`
	Description string   `yaml:"description"`
	Tags        []string `yaml:"tags"`
}

// cachedListing is the pre-built JSON response for GET /skills.
var cachedListing []byte

// fileContents maps "name.md" to the full raw markdown bytes.
var fileContents map[string][]byte

func init() {
	entries, err := skillFS.ReadDir("files")
	if err != nil {
		panic("skills: read embedded dir: " + err.Error())
	}

	var skills []skillMeta
	fileContents = make(map[string][]byte, len(entries))

	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".md") {
			continue
		}

		raw, err := skillFS.ReadFile(filepath.Join("files", e.Name()))
		if err != nil {
			continue
		}

		fileContents[e.Name()] = raw

		fm := parseFrontmatter(raw)
		skills = append(skills, skillMeta{
			File:        e.Name(),
			Name:        fm.Name,
			Description: fm.Description,
			Tags:        fm.Tags,
		})
	}

	cachedListing, _ = json.Marshal(skills)
}

// RegisterMCPResources registers each embedded skill file as an MCP resource
// so that clients can discover them via resources/list and read them via
// resources/read.
func RegisterMCPResources(s *server.MCPServer) {
	for name, content := range fileContents {
		fm := parseFrontmatter(content)
		uri := fmt.Sprintf("skill://vultisig/%s", name)
		raw := make([]byte, len(content))
		copy(raw, content)

		s.AddResource(
			mcp.NewResource(
				uri,
				fm.Name,
				mcp.WithResourceDescription(fm.Description),
				mcp.WithMIMEType("text/markdown"),
			),
			func(ctx context.Context, req mcp.ReadResourceRequest) ([]mcp.ResourceContents, error) {
				return []mcp.ResourceContents{
					mcp.TextResourceContents{
						URI:      req.Params.URI,
						MIMEType: "text/markdown",
						Text:     string(raw),
					},
				}, nil
			},
		)
	}
}

// parseFrontmatter extracts YAML frontmatter delimited by "---" lines.
func parseFrontmatter(data []byte) frontmatter {
	var fm frontmatter

	scanner := bufio.NewScanner(bytes.NewReader(data))
	if !scanner.Scan() || strings.TrimSpace(scanner.Text()) != "---" {
		return fm
	}

	var buf bytes.Buffer
	for scanner.Scan() {
		line := scanner.Text()
		if strings.TrimSpace(line) == "---" {
			break
		}
		buf.WriteString(line)
		buf.WriteByte('\n')
	}

	_ = yaml.Unmarshal(buf.Bytes(), &fm)
	return fm
}

// NewHandler returns an http.Handler that serves:
//   - GET /skills — JSON array of skill metadata
//   - GET /skills/{name}.md — raw markdown content
func NewHandler(logger *log.Logger) http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /skills", logged(logger, "list", handleList))
	mux.HandleFunc("GET /skills/{file}", logged(logger, "get", handleFile))

	return mux
}

// logged wraps an HTTP handler with request/response logging that mirrors
// the [CALL]/[OK]/[FAIL] format used by the MCP tool middleware.
func logged(logger *log.Logger, action string, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		file := r.PathValue("file")
		if file == "" {
			file = "*"
		}
		logger.Printf("[CALL]  skill=%-20s action=%-5s remote=%s", file, action, r.RemoteAddr)

		rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
		start := time.Now()
		next(rec, r)
		duration := time.Since(start)

		if rec.status >= 400 {
			logger.Printf("[FAIL]  skill=%-20s action=%-5s duration=%-12s status=%d", file, action, duration, rec.status)
		} else {
			logger.Printf("[OK]    skill=%-20s action=%-5s duration=%-12s status=%d", file, action, duration, rec.status)
		}
	}
}

// statusRecorder captures the HTTP status code written by downstream handlers.
type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (r *statusRecorder) WriteHeader(code int) {
	r.status = code
	r.ResponseWriter.WriteHeader(code)
}

func handleList(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Write(cachedListing)
}

func handleFile(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("file")
	if !strings.HasSuffix(name, ".md") {
		http.NotFound(w, r)
		return
	}

	content, ok := fileContents[name]
	if !ok {
		http.NotFound(w, r)
		return
	}

	w.Header().Set("Content-Type", "text/markdown")
	w.Write(content)
}
