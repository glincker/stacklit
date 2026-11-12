package parser

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
)

// cppMarkers are constructs that do not compile as C. They are matched as
// plain byte sequences on the whole header, because a header can hold several
// preprocessor branches and any C++ branch makes the file C++.
var cppMarkers = [][]byte{
	[]byte("class "),
	[]byte("namespace "),
	[]byte("template<"),
	[]byte("template <"),
	[]byte("public:"),
	[]byte("private:"),
	[]byte("protected:"),
	[]byte("std::"),
	[]byte("virtual "),
	[]byte("constexpr "),
	[]byte("nullptr"),
	[]byte("using namespace "),
	[]byte("extern \"C++\""),
}

// cppUnitExts are translation unit extensions that mark a directory as C++.
var cppUnitExts = []string{".cc", ".cpp", ".cxx"}

// HeaderLanguage decides whether a .h header belongs to C or C++. It returns
// "cpp" when the source shows a C++ construct, or when a C++ translation unit
// shares the header's directory, and "c" otherwise. fullPath may be empty or
// unresolvable; the neighbour check is then skipped. src may be nil.
func HeaderLanguage(fullPath string, src []byte) string {
	if HeaderLooksCpp(src) {
		return "cpp"
	}
	if SiblingIsCpp(fullPath) {
		return "cpp"
	}
	return "c"
}

// HeaderLooksCpp reports whether the header source uses C++ only constructs.
func HeaderLooksCpp(src []byte) bool {
	for _, m := range cppMarkers {
		if bytes.Contains(src, m) {
			return true
		}
	}
	return false
}

// SiblingIsCpp reports whether a C++ translation unit sits next to fullPath.
// A unit with the same base name wins over an unrelated one in the same
// directory. An empty or unreadable path reports false.
func SiblingIsCpp(fullPath string) bool {
	if fullPath == "" {
		return false
	}
	dir := filepath.Dir(fullPath)
	entries, err := os.ReadDir(dir)
	if err != nil {
		return false
	}
	base := strings.ToLower(strings.TrimSuffix(filepath.Base(fullPath), filepath.Ext(fullPath)))
	sameName, anyName := false, false
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := strings.ToLower(e.Name())
		ext := filepath.Ext(name)
		if !containsString(cppUnitExts, ext) {
			continue
		}
		anyName = true
		if strings.TrimSuffix(name, ext) == base {
			sameName = true
			break
		}
	}
	return sameName || anyName
}

func containsString(list []string, s string) bool {
	for _, v := range list {
		if v == s {
			return true
		}
	}
	return false
}
