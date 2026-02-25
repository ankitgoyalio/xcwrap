package cli

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"runtime"
	"slices"
	"strconv"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"
	"xcwrap/internal/assets"
)

var defaultExcludedPaths = []string{
	"Pods/",
	"Carthage/",
	"SourcePackages/",
	".build/",
	"vendor/",
}

func newAssetsCommand(ctx *runContext) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "assets",
		Short: "Analyze and maintain .xcassets",
	}

	cmd.AddCommand(newAssetsScanCommand(ctx))
	cmd.AddCommand(newAssetsUnusedCommand(ctx))
	cmd.AddCommand(newAssetsPruneCommand(ctx))

	return cmd
}

type scanResult struct {
	Command string   `json:"command"`
	Path    string   `json:"path"`
	Include []string `json:"include"`
	Exclude []string `json:"exclude"`
	Workers int      `json:"workers"`
	Summary struct {
		AssetCatalogs int `json:"assetCatalogs"`
		AssetSets     int `json:"assetSets"`
		UsedAssets    int `json:"usedAssets"`
		UnusedAssets  int `json:"unusedAssets"`
	} `json:"summary"`
}

func newAssetsScanCommand(ctx *runContext) *cobra.Command {
	var path string
	var include []string
	var exclude []string
	var workers int

	cmd := &cobra.Command{
		Use:   "scan",
		Short: "Scan project assets and references",
		RunE: func(_ *cobra.Command, _ []string) error {
			resolvedPath, err := resolveScanPath(path)
			if err != nil {
				return err
			}

			if workers < 1 {
				return usageError{Message: "invalid value for --workers: must be >= 1"}
			}

			sortedInclude := append([]string{}, include...)
			sortedExclude := append([]string{}, exclude...)
			slices.Sort(sortedInclude)
			slices.Sort(sortedExclude)
			if err := validateGlobPatterns(sortedInclude, "include"); err != nil {
				return err
			}
			if err := validateGlobPatterns(sortedExclude, "exclude"); err != nil {
				return err
			}

			result := scanResult{
				Command: "assets scan",
				Path:    resolvedPath,
				Include: sortedInclude,
				Exclude: sortedExclude,
				Workers: workers,
			}
			scan, err := assets.Scan(assets.Options{
				Root:    resolvedPath,
				Include: sortedInclude,
				Exclude: sortedExclude,
				Workers: workers,
			})
			if err != nil {
				return err
			}
			result.Summary.AssetCatalogs = scan.AssetCatalogs
			result.Summary.AssetSets = len(scan.AssetNames)
			result.Summary.UsedAssets = len(scan.UsedAssets)
			result.Summary.UnusedAssets = len(scan.UnusedAssets)

			return renderScanResult(ctx.stdout, ctx.output, result)
		},
	}

	cmd.Flags().StringVar(&path, "path", ".", "Path to scan")
	cmd.Flags().StringSliceVar(&include, "include", nil, "Include path globs")
	cmd.Flags().StringSliceVar(&exclude, "exclude", append([]string{}, defaultExcludedPaths...), "Exclude path globs (repeatable)")
	cmd.Flags().IntVar(&workers, "workers", defaultWorkers(), "Worker count")

	return cmd
}

type unusedResult struct {
	Command             string                      `json:"command"`
	Path                string                      `json:"path"`
	UnusedCount         int                         `json:"unusedCount"`
	PruneCandidateCount int                         `json:"pruneCandidateCount"`
	Unused              []string                    `json:"unused"`
	UnusedByFile        map[string]unusedFileResult `json:"unusedByFile"`
}

type unusedFileResult struct {
	UnusedAssets []string `json:"unusedAssets"`
}

func newAssetsUnusedCommand(ctx *runContext) *cobra.Command {
	var path string
	var include []string
	var exclude []string
	var workers int

	cmd := &cobra.Command{
		Use:   "unused",
		Short: "Detect unused assets",
		RunE: func(_ *cobra.Command, _ []string) error {
			resolvedPath, err := resolveScanPath(path)
			if err != nil {
				return err
			}

			if workers < 1 {
				return usageError{Message: "invalid value for --workers: must be >= 1"}
			}

			sortedInclude := append([]string{}, include...)
			sortedExclude := append([]string{}, exclude...)
			slices.Sort(sortedInclude)
			slices.Sort(sortedExclude)
			if err := validateGlobPatterns(sortedInclude, "include"); err != nil {
				return err
			}
			if err := validateGlobPatterns(sortedExclude, "exclude"); err != nil {
				return err
			}

			scan, err := assets.Scan(assets.Options{
				Root:    resolvedPath,
				Include: sortedInclude,
				Exclude: sortedExclude,
				Workers: workers,
			})
			if err != nil {
				return err
			}

			pruneCandidates := collectPruneTargets(scan.UnusedByFile)
			unusedByFile := buildUnusedByFilePayload(scan.UnusedByFile)
			unusedSummary := scan.UnusedAssets
			if len(unusedSummary) == 0 && len(unusedByFile) > 0 {
				unusedSummary = flattenUnusedByFileNames(unusedByFile)
			}
			result := unusedResult{
				Command:             "assets unused",
				Path:                resolvedPath,
				UnusedCount:         len(unusedSummary),
				PruneCandidateCount: len(pruneCandidates),
				Unused:              unusedSummary,
				UnusedByFile:        unusedByFile,
			}
			if err := renderUnusedResult(ctx.stdout, ctx.output, result); err != nil {
				return err
			}
			if result.UnusedCount > 0 {
				return unusedAssetsFoundError{}
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&path, "path", ".", "Path to scan")
	cmd.Flags().StringSliceVar(&include, "include", nil, "Include path globs")
	cmd.Flags().StringSliceVar(&exclude, "exclude", append([]string{}, defaultExcludedPaths...), "Exclude path globs (repeatable)")
	cmd.Flags().IntVar(&workers, "workers", defaultWorkers(), "Worker count")
	return cmd
}

type pruneResult struct {
	Command             string   `json:"command"`
	Path                string   `json:"path"`
	Apply               bool     `json:"apply"`
	Force               bool     `json:"force"`
	UnusedCount         int      `json:"unusedCount"`
	PruneCandidateCount int      `json:"pruneCandidateCount"`
	Deleted             []string `json:"deleted"`
	DryRun              bool     `json:"dryRun"`
}

func newAssetsPruneCommand(ctx *runContext) *cobra.Command {
	var path string
	var apply bool
	var force bool

	cmd := &cobra.Command{
		Use:   "prune",
		Short: "Prune unused assets (dry-run by default)",
		RunE: func(_ *cobra.Command, _ []string) error {
			resolvedPath, err := resolveScanPath(path)
			if err != nil {
				return err
			}

			if force && !apply {
				return usageError{Message: "--force requires --apply"}
			}

			// Prune intentionally scans with conservative defaults to keep delete
			// candidates deterministic across local/CI runs.
			scan, err := assets.Scan(assets.Options{
				Root:    resolvedPath,
				Exclude: append([]string{}, defaultExcludedPaths...),
				Workers: defaultWorkers(),
			})
			if err != nil {
				return err
			}

			pruneTargets := collectPruneTargets(scan.UnusedByFile)
			if apply {
				if !force {
					if err := requireCleanGitWorkingTree(resolvedPath); err != nil {
						return err
					}
				}
				if err := deletePruneTargets(pruneTargets); err != nil {
					return err
				}
			}

			result := pruneResult{
				Command:             "assets prune",
				Path:                resolvedPath,
				Apply:               apply,
				Force:               force,
				UnusedCount:         len(scan.UnusedAssets),
				PruneCandidateCount: len(pruneTargets),
				Deleted:             pruneTargets,
				DryRun:              !apply,
			}
			return renderPruneResult(ctx.stdout, ctx.output, result)
		},
	}

	cmd.Flags().StringVar(&path, "path", ".", "Path to scan")
	cmd.Flags().BoolVar(&apply, "apply", false, "Apply deletions")
	cmd.Flags().BoolVar(&force, "force", false, "Override safety checks for --apply")
	return cmd
}

func defaultWorkers() int {
	v, ok := os.LookupEnv("XCWRAP_WORKERS")
	if ok && strings.TrimSpace(v) != "" {
		n, err := strconv.Atoi(v)
		if err == nil && n > 0 {
			return n
		}
	}

	n := runtime.NumCPU()
	if n < 1 {
		return 1
	}
	return n
}

func resolveScanPath(path string) (string, error) {
	expandedPath, err := expandTildePath(path)
	if err != nil {
		return "", err
	}

	absolutePath, err := filepath.Abs(expandedPath)
	if err != nil {
		return "", fmt.Errorf("failed to resolve path %q: %w", path, err)
	}

	info, err := os.Stat(absolutePath)
	if err != nil {
		return "", fmt.Errorf("path does not exist or is inaccessible: %s", absolutePath)
	}
	if !info.IsDir() {
		return "", fmt.Errorf("path is not a directory: %s", absolutePath)
	}

	return absolutePath, nil
}

func expandTildePath(path string) (string, error) {
	if path == "~" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("failed to resolve home directory: %w", err)
		}
		return home, nil
	}

	if strings.HasPrefix(path, "~/") {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("failed to resolve home directory: %w", err)
		}
		return filepath.Join(home, path[2:]), nil
	}

	return path, nil
}

func renderScanResult(w io.Writer, output string, result scanResult) error {
	switch output {
	case outputJSON:
		return writeJSON(w, result)
	case outputTable:
		tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
		if _, err := fmt.Fprintln(tw, "command\tpath\tworkers\tasset_catalogs\tasset_sets\tused_assets\tunused_assets"); err != nil {
			return err
		}
		if _, err := fmt.Fprintf(
			tw,
			"%s\t%s\t%d\t%d\t%d\t%d\t%d\n",
			result.Command,
			result.Path,
			result.Workers,
			result.Summary.AssetCatalogs,
			result.Summary.AssetSets,
			result.Summary.UsedAssets,
			result.Summary.UnusedAssets,
		); err != nil {
			return err
		}
		return tw.Flush()
	case outputMarkdown:
		_, err := fmt.Fprintf(w,
			"| command | path | workers | asset_catalogs | asset_sets | used_assets | unused_assets |\n|---|---|---:|---:|---:|---:|---:|\n| %s | %s | %d | %d | %d | %d | %d |\n",
			result.Command,
			result.Path,
			result.Workers,
			result.Summary.AssetCatalogs,
			result.Summary.AssetSets,
			result.Summary.UsedAssets,
			result.Summary.UnusedAssets,
		)
		return err
	default:
		return usageError{Message: fmt.Sprintf("invalid value for --output: %q (allowed: json, table, markdown)", output)}
	}
}

func renderUnusedResult(w io.Writer, output string, result unusedResult) error {
	switch output {
	case outputJSON:
		return writeJSON(w, result)
	case outputTable:
		tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
		if _, err := fmt.Fprintln(tw, "Summary"); err != nil {
			return err
		}
		if _, err := fmt.Fprintf(tw, "  Command:\t%s\n", result.Command); err != nil {
			return err
		}
		if _, err := fmt.Fprintf(tw, "  Path:\t%s\n", result.Path); err != nil {
			return err
		}
		if _, err := fmt.Fprintf(tw, "  Unused Count:\t%d\n", result.UnusedCount); err != nil {
			return err
		}
		if _, err := fmt.Fprintf(tw, "  Prune Candidate Count:\t%d\n", result.PruneCandidateCount); err != nil {
			return err
		}
		if len(result.Unused) > 0 {
			if _, err := fmt.Fprintln(tw, "\nUnused Assets (Grouped By Catalog)"); err != nil {
				return err
			}
			for _, file := range sortedStringKeys(result.UnusedByFile) {
				if _, err := fmt.Fprintf(tw, "%s\n", file); err != nil {
					return err
				}
				for _, asset := range result.UnusedByFile[file].UnusedAssets {
					if _, err := fmt.Fprintf(tw, "  -\t%s\n", asset); err != nil {
						return err
					}
				}
			}
		}
		return tw.Flush()
	case outputMarkdown:
		if _, err := fmt.Fprintf(w, "| command | path | unused_count | prune_candidate_count |\n|---|---|---:|---:|\n| %s | %s | %d | %d |\n", result.Command, result.Path, result.UnusedCount, result.PruneCandidateCount); err != nil {
			return err
		}
		if len(result.Unused) == 0 {
			return nil
		}
		if _, err := fmt.Fprintln(w, "\n| file | asset |\n|---|---|"); err != nil {
			return err
		}
		for _, file := range sortedStringKeys(result.UnusedByFile) {
			for _, asset := range result.UnusedByFile[file].UnusedAssets {
				if _, err := fmt.Fprintf(w, "| %s | %s |\n", file, asset); err != nil {
					return err
				}
			}
		}
		return nil
	default:
		return usageError{Message: fmt.Sprintf("invalid value for --output: %q (allowed: json, table, markdown)", output)}
	}
}

func renderPruneResult(w io.Writer, output string, result pruneResult) error {
	switch output {
	case outputJSON:
		return writeJSON(w, result)
	case outputTable:
		tw := tabwriter.NewWriter(w, 0, 8, 2, ' ', 0)
		if _, err := fmt.Fprintf(tw, "command\tpath\tapply\tforce\tdry_run\tunused_count\tprune_candidate_count\tdeleted_count\n%s\t%s\t%t\t%t\t%t\t%d\t%d\t%d\n", result.Command, result.Path, result.Apply, result.Force, result.DryRun, result.UnusedCount, result.PruneCandidateCount, len(result.Deleted)); err != nil {
			return err
		}
		return tw.Flush()
	case outputMarkdown:
		_, err := fmt.Fprintf(w, "| command | path | apply | force | dry_run | unused_count | prune_candidate_count | deleted_count |\n|---|---|---|---|---|---:|---:|---:|\n| %s | %s | %t | %t | %t | %d | %d | %d |\n", result.Command, result.Path, result.Apply, result.Force, result.DryRun, result.UnusedCount, result.PruneCandidateCount, len(result.Deleted))
		return err
	default:
		return usageError{Message: fmt.Sprintf("invalid value for --output: %q (allowed: json, table, markdown)", output)}
	}
}

func sortedStringKeys[T any](m map[string]T) []string {
	keys := make([]string, 0, len(m))
	for key := range m {
		keys = append(keys, key)
	}
	slices.Sort(keys)
	return keys
}

func assetNameFromPath(assetPath string) string {
	base := filepath.Base(assetPath)
	return strings.TrimSuffix(base, filepath.Ext(base))
}

func buildUnusedByFilePayload(grouped map[string][]string) map[string]unusedFileResult {
	out := make(map[string]unusedFileResult, len(grouped))
	for fullPath, assetPaths := range grouped {
		entry := unusedFileResult{UnusedAssets: unusedAssetDisplayNames(assetPaths)}
		out[fullPath] = entry
	}

	return out
}

func flattenUnusedByFileNames(grouped map[string]unusedFileResult) []string {
	if len(grouped) == 0 {
		return []string{}
	}

	all := make([]string, 0, len(grouped))
	for _, entry := range grouped {
		all = append(all, entry.UnusedAssets...)
	}
	slices.Sort(all)
	return slices.Compact(all)
}

func unusedAssetDisplayNames(assetPaths []string) []string {
	if len(assetPaths) == 0 {
		return []string{}
	}

	type assetLabel struct {
		name      string
		assetType string
	}
	labels := make([]assetLabel, 0, len(assetPaths))
	typesByName := make(map[string]map[string]struct{}, len(assetPaths))
	for _, assetPath := range assetPaths {
		base := filepath.Base(assetPath)
		assetType := strings.TrimPrefix(filepath.Ext(base), ".")
		name := assetNameFromPath(assetPath)
		labels = append(labels, assetLabel{name: name, assetType: assetType})
		if _, ok := typesByName[name]; !ok {
			typesByName[name] = make(map[string]struct{}, 1)
		}
		typesByName[name][assetType] = struct{}{}
	}

	out := make([]string, 0, len(labels))
	for _, label := range labels {
		if len(typesByName[label.name]) > 1 && label.assetType != "" {
			out = append(out, label.name+"."+label.assetType)
			continue
		}
		out = append(out, label.name)
	}

	slices.Sort(out)
	return slices.Compact(out)
}

func validateGlobPatterns(patterns []string, flagName string) error {
	for _, pattern := range patterns {
		p := filepath.ToSlash(strings.TrimSpace(pattern))
		if p == "" {
			continue
		}
		p = strings.TrimPrefix(p, "./")
		p = strings.TrimPrefix(p, "/")
		if strings.HasSuffix(p, "/") {
			continue
		}
		if _, err := path.Match(p, ""); err != nil {
			return usageError{Message: fmt.Sprintf("invalid value for --%s: %q (%v)", flagName, pattern, err)}
		}
	}

	return nil
}

func collectPruneTargets(grouped map[string][]string) []string {
	set := make(map[string]struct{})
	for _, assetPaths := range grouped {
		for _, assetPath := range assetPaths {
			if !isPrunableAssetSetPath(assetPath) {
				continue
			}
			set[assetPath] = struct{}{}
		}
	}

	out := make([]string, 0, len(set))
	for path := range set {
		out = append(out, path)
	}
	slices.Sort(out)
	return out
}

func isPrunableAssetSetPath(path string) bool {
	switch filepath.Ext(path) {
	case ".imageset", ".colorset", ".dataset":
		return true
	default:
		return false
	}
}

func deletePruneTargets(paths []string) error {
	for _, path := range paths {
		if !isPrunableAssetSetPath(path) {
			return fmt.Errorf("refusing to delete non-asset-set path: %s", path)
		}
		if err := os.RemoveAll(path); err != nil {
			return fmt.Errorf("failed to delete %s: %w", path, err)
		}
	}
	return nil
}

func requireCleanGitWorkingTree(root string) error {
	cmd := exec.Command("git", "-C", root, "status", "--porcelain")
	cmd.Env = append(os.Environ(),
		"GIT_TERMINAL_PROMPT=0",
		"LC_ALL=C",
	)
	out, err := cmd.CombinedOutput()
	if err != nil {
		message := strings.TrimSpace(string(out))
		if message == "" {
			return fmt.Errorf("failed to check git working tree: %w", err)
		}
		return fmt.Errorf("failed to check git working tree: %w: %s", err, message)
	}
	if len(bytes.TrimSpace(out)) > 0 {
		return fmt.Errorf("git working tree is not clean; commit/stash changes or rerun with --force")
	}
	return nil
}
