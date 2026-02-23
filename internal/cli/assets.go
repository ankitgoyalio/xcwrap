package cli

import (
	"fmt"
	"io"
	"os"
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
	cmd.Flags().StringArrayVar(&exclude, "exclude", append([]string{}, defaultExcludedPaths...), "Exclude path globs (repeatable)")
	cmd.Flags().IntVar(&workers, "workers", defaultWorkers(), "Worker count")

	return cmd
}

type unusedResult struct {
	Command      string                      `json:"command"`
	Path         string                      `json:"path"`
	Unused       []string                    `json:"unused"`
	UnusedByFile map[string]unusedFileResult `json:"unusedByFile"`
}

type unusedFileResult struct {
	FilePath     string   `json:"filePath"`
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

			scan, err := assets.Scan(assets.Options{
				Root:    resolvedPath,
				Include: sortedInclude,
				Exclude: sortedExclude,
				Workers: workers,
			})
			if err != nil {
				return err
			}

			result := unusedResult{
				Command:      "assets unused",
				Path:         resolvedPath,
				Unused:       scan.UnusedAssets,
				UnusedByFile: buildUnusedByFilePayload(scan.UnusedByFile),
			}
			if err := renderUnusedResult(ctx.stdout, ctx.output, result); err != nil {
				return err
			}
			if len(result.Unused) > 0 {
				return unusedAssetsFoundError{}
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&path, "path", ".", "Path to scan")
	cmd.Flags().StringSliceVar(&include, "include", nil, "Include path globs")
	cmd.Flags().StringArrayVar(&exclude, "exclude", append([]string{}, defaultExcludedPaths...), "Exclude path globs (repeatable)")
	cmd.Flags().IntVar(&workers, "workers", defaultWorkers(), "Worker count")
	return cmd
}

type pruneResult struct {
	Command string   `json:"command"`
	Path    string   `json:"path"`
	Apply   bool     `json:"apply"`
	Force   bool     `json:"force"`
	Deleted []string `json:"deleted"`
	DryRun  bool     `json:"dryRun"`
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

			result := pruneResult{
				Command: "assets prune",
				Path:    resolvedPath,
				Apply:   apply,
				Force:   force,
				Deleted: []string{},
				DryRun:  !apply,
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
		_, err := fmt.Fprintf(w,
			"command\tpath\tworkers\tasset_catalogs\tasset_sets\tused_assets\tunused_assets\n%s\t%s\t%d\t%d\t%d\t%d\t%d\n",
			result.Command,
			result.Path,
			result.Workers,
			result.Summary.AssetCatalogs,
			result.Summary.AssetSets,
			result.Summary.UsedAssets,
			result.Summary.UnusedAssets,
		)
		return err
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
		if _, err := fmt.Fprintf(tw, "  Unused Count:\t%d\n", len(result.Unused)); err != nil {
			return err
		}
		if len(result.Unused) > 0 {
			if _, err := fmt.Fprintln(tw, "\nUnused Assets (Grouped By Catalog)"); err != nil {
				return err
			}
			for _, file := range sortedStringKeys(result.UnusedByFile) {
				if _, err := fmt.Fprintf(tw, "%s\n", filepath.Base(file)); err != nil {
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
		if _, err := fmt.Fprintf(w, "| command | path | unused_count |\n|---|---|---:|\n| %s | %s | %d |\n", result.Command, result.Path, len(result.Unused)); err != nil {
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
		_, err := fmt.Fprintf(w, "command\tpath\tapply\tforce\tdry_run\tdeleted_count\n%s\t%s\t%t\t%t\t%t\t%d\n", result.Command, result.Path, result.Apply, result.Force, result.DryRun, len(result.Deleted))
		return err
	case outputMarkdown:
		_, err := fmt.Fprintf(w, "| command | path | apply | force | dry_run | deleted_count |\n|---|---|---|---|---|---:|\n| %s | %s | %t | %t | %t | %d |\n", result.Command, result.Path, result.Apply, result.Force, result.DryRun, len(result.Deleted))
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
		fileName := filepath.Base(fullPath)
		entry, ok := out[fileName]
		if !ok {
			entry = unusedFileResult{
				FilePath:     fullPath,
				UnusedAssets: []string{},
			}
		}
		if fullPath < entry.FilePath {
			entry.FilePath = fullPath
		}
		for _, assetPath := range assetPaths {
			entry.UnusedAssets = append(entry.UnusedAssets, assetNameFromPath(assetPath))
		}
		out[fileName] = entry
	}

	for fileName, entry := range out {
		slices.Sort(entry.UnusedAssets)
		entry.UnusedAssets = slices.Compact(entry.UnusedAssets)
		out[fileName] = entry
	}

	return out
}
