package quality

import (
	"bufio"
	"os"
	"path/filepath"
	"strings"
)

type goModuleMetadata struct {
	modulePath string
	resolvable []string
}

type goModuleResolver struct {
	root             string
	workspaceModules []string
	workspaceReplace []string
	cache            map[string]goModuleMetadata
}

func newGoModuleResolver(root string) goModuleResolver {
	resolver := goModuleResolver{root: filepath.Clean(root), cache: map[string]goModuleMetadata{}}
	workPath := filepath.Join(resolver.root, "go.work")
	uses, replaces := parseGoManifest(workPath, true)
	resolver.workspaceReplace = replaces
	for _, use := range uses {
		moduleDir := use
		if !filepath.IsAbs(moduleDir) {
			moduleDir = filepath.Join(resolver.root, moduleDir)
		}
		modulePath, _ := parseGoManifest(filepath.Join(moduleDir, "go.mod"), false)
		resolver.workspaceModules = append(resolver.workspaceModules, modulePath...)
	}
	return resolver
}

func (r *goModuleResolver) metadataForFile(rel string) goModuleMetadata {
	dir := filepath.Dir(filepath.Join(r.root, filepath.FromSlash(rel)))
	for {
		if metadata, ok := r.cache[dir]; ok {
			return metadata
		}
		modPath := filepath.Join(dir, "go.mod")
		if _, err := os.Stat(modPath); err == nil {
			modules, dependencies := parseGoManifest(modPath, false)
			metadata := goModuleMetadata{resolvable: append([]string{}, r.workspaceModules...)}
			metadata.resolvable = append(metadata.resolvable, r.workspaceReplace...)
			metadata.resolvable = append(metadata.resolvable, dependencies...)
			if len(modules) > 0 {
				metadata.modulePath = modules[0]
			}
			r.cache[dir] = metadata
			return metadata
		}
		parent := filepath.Dir(dir)
		if parent == dir || !pathWithin(parent, r.root) {
			return goModuleMetadata{resolvable: append(append([]string{}, r.workspaceModules...), r.workspaceReplace...)}
		}
		dir = parent
	}
}

func pathWithin(path, root string) bool {
	rel, err := filepath.Rel(root, path)
	return err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}

// parseGoManifest returns module/use paths first and require/replace module
// paths second. It intentionally parses only resolution directives and does
// not execute the Go toolchain or consult a module cache.
func parseGoManifest(path string, workspace bool) ([]string, []string) {
	file, err := os.Open(path) //nolint:gosec // path is a fixed manifest under the configured scan root
	if err != nil {
		return nil, nil
	}
	defer file.Close()

	var primary, dependencies []string
	block := ""
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(strings.SplitN(scanner.Text(), "//", 2)[0])
		if line == "" {
			continue
		}
		if line == ")" {
			block = ""
			continue
		}
		fields := strings.Fields(line)
		if len(fields) == 2 && fields[1] == "(" {
			block = fields[0]
			continue
		}
		directive := block
		if directive == "" {
			directive = fields[0]
			fields = fields[1:]
		}
		if len(fields) == 0 {
			continue
		}
		switch directive {
		case "module":
			primary = append(primary, fields[0])
		case "use":
			if workspace {
				primary = append(primary, fields[0])
			}
		case "require":
			if !workspace {
				dependencies = append(dependencies, fields[0])
			}
		case "replace":
			dependencies = append(dependencies, fields[0])
		}
	}
	return primary, dependencies
}

func (m goModuleMetadata) resolvesImport(importPath string) bool {
	if importPath == "" || !strings.Contains(firstSegment(importPath), ".") {
		return true
	}
	if modulePrefixMatch(importPath, m.modulePath) {
		return true
	}
	for _, module := range m.resolvable {
		if modulePrefixMatch(importPath, module) {
			return true
		}
	}
	return false
}

func modulePrefixMatch(importPath, modulePath string) bool {
	return modulePath != "" && (importPath == modulePath || strings.HasPrefix(importPath, modulePath+"/"))
}
