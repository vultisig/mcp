package skills

import (
	"bufio"
	"bytes"
	"embed"
	"encoding/json"
	"net/http"
	"path/filepath"
	"strings"

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
func NewHandler() http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /skills", handleList)
	mux.HandleFunc("GET /skills/{file}", handleFile)

	return mux
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
