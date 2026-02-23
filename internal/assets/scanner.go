package assets

import (
	"io/fs"
	"path/filepath"
	"regexp"
	"runtime"
	"slices"
	"strings"
	"sync"
	"unicode"
)

var sourceExtensions = map[string]struct{}{
	".swift":      {},
	".m":          {},
	".h":          {},
	".xib":        {},
	".storyboard": {},
}

var swiftResourceRefRe = regexp.MustCompile(`\b(?:(?:UI|NS)?(?:Image|Color)|(?:NS)?DataAsset|Data)\s*\(\s*resource\s*:\s*\.([A-Za-z_][A-Za-z0-9_]*)`)
var ibAssetRefRe = regexp.MustCompile(`\b(?:image|selectedImage|highlightedImage|name)\s*=\s*"([A-Za-z0-9._ -]+)"`)
var sourceWordTokenRe = regexp.MustCompile(`[A-Za-z0-9_][A-Za-z0-9_-]*`)

type Options struct {
	Root    string
	Include []string
	Exclude []string
	Workers int
}

type Result struct {
	AssetCatalogs int
	AssetNames    []string
	UsedAssets    []string
	UnusedAssets  []string
	UnusedByFile  map[string][]string
}

type discoveredAsset struct {
	Name        string
	CatalogPath string
	AssetPath   string
}

func Scan(opts Options) (Result, error) {
	workers := opts.Workers
	if workers < 1 {
		workers = runtime.NumCPU()
	}
	if workers < 1 {
		workers = 1
	}

	assetCatalogs, assetNames, assetTypesByName, discoveredAssets, err := collectAssets(opts.Root, opts.Include, opts.Exclude)
	if err != nil {
		return Result{}, err
	}
	assetSet := make(map[string]struct{}, len(assetNames))
	for _, name := range assetNames {
		assetSet[name] = struct{}{}
	}

	usedSet, err := collectUsedAssets(opts.Root, opts.Include, opts.Exclude, assetSet, assetTypesByName, workers)
	if err != nil {
		return Result{}, err
	}

	used := make([]string, 0, len(usedSet))
	for name := range usedSet {
		used = append(used, name)
	}
	slices.Sort(used)

	unused := make([]string, 0, len(assetNames))
	for _, name := range assetNames {
		if _, ok := usedSet[name]; !ok {
			unused = append(unused, name)
		}
	}
	unusedByFile := make(map[string][]string)
	for _, asset := range discoveredAssets {
		if _, ok := usedSet[asset.Name]; ok {
			continue
		}
		unusedByFile[asset.CatalogPath] = append(unusedByFile[asset.CatalogPath], asset.AssetPath)
	}
	for file, values := range unusedByFile {
		slices.Sort(values)
		unusedByFile[file] = values
	}

	return Result{
		AssetCatalogs: assetCatalogs,
		AssetNames:    assetNames,
		UsedAssets:    used,
		UnusedAssets:  unused,
		UnusedByFile:  unusedByFile,
	}, nil
}

func collectAssets(root string, include []string, exclude []string) (int, []string, map[string]map[string]struct{}, []discoveredAsset, error) {
	assetNames := make([]string, 0, 256)
	seen := make(map[string]struct{}, 256)
	assetTypesByName := make(map[string]map[string]struct{}, 256)
	discoveredAssets := make([]discoveredAsset, 0, 256)
	assetCatalogs := 0

	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		rel, relErr := filepath.Rel(root, path)
		if relErr != nil {
			return relErr
		}
		if matchesAny(rel, exclude) {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if len(include) > 0 && !matchesAny(rel, include) {
			if d.IsDir() && rel != "." {
				return nil
			}
			return nil
		}

		if d.IsDir() && strings.HasSuffix(d.Name(), ".xcassets") {
			assetCatalogs++
			return nil
		}

		if d.IsDir() && isAssetSetDir(d.Name()) {
			assetExt := strings.TrimPrefix(filepath.Ext(d.Name()), ".")
			name := strings.TrimSuffix(d.Name(), filepath.Ext(d.Name()))
			if name != "" {
				catalogPath := catalogPathForAsset(path)
				if catalogPath == "" {
					catalogPath = filepath.Dir(path)
				}
				discoveredAssets = append(discoveredAssets, discoveredAsset{
					Name:        name,
					CatalogPath: catalogPath,
					AssetPath:   path,
				})
				if _, ok := seen[name]; !ok {
					seen[name] = struct{}{}
					assetNames = append(assetNames, name)
				}
				if _, ok := assetTypesByName[name]; !ok {
					assetTypesByName[name] = make(map[string]struct{}, 1)
				}
				assetTypesByName[name][assetExt] = struct{}{}
			}
		}

		return nil
	})
	if err != nil {
		return 0, nil, nil, nil, err
	}

	slices.Sort(assetNames)
	slices.SortFunc(discoveredAssets, func(a, b discoveredAsset) int {
		return strings.Compare(a.AssetPath, b.AssetPath)
	})
	return assetCatalogs, assetNames, assetTypesByName, discoveredAssets, nil
}

func catalogPathForAsset(assetPath string) string {
	marker := ".xcassets" + string(filepath.Separator)
	idx := strings.Index(assetPath, marker)
	if idx < 0 {
		if strings.HasSuffix(assetPath, ".xcassets") {
			return assetPath
		}
		return ""
	}
	return assetPath[:idx+len(".xcassets")]
}

func collectUsedAssets(root string, include []string, exclude []string, assetSet map[string]struct{}, assetTypesByName map[string]map[string]struct{}, workers int) (map[string]struct{}, error) {
	fileCh := make(chan string, workers*2)
	errCh := make(chan error, 1)
	usedSet := make(map[string]struct{}, 128)
	var usedMu sync.Mutex
	swiftResourceCandidates := buildSwiftResourceCandidateIndex(assetSet, assetTypesByName)

	var wg sync.WaitGroup
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for path := range fileCh {
				content, err := osReadFile(path)
				if err != nil {
					select {
					case errCh <- err:
					default:
					}
					continue
				}

				ext := strings.ToLower(filepath.Ext(path))
				switch ext {
				case ".storyboard", ".xib":
					for _, name := range extractIBAssetReferences(content) {
						if _, ok := assetSet[name]; !ok {
							continue
						}
						usedMu.Lock()
						usedSet[name] = struct{}{}
						usedMu.Unlock()
					}
				default:
					for _, token := range extractSourceWordTokens(content) {
						if ext == ".swift" {
							if matchedAssets, ok := swiftResourceCandidates[token]; ok {
								usedMu.Lock()
								for _, name := range matchedAssets {
									usedSet[name] = struct{}{}
								}
								usedMu.Unlock()
								continue
							}
						}
						if _, ok := assetSet[token]; ok {
							usedMu.Lock()
							usedSet[token] = struct{}{}
							usedMu.Unlock()
						}
					}

					literals := extractStringLiterals(content)
					for _, lit := range literals {
						if _, ok := assetSet[lit]; ok {
							usedMu.Lock()
							usedSet[lit] = struct{}{}
							usedMu.Unlock()
						}
					}
				}

				if ext == ".swift" {
					for _, identifier := range extractSwiftResourceIdentifiers(content) {
						matchedAssets, ok := swiftResourceCandidates[identifier]
						if !ok {
							continue
						}
						usedMu.Lock()
						for _, name := range matchedAssets {
							usedSet[name] = struct{}{}
						}
						usedMu.Unlock()
					}
				}
			}
		}()
	}

	walkErr := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if d.Name() == ".git" {
				return filepath.SkipDir
			}
			rel, relErr := filepath.Rel(root, path)
			if relErr != nil {
				return relErr
			}
			if matchesAny(rel, exclude) {
				return filepath.SkipDir
			}
			return nil
		}

		rel, relErr := filepath.Rel(root, path)
		if relErr != nil {
			return relErr
		}
		if matchesAny(rel, exclude) {
			return nil
		}
		if len(include) > 0 && !matchesAny(rel, include) {
			return nil
		}
		if strings.Contains(path, ".xcassets"+string(filepath.Separator)) {
			return nil
		}

		ext := strings.ToLower(filepath.Ext(path))
		if _, ok := sourceExtensions[ext]; !ok {
			return nil
		}

		fileCh <- path
		return nil
	})
	close(fileCh)
	wg.Wait()

	select {
	case err := <-errCh:
		if err != nil {
			return nil, err
		}
	default:
	}

	if walkErr != nil {
		return nil, walkErr
	}
	return usedSet, nil
}

func extractIBAssetReferences(content string) []string {
	matches := ibAssetRefRe.FindAllStringSubmatch(content, -1)
	if len(matches) == 0 {
		return nil
	}
	out := make([]string, 0, len(matches))
	for _, m := range matches {
		if len(m) < 2 {
			continue
		}
		name := strings.TrimSpace(m[1])
		if name == "" {
			continue
		}
		out = append(out, name)
	}
	return out
}

func extractSourceWordTokens(content string) []string {
	return sourceWordTokenRe.FindAllString(content, -1)
}

func extractSwiftResourceIdentifiers(content string) []string {
	matches := swiftResourceRefRe.FindAllStringSubmatch(content, -1)
	if len(matches) == 0 {
		return nil
	}

	identifiers := make([]string, 0, len(matches))
	for _, m := range matches {
		if len(m) < 2 || m[1] == "" {
			continue
		}
		identifiers = append(identifiers, m[1])
	}
	return identifiers
}

func buildSwiftResourceCandidateIndex(assetSet map[string]struct{}, assetTypesByName map[string]map[string]struct{}) map[string][]string {
	index := make(map[string][]string, len(assetSet))
	for name := range assetSet {
		for _, candidate := range swiftResourceCandidatesForAsset(name, assetTypesByName[name]) {
			existing := index[candidate]
			if !slices.Contains(existing, name) {
				index[candidate] = append(existing, name)
			}
		}
	}
	return index
}

func swiftResourceCandidatesForAsset(assetName string, assetTypes map[string]struct{}) []string {
	candidates := []string{assetName}
	parts := strings.FieldsFunc(assetName, func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsDigit(r)
	})
	if len(parts) == 0 {
		return candidates
	}

	var b strings.Builder
	b.WriteString(strings.ToLower(parts[0]))
	for _, p := range parts[1:] {
		if p == "" {
			continue
		}
		b.WriteString(strings.ToUpper(p[:1]))
		if len(p) > 1 {
			b.WriteString(p[1:])
		}
	}

	camel := b.String()
	if camel != "" && camel != assetName {
		candidates = append(candidates, camel)
	}

	// Xcode resource APIs for image assets may drop trailing "Image".
	// Example: "somethingImage.imageset" => UIImage(resource: .something)
	if _, isImage := assetTypes["imageset"]; isImage && strings.HasSuffix(assetName, "Image") && len(assetName) > len("Image") {
		candidates = append(candidates, strings.TrimSuffix(assetName, "Image"))
	}
	// Same suffix behavior for colors and data assets.
	if _, isColor := assetTypes["colorset"]; isColor && strings.HasSuffix(assetName, "Color") && len(assetName) > len("Color") {
		candidates = append(candidates, strings.TrimSuffix(assetName, "Color"))
	}
	if _, isData := assetTypes["dataset"]; isData && strings.HasSuffix(assetName, "Data") && len(assetName) > len("Data") {
		candidates = append(candidates, strings.TrimSuffix(assetName, "Data"))
	}

	seen := make(map[string]struct{}, len(candidates)*2)
	out := make([]string, 0, len(candidates)*2)
	for _, c := range candidates {
		if c == "" {
			continue
		}
		if _, ok := seen[c]; !ok {
			seen[c] = struct{}{}
			out = append(out, c)
		}
		for _, v := range swiftIdentifierVariants(c) {
			if v == "" {
				continue
			}
			if _, ok := seen[v]; ok {
				continue
			}
			seen[v] = struct{}{}
			out = append(out, v)
		}
	}
	return out
}

func swiftIdentifierVariants(name string) []string {
	if name == "" {
		return nil
	}

	var variants []string

	// Swift resource identifiers generated for leading-digit names are prefixed
	// with "_" and capitalize first alphabetic rune after leading digits.
	leadingDigits := 0
	for leadingDigits < len(name) && name[leadingDigits] >= '0' && name[leadingDigits] <= '9' {
		leadingDigits++
	}
	if leadingDigits > 0 {
		runes := []rune(name)
		runeDigitCount := 0
		for runeDigitCount < len(runes) && unicode.IsDigit(runes[runeDigitCount]) {
			runeDigitCount++
		}
		if runeDigitCount < len(runes) && unicode.IsLetter(runes[runeDigitCount]) {
			runes[runeDigitCount] = unicode.ToUpper(runes[runeDigitCount])
		}
		variants = append(variants, "_"+string(runes))
	}

	return variants
}

func isAssetSetDir(name string) bool {
	switch filepath.Ext(name) {
	case ".imageset", ".colorset", ".dataset":
		return true
	default:
		return false
	}
}

func matchesAny(path string, patterns []string) bool {
	if len(patterns) == 0 {
		return false
	}

	normalized := filepath.ToSlash(path)
	for _, pattern := range patterns {
		p := filepath.ToSlash(strings.TrimSpace(pattern))
		if p == "" {
			continue
		}
		ok, err := filepath.Match(p, normalized)
		if err == nil && ok {
			return true
		}
		if strings.Contains(normalized, p) {
			return true
		}
	}
	return false
}
