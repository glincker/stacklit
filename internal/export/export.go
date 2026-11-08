package export

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Render turns the input into one or more files for the given format.
func Render(format string, in Input, o Options) ([]File, error) {
	switch format {
	case "obsidian", "markdown-wiki":
		if in.Index == nil {
			return nil, fmt.Errorf("%s needs stacklit.json", format)
		}
		if format == "obsidian" {
			return renderObsidian(in, o), nil
		}
		return renderWiki(in, o), nil
	case "graphml", "cypher", "json", "dot", "mermaid":
		m, err := in.model(o)
		if err != nil {
			return nil, err
		}
		var body string
		var ext string
		switch format {
		case "graphml":
			body, ext = renderGraphML(m), "graphml"
		case "cypher":
			body, ext = renderCypher(m), "cypher"
		case "json":
			body, err = renderJSON(m)
			ext = "json"
			if err != nil {
				return nil, err
			}
		case "dot":
			body, ext = renderDOT(m.capped(o.maxNodes())), "dot"
		case "mermaid":
			body, ext = renderMermaid(m.capped(o.maxNodes())), "mmd"
		}
		return []File{{Name: "stacklit-" + m.Scope + "." + ext, Content: body}}, nil
	}
	return nil, fmt.Errorf("unknown format %q (supported: %s)", format, strings.Join(Formats, ", "))
}

// WriteStdout prints files, with a marker line between files when there are several.
func WriteStdout(w interface{ Write([]byte) (int, error) }, files []File) error {
	for i, f := range files {
		if len(files) > 1 {
			if i > 0 {
				fmt.Fprint(w, "\n")
			}
			fmt.Fprintf(w, "<!-- file: %s -->\n", f.Name)
		}
		if _, err := fmt.Fprint(w, f.Content); err != nil {
			return err
		}
	}
	return nil
}

// WriteTo writes files under out. A single file may target a plain file path;
// otherwise out is a directory that is created when missing.
func WriteTo(out string, files []File) ([]string, error) {
	if len(files) == 1 {
		if st, err := os.Stat(out); (err == nil && !st.IsDir()) || (err != nil && !strings.HasSuffix(out, "/") && filepath.Ext(out) != "") {
			if err := os.MkdirAll(filepath.Dir(out), 0o755); err != nil {
				return nil, err
			}
			return []string{out}, os.WriteFile(out, []byte(files[0].Content), 0o644)
		}
	}
	var written []string
	for _, f := range files {
		p := filepath.Join(out, filepath.FromSlash(f.Name))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			return written, err
		}
		if err := os.WriteFile(p, []byte(f.Content), 0o644); err != nil {
			return written, err
		}
		written = append(written, p)
	}
	return written, nil
}
