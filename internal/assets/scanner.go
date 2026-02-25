package assets

import (
	"io/fs"
	"path"
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

var swiftResourceRefRe = regexp.MustCompile(`\b(?:(?:UI|NS)?(?:Image|Color)|(?:NS)?DataAsset)\s*\(\s*resource\s*:\s*\.([A-Za-z_][A-Za-z0-9_]*)`)
var ibImageStateRefRe = regexp.MustCompile(`\b(?:image|selectedImage|highlightedImage)\s*=\s*"([A-Za-z0-9._ -]+)"`)
var ibNamedAssetTagRefRe = regexp.MustCompile(`<(?:image|color)\b[^>]*\bname\s*=\s*"([A-Za-z0-9._ -]+)"`)
var swiftNamedImageAssetRefRe = regexp.MustCompile(`\b(?:UI|NS)?Image\s*\(\s*(?:named|name)\s*:\s*"([A-Za-z0-9._ -]+)"`)
var swiftNamedColorAssetRefRe = regexp.MustCompile(`\b(?:UI|NS)?Color\s*\(\s*(?:named|name)\s*:\s*"([A-Za-z0-9._ -]+)"`)
var swiftNamedDataAssetRefRe = regexp.MustCompile(`\b(?:NS)?DataAsset\s*\(\s*(?:named|name)\s*:\s*"([A-Za-z0-9._ -]+)"`)
var swiftUIImageAssetRefRe = regexp.MustCompile(`\bImage\s*\(\s*"([A-Za-z0-9._ -]+)"(?:\s*,[^)]*)?\)`)
var swiftUIColorAssetRefRe = regexp.MustCompile(`\bColor\s*\(\s*"([A-Za-z0-9._ -]+)"(?:\s*,[^)]*)?\)`)
var swiftResourceParameterRe = regexp.MustCompile(`(?:^|[,(])\s*([A-Za-z_][A-Za-z0-9_]*|_)\s*(?:[A-Za-z_][A-Za-z0-9_]*)?\s*:\s*(?:\[[ \t]*)?(ImageResource|ColorResource)(?:[ \t]*\])?\s*[!?]?`)
var objcImageNamedAssetRefRe = regexp.MustCompile(`\b(?:UI|NS)Image\s+imageNamed:\s*@\"([A-Za-z0-9._ -]+)\"`)
var objcImageNamedVariableRefRe = regexp.MustCompile(`\b(?:UI|NS)Image\s+imageNamed:\s*([A-Za-z_][A-Za-z0-9_]*)`)
var objcColorNamedAssetRefRe = regexp.MustCompile(`\b(?:UI|NS)Color\s+colorNamed:\s*@\"([A-Za-z0-9._ -]+)\"`)
var objcDataAssetNameRefRe = regexp.MustCompile(`\b(?:NS)?DataAsset\b[^\n\r;]*\binitWithName:\s*@\"([A-Za-z0-9._ -]+)\"`)
var swiftTypedResourceVarRe = regexp.MustCompile(`\b(?:var|let)\s+([A-Za-z_][A-Za-z0-9_]*)\s*:\s*(?:\[[ \t]*)?(?:ImageResource|ColorResource)(?:[ \t]*\])?`)
var swiftTypedResourceVarInitRe = regexp.MustCompile(`\b(?:var|let)\s+([A-Za-z_][A-Za-z0-9_]*)\s*=\s*\[\s*(?:ImageResource|ColorResource)\s*\]\s*\(\s*\)`)
var swiftTypedResourceScalarVarRe = regexp.MustCompile(`\b(?:var|let)\s+([A-Za-z_][A-Za-z0-9_]*)\s*:\s*(?:ImageResource|ColorResource)\s*[!?]?`)
var swiftResourceReturnTypeRe = regexp.MustCompile(`(?:func|var)\s+[A-Za-z_][A-Za-z0-9_]*[^{\n\r]*->\s*(?:ImageResource|ColorResource)|\bvar\s+[A-Za-z_][A-Za-z0-9_]*\s*:\s*(?:ImageResource|ColorResource)\s*\{`)
var swiftReturnEnumMemberRe = regexp.MustCompile(`\breturn\s+\.([A-Za-z_][A-Za-z0-9_]*)`)
var swiftEnumMemberRefRe = regexp.MustCompile(`\.\s*([A-Za-z_][A-Za-z0-9_]*)`)

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
	AssetType   string
}

type sourceAssetReference struct {
	Name      string
	AssetType string
}

func Scan(opts Options) (Result, error) {
	workers := opts.Workers
	if workers < 1 {
		workers = runtime.NumCPU()
	}
	if workers < 1 {
		workers = 1
	}

	assetCatalogs, _, discoveredAssets, err := collectAssets(opts.Root, opts.Include, opts.Exclude)
	if err != nil {
		return Result{}, err
	}
	usedAssetPaths, err := collectUsedAssets(opts.Root, opts.Include, opts.Exclude, discoveredAssets, workers)
	if err != nil {
		return Result{}, err
	}

	summaryNameForAsset := buildAssetSummaryNamer(discoveredAssets)
	assetNamesSet := make(map[string]struct{}, len(discoveredAssets))
	usedNames := make(map[string]struct{}, len(discoveredAssets))
	unusedNames := make(map[string]struct{}, len(discoveredAssets))
	unusedByFile := make(map[string][]string)
	for _, asset := range discoveredAssets {
		summaryName := summaryNameForAsset(asset)
		assetNamesSet[summaryName] = struct{}{}
		if _, ok := usedAssetPaths[asset.AssetPath]; ok {
			usedNames[summaryName] = struct{}{}
			continue
		}
		unusedNames[summaryName] = struct{}{}
		unusedByFile[asset.CatalogPath] = append(unusedByFile[asset.CatalogPath], asset.AssetPath)
	}
	assetNames := make([]string, 0, len(assetNamesSet))
	for name := range assetNamesSet {
		assetNames = append(assetNames, name)
	}
	slices.Sort(assetNames)
	used := make([]string, 0, len(usedNames))
	for name := range usedNames {
		used = append(used, name)
	}
	slices.Sort(used)
	unused := make([]string, 0, len(unusedNames))
	for name := range unusedNames {
		unused = append(unused, name)
	}
	slices.Sort(unused)
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

func buildAssetSummaryNamer(discoveredAssets []discoveredAsset) func(discoveredAsset) string {
	assetTypesByName := make(map[string]map[string]struct{}, len(discoveredAssets))
	for _, asset := range discoveredAssets {
		if _, ok := assetTypesByName[asset.Name]; !ok {
			assetTypesByName[asset.Name] = make(map[string]struct{}, 1)
		}
		assetTypesByName[asset.Name][asset.AssetType] = struct{}{}
	}

	hasTypeCollision := make(map[string]bool, len(assetTypesByName))
	for name, types := range assetTypesByName {
		hasTypeCollision[name] = len(types) > 1
	}

	return func(asset discoveredAsset) string {
		if hasTypeCollision[asset.Name] {
			return asset.Name + "." + asset.AssetType
		}
		return asset.Name
	}
}

func collectAssets(root string, include []string, exclude []string) (int, []string, []discoveredAsset, error) {
	assetNames := make([]string, 0, 256)
	seen := make(map[string]struct{}, 256)
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
					return nil
				}
				discoveredAssets = append(discoveredAssets, discoveredAsset{
					Name:        name,
					CatalogPath: catalogPath,
					AssetPath:   path,
					AssetType:   assetExt,
				})
				if _, ok := seen[name]; !ok {
					seen[name] = struct{}{}
					assetNames = append(assetNames, name)
				}
			}
		}

		return nil
	})
	if err != nil {
		return 0, nil, nil, err
	}

	slices.Sort(assetNames)
	slices.SortFunc(discoveredAssets, func(a, b discoveredAsset) int {
		return strings.Compare(a.AssetPath, b.AssetPath)
	})
	return assetCatalogs, assetNames, discoveredAssets, nil
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

func collectUsedAssets(root string, include []string, exclude []string, discoveredAssets []discoveredAsset, workers int) (map[string]struct{}, error) {
	fileCh := make(chan string, workers*2)
	errCh := make(chan error, 1)
	usedSet := make(map[string]struct{}, 128)
	var usedMu sync.Mutex
	assetPathsByName := make(map[string][]discoveredAsset, len(discoveredAssets))
	assetPathsByTypeAndName := make(map[string][]discoveredAsset, len(discoveredAssets))
	for _, asset := range discoveredAssets {
		assetPathsByName[asset.Name] = append(assetPathsByName[asset.Name], asset)
		typeKey := sourceAssetTypeKey(asset.Name, asset.AssetType)
		assetPathsByTypeAndName[typeKey] = append(assetPathsByTypeAndName[typeKey], asset)
	}
	swiftResourceCandidates := buildSwiftResourceCandidateIndex(discoveredAssets)
	swiftResourceLabelAssetTypes, err := collectSwiftResourceArgumentLabelAssetTypes(root, include, exclude)
	if err != nil {
		return nil, err
	}

	markUsed := func(sourcePath string, name string, assetType string) {
		var (
			candidates []discoveredAsset
			ok         bool
		)
		if assetType != "" {
			candidates, ok = assetPathsByTypeAndName[sourceAssetTypeKey(name, assetType)]
		} else {
			candidates, ok = assetPathsByName[name]
		}
		if !ok || len(candidates) == 0 {
			return
		}
		selected := selectClosestAssets(sourcePath, candidates)
		if len(selected) == 0 {
			return
		}
		usedMu.Lock()
		for _, asset := range selected {
			usedSet[asset.AssetPath] = struct{}{}
		}
		usedMu.Unlock()
	}

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
						markUsed(path, name, "")
					}
				default:
					for _, ref := range extractExplicitSourceAssetReferences(content, swiftResourceLabelAssetTypes) {
						markUsed(path, ref.Name, ref.AssetType)
					}
				}

				if ext == ".swift" {
					for _, identifier := range extractSwiftTypedResourceIdentifiers(content) {
						matchedAssets, ok := swiftResourceCandidates[identifier]
						if !ok {
							continue
						}
						usedMu.Lock()
						for _, asset := range selectClosestAssets(path, matchedAssets) {
							usedSet[asset.AssetPath] = struct{}{}
						}
						usedMu.Unlock()
					}
					for _, identifier := range extractSwiftResourceIdentifiers(content) {
						matchedAssets, ok := swiftResourceCandidates[identifier]
						if !ok {
							continue
						}
						usedMu.Lock()
						for _, asset := range selectClosestAssets(path, matchedAssets) {
							usedSet[asset.AssetPath] = struct{}{}
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
	imageStateMatches := ibImageStateRefRe.FindAllStringSubmatch(content, -1)
	namedTagMatches := ibNamedAssetTagRefRe.FindAllStringSubmatch(content, -1)
	if len(imageStateMatches) == 0 && len(namedTagMatches) == 0 {
		return nil
	}
	seen := make(map[string]struct{})
	out := make([]string, 0, len(imageStateMatches)+len(namedTagMatches))
	appendMatches := func(matches [][]string) {
		for _, m := range matches {
			if len(m) < 2 {
				continue
			}
			name := strings.TrimSpace(m[1])
			if name == "" {
				continue
			}
			if _, exists := seen[name]; exists {
				continue
			}
			seen[name] = struct{}{}
			out = append(out, name)
		}
	}
	appendMatches(imageStateMatches)
	appendMatches(namedTagMatches)
	return out
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

func extractSwiftTypedResourceIdentifiers(content string) []string {
	varMatches := swiftTypedResourceVarRe.FindAllStringSubmatch(content, -1)
	initMatches := swiftTypedResourceVarInitRe.FindAllStringSubmatch(content, -1)
	scalarVarMatches := swiftTypedResourceScalarVarRe.FindAllStringSubmatch(content, -1)
	resourceReturnBodies := extractSwiftResourceReturnBodies(content)
	if len(varMatches) == 0 && len(initMatches) == 0 && len(scalarVarMatches) == 0 && len(resourceReturnBodies) == 0 {
		return nil
	}

	mergedMatches := append([][]string{}, varMatches...)
	mergedMatches = append(mergedMatches, initMatches...)
	mergedMatches = append(mergedMatches, scalarVarMatches...)

	seenIdentifiers := make(map[string]struct{})
	identifiers := make([]string, 0, len(mergedMatches))
	for _, m := range mergedMatches {
		if len(m) < 2 || strings.TrimSpace(m[1]) == "" {
			continue
		}
		varName := m[1]
		for _, identifier := range extractEnumIdentifiersForSwiftVar(content, varName) {
			if _, exists := seenIdentifiers[identifier]; exists {
				continue
			}
			seenIdentifiers[identifier] = struct{}{}
			identifiers = append(identifiers, identifier)
		}
	}

	for _, body := range resourceReturnBodies {
		for _, m := range swiftReturnEnumMemberRe.FindAllStringSubmatch(body, -1) {
			if len(m) < 2 {
				continue
			}
			identifier := strings.TrimSpace(m[1])
			if identifier == "" {
				continue
			}
			if _, exists := seenIdentifiers[identifier]; exists {
				continue
			}
				seenIdentifiers[identifier] = struct{}{}
				identifiers = append(identifiers, identifier)
			}
		}
	return identifiers
}

func extractSwiftResourceReturnBodies(content string) []string {
	matches := swiftResourceReturnTypeRe.FindAllStringIndex(content, -1)
	if len(matches) == 0 {
		return nil
	}

	bodies := make([]string, 0, len(matches))
	for _, match := range matches {
		if len(match) < 2 {
			continue
		}
		openBrace := strings.Index(content[match[0]:], "{")
		if openBrace < 0 {
			continue
		}
		openIdx := match[0] + openBrace
		closeIdx := findMatchingBrace(content, openIdx)
		if closeIdx < 0 || closeIdx <= openIdx {
			continue
		}
		bodies = append(bodies, content[openIdx+1:closeIdx])
	}

	return bodies
}

func findMatchingBrace(content string, openIdx int) int {
	if openIdx < 0 || openIdx >= len(content) || content[openIdx] != '{' {
		return -1
	}

	depth := 0
	for i := openIdx; i < len(content); i++ {
		switch content[i] {
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				return i
			}
		}
	}

	return -1
}

func extractEnumIdentifiersForSwiftVar(content string, varName string) []string {
	seen := make(map[string]struct{})
	out := make([]string, 0, 8)

	appendIdentifier := func(identifier string) {
		if identifier == "" {
			return
		}
		if _, ok := seen[identifier]; ok {
			return
		}
		seen[identifier] = struct{}{}
		out = append(out, identifier)
	}

	assignRe := regexp.MustCompile(`\b` + regexp.QuoteMeta(varName) + `(?:\s*:\s*[^=\n\r]+)?\s*=\s*\[([^\]]*)\]`)
	for _, m := range assignRe.FindAllStringSubmatch(content, -1) {
		if len(m) < 2 {
			continue
		}
		for _, enumMatch := range swiftEnumMemberRefRe.FindAllStringSubmatch(m[1], -1) {
			if len(enumMatch) < 2 {
				continue
			}
			appendIdentifier(strings.TrimSpace(enumMatch[1]))
		}
	}

	appendRe := regexp.MustCompile(`\b` + regexp.QuoteMeta(varName) + `\s*\.append\s*\(\s*\.([A-Za-z_][A-Za-z0-9_]*)`)
	for _, m := range appendRe.FindAllStringSubmatch(content, -1) {
		if len(m) < 2 {
			continue
		}
		appendIdentifier(strings.TrimSpace(m[1]))
	}

	insertRe := regexp.MustCompile(`\b` + regexp.QuoteMeta(varName) + `\s*\.insert\s*\(\s*\.([A-Za-z_][A-Za-z0-9_]*)`)
	for _, m := range insertRe.FindAllStringSubmatch(content, -1) {
		if len(m) < 2 {
			continue
		}
		appendIdentifier(strings.TrimSpace(m[1]))
	}

	scalarAssignRe := regexp.MustCompile(`\b` + regexp.QuoteMeta(varName) + `(?:\s*:\s*[^=\n\r]+)?\s*=\s*\.([A-Za-z_][A-Za-z0-9_]*)`)
	for _, m := range scalarAssignRe.FindAllStringSubmatch(content, -1) {
		if len(m) < 2 {
			continue
		}
		appendIdentifier(strings.TrimSpace(m[1]))
	}

	return out
}

func extractExplicitSourceAssetReferences(content string, labelAssetTypes map[string]map[string]struct{}) []sourceAssetReference {
	results := make([]sourceAssetReference, 0, 16)
	seen := make(map[string]struct{})

	appendTypedMatches := func(re *regexp.Regexp, assetType string) {
		matches := re.FindAllStringSubmatch(content, -1)
		for _, m := range matches {
			if len(m) < 2 {
				continue
			}
			name := strings.TrimSpace(m[1])
			if name == "" {
				continue
			}
			key := sourceAssetTypeKey(name, assetType)
			if _, exists := seen[key]; exists {
				continue
			}
			seen[key] = struct{}{}
			results = append(results, sourceAssetReference{Name: name, AssetType: assetType})
		}
	}

	appendTypedMatches(swiftNamedImageAssetRefRe, "imageset")
	appendTypedMatches(swiftNamedColorAssetRefRe, "colorset")
	appendTypedMatches(swiftNamedDataAssetRefRe, "dataset")
	appendTypedMatches(swiftUIImageAssetRefRe, "imageset")
	appendTypedMatches(swiftUIColorAssetRefRe, "colorset")
	for _, ref := range extractSwiftLabeledResourceArgumentReferences(content, labelAssetTypes) {
		key := sourceAssetTypeKey(ref.Name, ref.AssetType)
		if _, exists := seen[key]; exists {
			continue
		}
		seen[key] = struct{}{}
		results = append(results, ref)
	}
	appendTypedMatches(objcImageNamedAssetRefRe, "imageset")
	appendTypedMatches(objcColorNamedAssetRefRe, "colorset")
	appendTypedMatches(objcDataAssetNameRefRe, "dataset")
	for _, name := range extractObjCImageNamedVariableReferences(content) {
		key := sourceAssetTypeKey(name, "imageset")
		if _, exists := seen[key]; exists {
			continue
		}
		seen[key] = struct{}{}
		results = append(results, sourceAssetReference{Name: name, AssetType: "imageset"})
	}

	return results
}

func collectSwiftResourceArgumentLabelAssetTypes(root string, include []string, exclude []string) (map[string]map[string]struct{}, error) {
	labels := make(map[string]map[string]struct{})
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
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

		if strings.Contains(path, ".xcassets"+string(filepath.Separator)) {
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
		if strings.ToLower(filepath.Ext(path)) != ".swift" {
			return nil
		}

		content, readErr := osReadFile(path)
		if readErr != nil {
			return readErr
		}
		for _, m := range swiftResourceParameterRe.FindAllStringSubmatch(content, -1) {
			if len(m) < 3 {
				continue
			}
			label := strings.TrimSpace(m[1])
			resourceType := strings.TrimSpace(m[2])
			if label == "" || label == "_" || resourceType == "" {
				continue
			}
			assetType := resourceTypeToAssetType(resourceType)
			if assetType == "" {
				continue
			}
			if _, exists := labels[label]; !exists {
				labels[label] = make(map[string]struct{}, 1)
			}
			labels[label][assetType] = struct{}{}
		}

		return nil
	})
	if err != nil {
		return nil, err
	}
	return labels, nil
}

func resourceTypeToAssetType(resourceType string) string {
	switch resourceType {
	case "ImageResource":
		return "imageset"
	case "ColorResource":
		return "colorset"
	default:
		return ""
	}
}

func extractSwiftLabeledResourceArgumentReferences(content string, labelAssetTypes map[string]map[string]struct{}) []sourceAssetReference {
	if len(labelAssetTypes) == 0 {
		return nil
	}

	labels := make([]string, 0, len(labelAssetTypes))
	for label := range labelAssetTypes {
		labels = append(labels, label)
	}
	slices.Sort(labels)

	seen := make(map[string]struct{})
	out := make([]sourceAssetReference, 0, 8)
	for _, label := range labels {
		re := regexp.MustCompile(`\b` + regexp.QuoteMeta(label) + `\s*:\s*\.([A-Za-z_][A-Za-z0-9_]*)`)
		for _, m := range re.FindAllStringSubmatch(content, -1) {
			if len(m) < 2 {
				continue
			}
			name := strings.TrimSpace(m[1])
			if name == "" {
				continue
			}
			for assetType := range labelAssetTypes[label] {
				key := sourceAssetTypeKey(name, assetType)
				if _, exists := seen[key]; exists {
					continue
				}
				seen[key] = struct{}{}
				out = append(out, sourceAssetReference{Name: name, AssetType: assetType})
			}
		}
	}
	return out
}

func sourceAssetTypeKey(name string, assetType string) string {
	return assetType + "\x00" + name
}

func extractObjCImageNamedVariableReferences(content string) []string {
	varMatches := objcImageNamedVariableRefRe.FindAllStringSubmatch(content, -1)
	if len(varMatches) == 0 {
		return nil
	}

	seenNames := make(map[string]struct{})
	names := make([]string, 0, len(varMatches))
	for _, m := range varMatches {
		if len(m) < 2 || strings.TrimSpace(m[1]) == "" {
			continue
		}
		varName := m[1]
		assignRe := regexp.MustCompile(`\b` + regexp.QuoteMeta(varName) + `\s*=\s*([^;]+);`)
		assignMatches := assignRe.FindAllStringSubmatch(content, -1)
		for _, assignMatch := range assignMatches {
			if len(assignMatch) < 2 {
				continue
			}
			for _, literal := range extractObjCStringLiterals(assignMatch[1]) {
				if _, exists := seenNames[literal]; exists {
					continue
				}
				seenNames[literal] = struct{}{}
				names = append(names, literal)
			}
		}
	}

	return names
}

func extractObjCStringLiterals(content string) []string {
	re := regexp.MustCompile(`@\"([A-Za-z0-9._ -]+)\"`)
	matches := re.FindAllStringSubmatch(content, -1)
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

func buildSwiftResourceCandidateIndex(discoveredAssets []discoveredAsset) map[string][]discoveredAsset {
	index := make(map[string][]discoveredAsset, len(discoveredAssets))
	for _, asset := range discoveredAssets {
		for _, candidate := range swiftResourceCandidatesForAsset(asset.Name, asset.AssetType) {
			existing := index[candidate]
			if !containsAssetPath(existing, asset.AssetPath) {
				index[candidate] = append(existing, asset)
			}
		}
	}
	return index
}

func swiftResourceCandidatesForAsset(assetName string, assetType string) []string {
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
	if assetType == "imageset" && strings.HasSuffix(assetName, "Image") && len(assetName) > len("Image") {
		candidates = append(candidates, strings.TrimSuffix(assetName, "Image"))
	}
	// Same suffix behavior for colors and data assets.
	if assetType == "colorset" && strings.HasSuffix(assetName, "Color") && len(assetName) > len("Color") {
		candidates = append(candidates, strings.TrimSuffix(assetName, "Color"))
	}
	if assetType == "dataset" && strings.HasSuffix(assetName, "Data") && len(assetName) > len("Data") {
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

func containsAssetPath(assets []discoveredAsset, assetPath string) bool {
	for _, asset := range assets {
		if asset.AssetPath == assetPath {
			return true
		}
	}
	return false
}

func selectClosestAssets(sourcePath string, candidates []discoveredAsset) []discoveredAsset {
	if len(candidates) <= 1 {
		return candidates
	}

	best := make([]discoveredAsset, 0, len(candidates))
	bestScore := -1
	for _, candidate := range candidates {
		score := commonPathPrefixSegments(sourcePath, candidate.CatalogPath)
		if score > bestScore {
			bestScore = score
			best = best[:0]
			best = append(best, candidate)
			continue
		}
		if score == bestScore {
			best = append(best, candidate)
		}
	}
	return best
}

func commonPathPrefixSegments(a string, b string) int {
	aParts := strings.Split(filepath.Clean(a), string(filepath.Separator))
	bParts := strings.Split(filepath.Clean(b), string(filepath.Separator))
	max := len(aParts)
	if len(bParts) < max {
		max = len(bParts)
	}

	count := 0
	for i := 0; i < max; i++ {
		if aParts[i] != bParts[i] {
			break
		}
		count++
	}
	return count
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

func matchesAny(candidatePath string, patterns []string) bool {
	if len(patterns) == 0 {
		return false
	}

	normalized := filepath.ToSlash(candidatePath)
	normalized = strings.TrimPrefix(normalized, "./")
	normalized = strings.TrimPrefix(normalized, "/")
	for _, pattern := range patterns {
		p := filepath.ToSlash(strings.TrimSpace(pattern))
		if p == "" {
			continue
		}
		p = strings.TrimPrefix(p, "./")
		p = strings.TrimPrefix(p, "/")
		if strings.HasSuffix(p, "/") {
			base := strings.TrimSuffix(p, "/")
			if normalized == base || strings.HasPrefix(normalized, base+"/") {
				return true
			}
			continue
		}
		ok, err := path.Match(p, normalized)
		if err == nil && ok {
			return true
		}
	}
	return false
}
