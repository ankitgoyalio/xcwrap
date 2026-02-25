package assets

import (
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"
)

func TestScan_FindsUsedAndUnusedAssets(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	catalog := filepath.Join(root, "App", "Assets.xcassets")
	usedSet := filepath.Join(catalog, "icon.imageset")
	unusedSet := filepath.Join(catalog, "unused.colorset")
	if err := os.MkdirAll(usedSet, 0o755); err != nil {
		t.Fatalf("mkdir used asset set: %v", err)
	}
	if err := os.MkdirAll(unusedSet, 0o755); err != nil {
		t.Fatalf("mkdir unused asset set: %v", err)
	}

	swiftPath := filepath.Join(root, "App", "ViewController.swift")
	if err := os.WriteFile(swiftPath, []byte(`let image = UIImage(named: "icon")`), 0o644); err != nil {
		t.Fatalf("write swift source: %v", err)
	}

	res, err := Scan(Options{Root: root, Workers: 2})
	if err != nil {
		t.Fatalf("scan error: %v", err)
	}

	if res.AssetCatalogs != 1 {
		t.Fatalf("expected 1 catalog, got %d", res.AssetCatalogs)
	}
	if len(res.AssetNames) != 2 {
		t.Fatalf("expected 2 assets, got %d", len(res.AssetNames))
	}
	if len(res.UsedAssets) != 1 || res.UsedAssets[0] != "icon" {
		t.Fatalf("unexpected used assets: %#v", res.UsedAssets)
	}
	if len(res.UnusedAssets) != 1 || res.UnusedAssets[0] != "unused" {
		t.Fatalf("unexpected unused assets: %#v", res.UnusedAssets)
	}
	if len(res.UnusedByFile) != 1 {
		t.Fatalf("expected one grouped file entry, got %#v", res.UnusedByFile)
	}
}

func TestScan_ReturnsErrorForInvalidUTF8SourceFile(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	catalog := filepath.Join(root, "App", "Assets.xcassets")
	if err := os.MkdirAll(filepath.Join(catalog, "icon.imageset"), 0o755); err != nil {
		t.Fatalf("mkdir asset set: %v", err)
	}

	badSwiftPath := filepath.Join(root, "App", "Bad.swift")
	if err := os.WriteFile(badSwiftPath, []byte{0xff, 0xfe, 0xfd}, 0o644); err != nil {
		t.Fatalf("write invalid swift source: %v", err)
	}

	_, err := Scan(Options{Root: root, Workers: 2})
	if err == nil {
		t.Fatalf("expected scan to fail for invalid UTF-8 source")
	}
	if !strings.Contains(err.Error(), "invalid UTF-8 encoding") {
		t.Fatalf("expected UTF-8 error, got %v", err)
	}
	if !strings.Contains(err.Error(), "Bad.swift") {
		t.Fatalf("expected error to mention source file, got %v", err)
	}
}

func TestScan_FindsSwiftUIImageResourceReference(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	catalog := filepath.Join(root, "App", "Assets.xcassets")
	usedSet := filepath.Join(catalog, "icon.imageset")
	if err := os.MkdirAll(usedSet, 0o755); err != nil {
		t.Fatalf("mkdir used asset set: %v", err)
	}

	swiftPath := filepath.Join(root, "App", "ViewController.swift")
	if err := os.WriteFile(swiftPath, []byte(`let image = UIImage(resource: .icon)`), 0o644); err != nil {
		t.Fatalf("write swift source: %v", err)
	}

	res, err := Scan(Options{Root: root, Workers: 2})
	if err != nil {
		t.Fatalf("scan error: %v", err)
	}

	if len(res.UsedAssets) != 1 || res.UsedAssets[0] != "icon" {
		t.Fatalf("unexpected used assets: %#v", res.UsedAssets)
	}
	if len(res.UnusedAssets) != 0 {
		t.Fatalf("unexpected unused assets: %#v", res.UnusedAssets)
	}
}

func TestScan_FindsSwiftUIImageResourceReference_WithImageSuffixTrim(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	catalog := filepath.Join(root, "App", "Assets.xcassets")
	usedSet := filepath.Join(catalog, "somethingImage.imageset")
	if err := os.MkdirAll(usedSet, 0o755); err != nil {
		t.Fatalf("mkdir used asset set: %v", err)
	}

	swiftPath := filepath.Join(root, "App", "ViewController.swift")
	if err := os.WriteFile(swiftPath, []byte(`let image = UIImage(resource: .something)`), 0o644); err != nil {
		t.Fatalf("write swift source: %v", err)
	}

	res, err := Scan(Options{Root: root, Workers: 2})
	if err != nil {
		t.Fatalf("scan error: %v", err)
	}

	if len(res.UsedAssets) != 1 || res.UsedAssets[0] != "somethingImage" {
		t.Fatalf("unexpected used assets: %#v", res.UsedAssets)
	}
	if len(res.UnusedAssets) != 0 {
		t.Fatalf("unexpected unused assets: %#v", res.UnusedAssets)
	}
}

func TestScan_FindsSwiftUIImageResourceReference_LowercaseImageNoTrim(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	catalog := filepath.Join(root, "App", "Assets.xcassets")
	usedSet := filepath.Join(catalog, "somethingimage.imageset")
	if err := os.MkdirAll(usedSet, 0o755); err != nil {
		t.Fatalf("mkdir used asset set: %v", err)
	}

	swiftPath := filepath.Join(root, "App", "ViewController.swift")
	if err := os.WriteFile(swiftPath, []byte(`let image = UIImage(resource: .somethingimage)`), 0o644); err != nil {
		t.Fatalf("write swift source: %v", err)
	}

	res, err := Scan(Options{Root: root, Workers: 2})
	if err != nil {
		t.Fatalf("scan error: %v", err)
	}

	if len(res.UsedAssets) != 1 || res.UsedAssets[0] != "somethingimage" {
		t.Fatalf("unexpected used assets: %#v", res.UsedAssets)
	}
	if len(res.UnusedAssets) != 0 {
		t.Fatalf("unexpected unused assets: %#v", res.UnusedAssets)
	}
}

func TestScan_FindsSwiftUIColorResourceReference_WithColorSuffixTrim(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	catalog := filepath.Join(root, "App", "Assets.xcassets")
	usedSet := filepath.Join(catalog, "brandPrimaryColor.colorset")
	if err := os.MkdirAll(usedSet, 0o755); err != nil {
		t.Fatalf("mkdir used asset set: %v", err)
	}

	swiftPath := filepath.Join(root, "App", "Theme.swift")
	if err := os.WriteFile(swiftPath, []byte(`let color = UIColor(resource: .brandPrimary)`), 0o644); err != nil {
		t.Fatalf("write swift source: %v", err)
	}

	res, err := Scan(Options{Root: root, Workers: 2})
	if err != nil {
		t.Fatalf("scan error: %v", err)
	}

	if len(res.UsedAssets) != 1 || res.UsedAssets[0] != "brandPrimaryColor" {
		t.Fatalf("unexpected used assets: %#v", res.UsedAssets)
	}
	if len(res.UnusedAssets) != 0 {
		t.Fatalf("unexpected unused assets: %#v", res.UnusedAssets)
	}
}

func TestScan_FindsSwiftUIColorResourceReference_LowercaseColorNoTrim(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	catalog := filepath.Join(root, "App", "Assets.xcassets")
	usedSet := filepath.Join(catalog, "brandprimarycolor.colorset")
	if err := os.MkdirAll(usedSet, 0o755); err != nil {
		t.Fatalf("mkdir used asset set: %v", err)
	}

	swiftPath := filepath.Join(root, "App", "Theme.swift")
	if err := os.WriteFile(swiftPath, []byte(`let color = UIColor(resource: .brandprimarycolor)`), 0o644); err != nil {
		t.Fatalf("write swift source: %v", err)
	}

	res, err := Scan(Options{Root: root, Workers: 2})
	if err != nil {
		t.Fatalf("scan error: %v", err)
	}

	if len(res.UsedAssets) != 1 || res.UsedAssets[0] != "brandprimarycolor" {
		t.Fatalf("unexpected used assets: %#v", res.UsedAssets)
	}
	if len(res.UnusedAssets) != 0 {
		t.Fatalf("unexpected unused assets: %#v", res.UnusedAssets)
	}
}

func TestScan_FindsSwiftDataAssetResourceReference_WithDataSuffixTrim(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	catalog := filepath.Join(root, "App", "Assets.xcassets")
	usedSet := filepath.Join(catalog, "configData.dataset")
	if err := os.MkdirAll(usedSet, 0o755); err != nil {
		t.Fatalf("mkdir used asset set: %v", err)
	}

	swiftPath := filepath.Join(root, "App", "Data.swift")
	if err := os.WriteFile(swiftPath, []byte(`let data = DataAsset(resource: .config)`), 0o644); err != nil {
		t.Fatalf("write swift source: %v", err)
	}

	res, err := Scan(Options{Root: root, Workers: 2})
	if err != nil {
		t.Fatalf("scan error: %v", err)
	}

	if len(res.UsedAssets) != 1 || res.UsedAssets[0] != "configData" {
		t.Fatalf("unexpected used assets: %#v", res.UsedAssets)
	}
	if len(res.UnusedAssets) != 0 {
		t.Fatalf("unexpected unused assets: %#v", res.UnusedAssets)
	}
}

func TestScan_FindsStoryboardImageReferences(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	catalog := filepath.Join(root, "App", "Assets.xcassets")
	if err := os.MkdirAll(filepath.Join(catalog, "appointmentsIcon.imageset"), 0o755); err != nil {
		t.Fatalf("mkdir asset set: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(catalog, "appointmentIcon.imageset"), 0o755); err != nil {
		t.Fatalf("mkdir asset set: %v", err)
	}

	storyboard := `<?xml version="1.0" encoding="UTF-8"?>
<document type="com.apple.InterfaceBuilder3.CocoaTouch.Storyboard.XIB">
    <scenes>
        <scene>
            <objects>
                <viewController>
                    <view key="view">
                        <subviews>
                            <imageView image="appointmentsIcon" id="a"/>
                            <button id="b">
                                <state key="normal" image="appointmentIcon"/>
                            </button>
                        </subviews>
                    </view>
                </viewController>
            </objects>
        </scene>
    </scenes>
</document>`
	if err := os.WriteFile(filepath.Join(root, "Main.storyboard"), []byte(storyboard), 0o644); err != nil {
		t.Fatalf("write storyboard: %v", err)
	}

	res, err := Scan(Options{Root: root, Workers: 2})
	if err != nil {
		t.Fatalf("scan error: %v", err)
	}
	if len(res.UnusedAssets) != 0 {
		t.Fatalf("unexpected unused assets: %#v", res.UnusedAssets)
	}
}

func TestScan_IgnoresGenericStoryboardNameAttributes(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	catalog := filepath.Join(root, "App", "Assets.xcassets")
	if err := os.MkdirAll(filepath.Join(catalog, "capability.imageset"), 0o755); err != nil {
		t.Fatalf("mkdir asset set: %v", err)
	}

	storyboard := `<?xml version="1.0" encoding="UTF-8"?>
<document type="com.apple.InterfaceBuilder3.CocoaTouch.Storyboard.XIB">
    <capabilities>
        <capability name="capability" minToolsVersion="8.0"/>
    </capabilities>
</document>`
	if err := os.WriteFile(filepath.Join(root, "Main.storyboard"), []byte(storyboard), 0o644); err != nil {
		t.Fatalf("write storyboard: %v", err)
	}

	res, err := Scan(Options{Root: root, Workers: 2})
	if err != nil {
		t.Fatalf("scan error: %v", err)
	}
	if len(res.UsedAssets) != 0 {
		t.Fatalf("expected no used assets, got %#v", res.UsedAssets)
	}
	if len(res.UnusedAssets) != 1 || res.UnusedAssets[0] != "capability" {
		t.Fatalf("expected capability to remain unused, got %#v", res.UnusedAssets)
	}
}

func TestScan_FindsIBImageTagNameReferences(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	catalog := filepath.Join(root, "App", "Assets.xcassets")
	if err := os.MkdirAll(filepath.Join(catalog, "starRatingIcon.imageset"), 0o755); err != nil {
		t.Fatalf("mkdir asset set: %v", err)
	}

	xib := `<?xml version="1.0" encoding="UTF-8"?>
<document type="com.apple.InterfaceBuilder3.CocoaTouch.XIB">
    <resources>
        <image name="starRatingIcon" width="16" height="16"/>
    </resources>
</document>`
	if err := os.WriteFile(filepath.Join(root, "RatingView.xib"), []byte(xib), 0o644); err != nil {
		t.Fatalf("write xib: %v", err)
	}

	res, err := Scan(Options{Root: root, Workers: 2})
	if err != nil {
		t.Fatalf("scan error: %v", err)
	}
	if len(res.UnusedAssets) != 0 {
		t.Fatalf("unexpected unused assets: %#v", res.UnusedAssets)
	}
}

func TestScan_FindsIBColorTagNameReferences(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	catalog := filepath.Join(root, "App", "Assets.xcassets")
	if err := os.MkdirAll(filepath.Join(catalog, "notificationBackgroundViewColor.colorset"), 0o755); err != nil {
		t.Fatalf("mkdir asset set: %v", err)
	}

	storyboard := `<?xml version="1.0" encoding="UTF-8"?>
<document type="com.apple.InterfaceBuilder3.CocoaTouch.Storyboard.XIB">
    <scenes>
        <scene>
            <objects>
                <viewController>
                    <view key="view">
                        <color key="backgroundColor" name="notificationBackgroundViewColor"/>
                    </view>
                </viewController>
            </objects>
        </scene>
    </scenes>
</document>`
	if err := os.WriteFile(filepath.Join(root, "Main.storyboard"), []byte(storyboard), 0o644); err != nil {
		t.Fatalf("write storyboard: %v", err)
	}

	res, err := Scan(Options{Root: root, Workers: 2})
	if err != nil {
		t.Fatalf("scan error: %v", err)
	}
	if len(res.UnusedAssets) != 0 {
		t.Fatalf("unexpected unused assets: %#v", res.UnusedAssets)
	}
}

func TestScan_IBColorReference_DoesNotMarkSameNameImageAssetUsed(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	catalog := filepath.Join(root, "App", "Assets.xcassets")
	imageAssetPath := filepath.Join(catalog, "logo.imageset")
	colorAssetPath := filepath.Join(catalog, "logo.colorset")
	if err := os.MkdirAll(imageAssetPath, 0o755); err != nil {
		t.Fatalf("mkdir image asset set: %v", err)
	}
	if err := os.MkdirAll(colorAssetPath, 0o755); err != nil {
		t.Fatalf("mkdir color asset set: %v", err)
	}

	storyboard := `<?xml version="1.0" encoding="UTF-8"?>
<document type="com.apple.InterfaceBuilder3.CocoaTouch.Storyboard.XIB">
    <scenes>
        <scene>
            <objects>
                <viewController>
                    <view key="view">
                        <color key="backgroundColor" name="logo"/>
                    </view>
                </viewController>
            </objects>
        </scene>
    </scenes>
</document>`
	if err := os.WriteFile(filepath.Join(root, "Main.storyboard"), []byte(storyboard), 0o644); err != nil {
		t.Fatalf("write storyboard: %v", err)
	}

	res, err := Scan(Options{Root: root, Workers: 2})
	if err != nil {
		t.Fatalf("scan error: %v", err)
	}
	if len(res.AssetNames) != 2 || res.AssetNames[0] != "logo.colorset" || res.AssetNames[1] != "logo.imageset" {
		t.Fatalf("unexpected asset names: %#v", res.AssetNames)
	}
	if len(res.UsedAssets) != 1 || res.UsedAssets[0] != "logo.colorset" {
		t.Fatalf("unexpected used assets: %#v", res.UsedAssets)
	}
	if len(res.UnusedAssets) != 1 || res.UnusedAssets[0] != "logo.imageset" {
		t.Fatalf("unexpected unused assets: %#v", res.UnusedAssets)
	}
	unusedInCatalog, ok := res.UnusedByFile[catalog]
	if !ok {
		t.Fatalf("expected catalog %q in unusedByFile, got %#v", catalog, res.UnusedByFile)
	}
	if len(unusedInCatalog) != 1 || unusedInCatalog[0] != imageAssetPath {
		t.Fatalf("expected only imageset unused path %q, got %#v", imageAssetPath, unusedInCatalog)
	}
}

func TestScan_FindsSwiftUIImageResourceReference_WithIdentifier(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	catalog := filepath.Join(root, "App", "Assets.xcassets")
	if err := os.MkdirAll(filepath.Join(catalog, "betaSubscriptionIcon.imageset"), 0o755); err != nil {
		t.Fatalf("mkdir asset set: %v", err)
	}
	swiftPath := filepath.Join(root, "App", "Subscription.swift")
	if err := os.WriteFile(swiftPath, []byte("let image = UIImage(resource: .betaSubscriptionIcon)"), 0o644); err != nil {
		t.Fatalf("write swift source: %v", err)
	}

	res, err := Scan(Options{Root: root, Workers: 2})
	if err != nil {
		t.Fatalf("scan error: %v", err)
	}
	if len(res.UnusedAssets) != 0 {
		t.Fatalf("unexpected unused assets: %#v", res.UnusedAssets)
	}
}

func TestScan_FindsSwiftResourceReference_ForLeadingDigitAssetName(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	catalog := filepath.Join(root, "App", "Assets.xcassets")
	if err := os.MkdirAll(filepath.Join(catalog, "2checkoutPaymentIcon.imageset"), 0o755); err != nil {
		t.Fatalf("mkdir asset set: %v", err)
	}
	swiftPath := filepath.Join(root, "App", "PaymentGatewaySelectionView.swift")
	if err := os.WriteFile(swiftPath, []byte("let image = UIImage(resource: ._2CheckoutPaymentIcon)"), 0o644); err != nil {
		t.Fatalf("write swift source: %v", err)
	}

	res, err := Scan(Options{Root: root, Workers: 2})
	if err != nil {
		t.Fatalf("scan error: %v", err)
	}
	if len(res.UnusedAssets) != 0 {
		t.Fatalf("unexpected unused assets: %#v", res.UnusedAssets)
	}
}

func TestScan_FindsSwiftUIImageResourceReference_ForDashedAssetNameCamelCase(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	catalog := filepath.Join(root, "App", "Assets.xcassets")
	if err := os.MkdirAll(filepath.Join(catalog, "FSM-Onboarding-2.imageset"), 0o755); err != nil {
		t.Fatalf("mkdir asset set: %v", err)
	}
	swiftPath := filepath.Join(root, "App", "PageViewController.swift")
	if err := os.WriteFile(swiftPath, []byte("let image = UIImage(resource: .fsmOnboarding2)"), 0o644); err != nil {
		t.Fatalf("write swift source: %v", err)
	}

	res, err := Scan(Options{Root: root, Workers: 2})
	if err != nil {
		t.Fatalf("scan error: %v", err)
	}
	if len(res.UnusedAssets) != 0 {
		t.Fatalf("unexpected unused assets: %#v", res.UnusedAssets)
	}
}

func TestScan_DoesNotMarkAssetUsedFromGenericTokenOrStringLiteral(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	catalog := filepath.Join(root, "App", "Assets.xcassets")
	if err := os.MkdirAll(filepath.Join(catalog, "orphanIcon.imageset"), 0o755); err != nil {
		t.Fatalf("mkdir asset set: %v", err)
	}

	swiftPath := filepath.Join(root, "App", "Noise.swift")
	content := `// orphanIcon appears as an unrelated token and string
let value = "orphanIcon"
let orphanIcon = "debug-only"
`
	if err := os.WriteFile(swiftPath, []byte(content), 0o644); err != nil {
		t.Fatalf("write swift source: %v", err)
	}

	res, err := Scan(Options{Root: root, Workers: 2})
	if err != nil {
		t.Fatalf("scan error: %v", err)
	}

	if len(res.UsedAssets) != 0 {
		t.Fatalf("expected no used assets, got %#v", res.UsedAssets)
	}
	if len(res.UnusedAssets) != 1 || res.UnusedAssets[0] != "orphanIcon" {
		t.Fatalf("expected orphanIcon to remain unused, got %#v", res.UnusedAssets)
	}
}

func TestScan_FindsSwiftUIImageAndColorStringReference_WithBundleArgument(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	catalog := filepath.Join(root, "App", "Assets.xcassets")
	if err := os.MkdirAll(filepath.Join(catalog, "hero.imageset"), 0o755); err != nil {
		t.Fatalf("mkdir image asset set: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(catalog, "brand.colorset"), 0o755); err != nil {
		t.Fatalf("mkdir color asset set: %v", err)
	}

	swiftPath := filepath.Join(root, "App", "View.swift")
	content := `import SwiftUI
let image = Image("hero", bundle: .main)
let color = Color("brand", bundle: .main)
`
	if err := os.WriteFile(swiftPath, []byte(content), 0o644); err != nil {
		t.Fatalf("write swift source: %v", err)
	}

	res, err := Scan(Options{Root: root, Workers: 2})
	if err != nil {
		t.Fatalf("scan error: %v", err)
	}
	if len(res.UnusedAssets) != 0 {
		t.Fatalf("unexpected unused assets: %#v", res.UnusedAssets)
	}
}

func TestScan_FindsSwiftTypedImageResourceIdentifiers(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	catalog := filepath.Join(root, "App", "Assets.xcassets")
	if err := os.MkdirAll(filepath.Join(catalog, "dropDownAttach.imageset"), 0o755); err != nil {
		t.Fatalf("mkdir image asset set: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(catalog, "unused.imageset"), 0o755); err != nil {
		t.Fatalf("mkdir image asset set: %v", err)
	}

	swiftPath := filepath.Join(root, "App", "View.swift")
	content := `var icons: [ImageResource] = [.dropDownEdit]
icons.append(.dropDownAttach)
let image = UIImage(resource: icons[0])
`
	if err := os.WriteFile(swiftPath, []byte(content), 0o644); err != nil {
		t.Fatalf("write swift source: %v", err)
	}

	res, err := Scan(Options{Root: root, Workers: 2})
	if err != nil {
		t.Fatalf("scan error: %v", err)
	}
	if len(res.UsedAssets) != 1 || res.UsedAssets[0] != "dropDownAttach" {
		t.Fatalf("unexpected used assets: %#v", res.UsedAssets)
	}
	if len(res.UnusedAssets) != 1 || res.UnusedAssets[0] != "unused" {
		t.Fatalf("unexpected unused assets: %#v", res.UnusedAssets)
	}
}

func TestScan_FindsSwiftTypedImageResourceIdentifiers_WithInferredArrayType(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	catalog := filepath.Join(root, "App", "Assets.xcassets")
	if err := os.MkdirAll(filepath.Join(catalog, "dropDownAttachment.imageset"), 0o755); err != nil {
		t.Fatalf("mkdir image asset set: %v", err)
	}

	swiftPath := filepath.Join(root, "App", "View.swift")
	content := `private var dropDownIcons = [ImageResource]()
dropDownIcons = [.dropDownAttachment]
let image = UIImage(resource: dropDownIcons[0])
`
	if err := os.WriteFile(swiftPath, []byte(content), 0o644); err != nil {
		t.Fatalf("write swift source: %v", err)
	}

	res, err := Scan(Options{Root: root, Workers: 2})
	if err != nil {
		t.Fatalf("scan error: %v", err)
	}
	if len(res.UnusedAssets) != 0 {
		t.Fatalf("unexpected unused assets: %#v", res.UnusedAssets)
	}
}

func TestScan_FindsSwiftTypedImageResourceIdentifiers_WithInlineTypedAssignment(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	catalog := filepath.Join(root, "App", "Assets.xcassets")
	if err := os.MkdirAll(filepath.Join(catalog, "dropDownCall.imageset"), 0o755); err != nil {
		t.Fatalf("mkdir image asset set: %v", err)
	}

	swiftPath := filepath.Join(root, "App", "View.swift")
	content := `private let dropDownIconsValues: [ImageResource] = [.dropDownCall, .dropDownEmail]
private var dropDownIcons: [ImageResource] = [.dropDownCall]
let image = UIImage(resource: dropDownIconsValues[0])
`
	if err := os.WriteFile(swiftPath, []byte(content), 0o644); err != nil {
		t.Fatalf("write swift source: %v", err)
	}

	res, err := Scan(Options{Root: root, Workers: 2})
	if err != nil {
		t.Fatalf("scan error: %v", err)
	}
	if len(res.UnusedAssets) != 0 {
		t.Fatalf("unexpected unused assets: %#v", res.UnusedAssets)
	}
}

func TestScan_FindsSwiftTypedImageResourceIdentifiers_FromTypedReturn(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	catalog := filepath.Join(root, "App", "Assets.xcassets")
	if err := os.MkdirAll(filepath.Join(catalog, "betaSubscriptionIcon.imageset"), 0o755); err != nil {
		t.Fatalf("mkdir image asset set: %v", err)
	}

	swiftPath := filepath.Join(root, "App", "Subscription.swift")
	content := `func getCurrentEditionIcon() -> ImageResource {
    return .betaSubscriptionIcon
}
`
	if err := os.WriteFile(swiftPath, []byte(content), 0o644); err != nil {
		t.Fatalf("write swift source: %v", err)
	}

	res, err := Scan(Options{Root: root, Workers: 2})
	if err != nil {
		t.Fatalf("scan error: %v", err)
	}
	if len(res.UnusedAssets) != 0 {
		t.Fatalf("unexpected unused assets: %#v", res.UnusedAssets)
	}
}

func TestScan_ScopesSwiftTypedReturnEnumMembersToResourceContexts(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	catalog := filepath.Join(root, "App", "Assets.xcassets")
	if err := os.MkdirAll(filepath.Join(catalog, "betaSubscriptionIcon.imageset"), 0o755); err != nil {
		t.Fatalf("mkdir used image asset set: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(catalog, "notAnAsset.imageset"), 0o755); err != nil {
		t.Fatalf("mkdir unrelated image asset set: %v", err)
	}

	swiftPath := filepath.Join(root, "App", "Subscription.swift")
	content := `func getCurrentEditionIcon() -> ImageResource {
    return .betaSubscriptionIcon
}

enum UnrelatedState {
    case notAnAsset
}

func currentState() -> UnrelatedState {
    return .notAnAsset
}
`
	if err := os.WriteFile(swiftPath, []byte(content), 0o644); err != nil {
		t.Fatalf("write swift source: %v", err)
	}

	res, err := Scan(Options{Root: root, Workers: 2})
	if err != nil {
		t.Fatalf("scan error: %v", err)
	}
	if len(res.UsedAssets) != 1 || res.UsedAssets[0] != "betaSubscriptionIcon" {
		t.Fatalf("unexpected used assets: %#v", res.UsedAssets)
	}
	if len(res.UnusedAssets) != 1 || res.UnusedAssets[0] != "notAnAsset" {
		t.Fatalf("unexpected unused assets: %#v", res.UnusedAssets)
	}
}

func TestScan_ExplicitImageNamedReference_DoesNotMarkSameNameColorAssetUsed(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	catalog := filepath.Join(root, "App", "Assets.xcassets")
	imageAssetPath := filepath.Join(catalog, "logo.imageset")
	colorAssetPath := filepath.Join(catalog, "logo.colorset")
	if err := os.MkdirAll(imageAssetPath, 0o755); err != nil {
		t.Fatalf("mkdir image asset set: %v", err)
	}
	if err := os.MkdirAll(colorAssetPath, 0o755); err != nil {
		t.Fatalf("mkdir color asset set: %v", err)
	}

	swiftPath := filepath.Join(root, "App", "Feature.swift")
	content := `let _ = UIImage(named: "logo")`
	if err := os.WriteFile(swiftPath, []byte(content), 0o644); err != nil {
		t.Fatalf("write swift source: %v", err)
	}

	res, err := Scan(Options{Root: root, Workers: 2})
	if err != nil {
		t.Fatalf("scan error: %v", err)
	}
	if len(res.AssetNames) != 2 || res.AssetNames[0] != "logo.colorset" || res.AssetNames[1] != "logo.imageset" {
		t.Fatalf("unexpected asset names: %#v", res.AssetNames)
	}
	if len(res.UsedAssets) != 1 || res.UsedAssets[0] != "logo.imageset" {
		t.Fatalf("unexpected used assets: %#v", res.UsedAssets)
	}
	if len(res.UnusedAssets) != 1 || res.UnusedAssets[0] != "logo.colorset" {
		t.Fatalf("unexpected unused assets: %#v", res.UnusedAssets)
	}
	unusedInCatalog, ok := res.UnusedByFile[catalog]
	if !ok {
		t.Fatalf("expected catalog %q in unusedByFile, got %#v", catalog, res.UnusedByFile)
	}
	if len(unusedInCatalog) != 1 || unusedInCatalog[0] != colorAssetPath {
		t.Fatalf("expected only colorset unused path %q, got %#v", colorAssetPath, unusedInCatalog)
	}
}

func TestScan_FindsSwiftTypedImageResourceIdentifiers_FromScalarAssignment(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	catalog := filepath.Join(root, "App", "Assets.xcassets")
	if err := os.MkdirAll(filepath.Join(catalog, "billingAddressIcon.imageset"), 0o755); err != nil {
		t.Fatalf("mkdir image asset set: %v", err)
	}

	swiftPath := filepath.Join(root, "App", "CommonUtil.swift")
	content := `func getIcon() -> ImageResource {
    var icon: ImageResource = .fsmOnboarding1
    icon = .billingAddressIcon
    return icon
}
`
	if err := os.WriteFile(swiftPath, []byte(content), 0o644); err != nil {
		t.Fatalf("write swift source: %v", err)
	}

	res, err := Scan(Options{Root: root, Workers: 2})
	if err != nil {
		t.Fatalf("scan error: %v", err)
	}
	if len(res.UnusedAssets) != 0 {
		t.Fatalf("unexpected unused assets: %#v", res.UnusedAssets)
	}
}

func TestScan_DoesNotInferAssetUsageFromLabeledResourceLikeMembers(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	catalog := filepath.Join(root, "App", "Assets.xcassets")
	if err := os.MkdirAll(filepath.Join(catalog, "dashboardTSCountIcon.imageset"), 0o755); err != nil {
		t.Fatalf("mkdir image asset set: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(catalog, "emptyDashboardIllustration.imageset"), 0o755); err != nil {
		t.Fatalf("mkdir image asset set: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(catalog, "dropDownMarkAsSent.imageset"), 0o755); err != nil {
		t.Fatalf("mkdir image asset set: %v", err)
	}

	swiftPath := filepath.Join(root, "App", "Home.swift")
	content := `timeSheetCountView.setData(icon: .dashboardTSCountIcon, fieldName: "x", value: "y")
emptyListIllustrationView.setData(illustration: .emptyDashboardIllustration, title: "x", body: "y")
let action = makeAction(title: "x", imageResource: .dropDownMarkAsSent) { _ in }
`
	if err := os.WriteFile(swiftPath, []byte(content), 0o644); err != nil {
		t.Fatalf("write swift source: %v", err)
	}

	res, err := Scan(Options{Root: root, Workers: 2})
	if err != nil {
		t.Fatalf("scan error: %v", err)
	}
	if len(res.UsedAssets) != 0 {
		t.Fatalf("expected no used assets, got %#v", res.UsedAssets)
	}
	if len(res.UnusedAssets) != 3 {
		t.Fatalf("expected all labeled-member assets to remain unused, got %#v", res.UnusedAssets)
	}
}

func TestScan_DoesNotTreatLabeledEnumMembersAsAssetReferences(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	catalog := filepath.Join(root, "App", "Assets.xcassets")
	if err := os.MkdirAll(filepath.Join(catalog, "warning.imageset"), 0o755); err != nil {
		t.Fatalf("mkdir image asset set: %v", err)
	}

	swiftPath := filepath.Join(root, "App", "Alert.swift")
	content := `enum AlertIcon {
    case warning
}

func render(icon: AlertIcon) {}

render(icon: .warning)
`
	if err := os.WriteFile(swiftPath, []byte(content), 0o644); err != nil {
		t.Fatalf("write swift source: %v", err)
	}

	res, err := Scan(Options{Root: root, Workers: 2})
	if err != nil {
		t.Fatalf("scan error: %v", err)
	}
	if len(res.UsedAssets) != 0 {
		t.Fatalf("expected no used assets, got %#v", res.UsedAssets)
	}
	if len(res.UnusedAssets) != 1 || res.UnusedAssets[0] != "warning" {
		t.Fatalf("expected warning to remain unused, got %#v", res.UnusedAssets)
	}
}

func TestScan_FindsLabeledMembersWhenLabelIsTypedAsImageResource(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	catalog := filepath.Join(root, "App", "Assets.xcassets")
	if err := os.MkdirAll(filepath.Join(catalog, "dashboardTSCountIcon.imageset"), 0o755); err != nil {
		t.Fatalf("mkdir image asset set: %v", err)
	}

	componentPath := filepath.Join(root, "App", "DashboardInfoView.swift")
	componentContent := `struct DashboardInfoView {
    func setData(icon: ImageResource, fieldName: String, value: String) {}
}`
	if err := os.WriteFile(componentPath, []byte(componentContent), 0o644); err != nil {
		t.Fatalf("write component: %v", err)
	}

	usagePath := filepath.Join(root, "App", "Home.swift")
	usageContent := `func render(view: DashboardInfoView) {
    view.setData(icon: .dashboardTSCountIcon, fieldName: "x", value: "y")
}`
	if err := os.WriteFile(usagePath, []byte(usageContent), 0o644); err != nil {
		t.Fatalf("write usage: %v", err)
	}

	res, err := Scan(Options{Root: root, Workers: 2})
	if err != nil {
		t.Fatalf("scan error: %v", err)
	}
	if len(res.UnusedAssets) != 0 {
		t.Fatalf("unexpected unused assets: %#v", res.UnusedAssets)
	}
}

func TestScan_FindsObjCImageNamedVariableReferences(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	catalog := filepath.Join(root, "App", "Assets.xcassets")
	if err := os.MkdirAll(filepath.Join(catalog, "ipad_key_remove.imageset"), 0o755); err != nil {
		t.Fatalf("mkdir image asset set: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(catalog, "iphone_key_remove.imageset"), 0o755); err != nil {
		t.Fatalf("mkdir image asset set: %v", err)
	}

	objcPath := filepath.Join(root, "App", "VTPasscodeUtilConstants.m")
	content := `+ (UIImage *)removeKeyImage {
    NSString *removeKey = isCurrentDeviceTypeiPad() ? @"ipad_key_remove" : @"iphone_key_remove";
    return [UIImage imageNamed:removeKey];
}`
	if err := os.WriteFile(objcPath, []byte(content), 0o644); err != nil {
		t.Fatalf("write objc source: %v", err)
	}

	res, err := Scan(Options{Root: root, Workers: 2})
	if err != nil {
		t.Fatalf("scan error: %v", err)
	}
	if len(res.UnusedAssets) != 0 {
		t.Fatalf("unexpected unused assets: %#v", res.UnusedAssets)
	}
}

func TestScan_ReadErrorDoesNotDeadlock(t *testing.T) {
	t.Parallel()
	if runtime.GOOS == "windows" {
		t.Skip("permission model differs on windows")
	}

	root := t.TempDir()
	catalog := filepath.Join(root, "Assets.xcassets")
	if err := os.MkdirAll(filepath.Join(catalog, "used.imageset"), 0o755); err != nil {
		t.Fatalf("mkdir asset set: %v", err)
	}

	unreadable := filepath.Join(root, "000_unreadable.swift")
	if err := os.WriteFile(unreadable, []byte(`let _ = UIImage(named: "used")`), 0o644); err != nil {
		t.Fatalf("write unreadable source: %v", err)
	}
	if err := os.Chmod(unreadable, 0o000); err != nil {
		t.Fatalf("chmod unreadable source: %v", err)
	}
	t.Cleanup(func() {
		_ = os.Chmod(unreadable, 0o644)
	})

	for i := 0; i < 32; i++ {
		path := filepath.Join(root, "src", "file_"+strconv.Itoa(i)+".swift")
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatalf("mkdir source dir: %v", err)
		}
		if err := os.WriteFile(path, []byte("let value = 1"), 0o644); err != nil {
			t.Fatalf("write source file: %v", err)
		}
	}

	done := make(chan error, 1)
	go func() {
		_, err := Scan(Options{Root: root, Workers: 1})
		done <- err
	}()

	select {
	case err := <-done:
		if err == nil {
			t.Fatalf("expected read error, got nil")
		}
	case <-time.After(3 * time.Second):
		t.Fatal("scan deadlocked after file read error")
	}
}

func TestScan_DuplicateAssetNamesAcrossCatalogs(t *testing.T) {
	t.Parallel()
	root := t.TempDir()

	moduleACatalog := filepath.Join(root, "Modules", "ModuleA", "Assets.xcassets")
	moduleBCatalog := filepath.Join(root, "Modules", "ModuleB", "Assets.xcassets")
	if err := os.MkdirAll(filepath.Join(moduleACatalog, "icon.imageset"), 0o755); err != nil {
		t.Fatalf("mkdir module a asset: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(moduleBCatalog, "icon.imageset"), 0o755); err != nil {
		t.Fatalf("mkdir module b asset: %v", err)
	}

	sourcePath := filepath.Join(root, "Modules", "ModuleA", "Feature.swift")
	if err := os.WriteFile(sourcePath, []byte(`let _ = UIImage(named: "icon")`), 0o644); err != nil {
		t.Fatalf("write source: %v", err)
	}

	res, err := Scan(Options{Root: root, Workers: 2})
	if err != nil {
		t.Fatalf("scan error: %v", err)
	}

	if len(res.UsedAssets) != 1 || res.UsedAssets[0] != "icon" {
		t.Fatalf("expected icon to be marked used in summary, got %#v", res.UsedAssets)
	}
	if len(res.UnusedAssets) != 0 {
		t.Fatalf("expected no overlapping summary entries in unused list, got %#v", res.UnusedAssets)
	}
	if len(res.UnusedByFile) != 1 {
		t.Fatalf("expected one unused catalog, got %#v", res.UnusedByFile)
	}
	unusedAssetsInModuleB, ok := res.UnusedByFile[moduleBCatalog]
	if !ok {
		t.Fatalf("expected module B catalog key %q in unusedByFile, got %#v", moduleBCatalog, res.UnusedByFile)
	}
	if len(unusedAssetsInModuleB) != 1 || filepath.Base(unusedAssetsInModuleB[0]) != "icon.imageset" {
		t.Fatalf("unexpected module B unused entries: %#v", unusedAssetsInModuleB)
	}
	if _, exists := res.UnusedByFile[moduleACatalog]; exists {
		t.Fatalf("expected module A catalog to be used, but found in unusedByFile: %#v", res.UnusedByFile[moduleACatalog])
	}
}

func TestScan_DuplicateAssetNamesAcrossCatalogs_DoesNotEmitSameSummaryAsUsedAndUnused(t *testing.T) {
	t.Parallel()
	root := t.TempDir()

	moduleACatalog := filepath.Join(root, "Modules", "ModuleA", "Assets.xcassets")
	moduleBCatalog := filepath.Join(root, "Modules", "ModuleB", "Assets.xcassets")
	if err := os.MkdirAll(filepath.Join(moduleACatalog, "icon.imageset"), 0o755); err != nil {
		t.Fatalf("mkdir module a asset: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(moduleBCatalog, "icon.imageset"), 0o755); err != nil {
		t.Fatalf("mkdir module b asset: %v", err)
	}

	sourcePath := filepath.Join(root, "Modules", "ModuleA", "Feature.swift")
	if err := os.WriteFile(sourcePath, []byte(`let _ = UIImage(named: "icon")`), 0o644); err != nil {
		t.Fatalf("write source: %v", err)
	}

	res, err := Scan(Options{Root: root, Workers: 2})
	if err != nil {
		t.Fatalf("scan error: %v", err)
	}

	if len(res.UsedAssets) != 1 || res.UsedAssets[0] != "icon" {
		t.Fatalf("expected icon in used summary, got %#v", res.UsedAssets)
	}
	if len(res.UnusedAssets) != 0 {
		t.Fatalf("expected no overlapping names in unused summary, got %#v", res.UnusedAssets)
	}
}

func TestScan_IgnoresAssetSetsOutsideCatalogs(t *testing.T) {
	t.Parallel()
	root := t.TempDir()

	catalog := filepath.Join(root, "App", "Assets.xcassets")
	if err := os.MkdirAll(filepath.Join(catalog, "inCatalog.imageset"), 0o755); err != nil {
		t.Fatalf("mkdir in-catalog asset: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(root, "Fixtures", "outOfCatalog.imageset"), 0o755); err != nil {
		t.Fatalf("mkdir out-of-catalog asset: %v", err)
	}

	sourcePath := filepath.Join(root, "App", "Feature.swift")
	if err := os.WriteFile(sourcePath, []byte(`let _ = UIImage(named: "inCatalog")`), 0o644); err != nil {
		t.Fatalf("write source: %v", err)
	}

	res, err := Scan(Options{Root: root, Workers: 2})
	if err != nil {
		t.Fatalf("scan error: %v", err)
	}

	if res.AssetCatalogs != 1 {
		t.Fatalf("expected 1 catalog, got %d", res.AssetCatalogs)
	}
	if len(res.AssetNames) != 1 || res.AssetNames[0] != "inCatalog" {
		t.Fatalf("unexpected asset names: %#v", res.AssetNames)
	}
	if len(res.UsedAssets) != 1 || res.UsedAssets[0] != "inCatalog" {
		t.Fatalf("unexpected used assets: %#v", res.UsedAssets)
	}
	if len(res.UnusedAssets) != 0 {
		t.Fatalf("unexpected unused assets: %#v", res.UnusedAssets)
	}
	if len(res.UnusedByFile) != 0 {
		t.Fatalf("expected no unused grouped entries, got %#v", res.UnusedByFile)
	}
}

func TestMatchesAny_GlobPatternMatchesExpectedPath(t *testing.T) {
	t.Parallel()
	if !matchesAny("App/Main.swift", []string{"App/*.swift"}) {
		t.Fatalf("expected glob to match path")
	}
}

func TestMatchesAny_GlobstarPatternMatchesNestedPath(t *testing.T) {
	t.Parallel()
	if !matchesAny("Sources/App/Features/Home/View.swift", []string{"Sources/**/*.swift"}) {
		t.Fatalf("expected globstar pattern to match nested path")
	}
}

func TestMatchesAny_DirectoryPatternMatchesSubtree(t *testing.T) {
	t.Parallel()
	if !matchesAny("ExternalLib/Assets.xcassets/icon.imageset", []string{"ExternalLib/"}) {
		t.Fatalf("expected directory pattern to match subtree")
	}
}

func TestMatchesAny_DoesNotUseSubstringFallback(t *testing.T) {
	t.Parallel()
	if matchesAny("MyExternalLib/Assets.xcassets/icon.imageset", []string{"ExternalLib/"}) {
		t.Fatalf("did not expect substring overlap to match")
	}
}

func TestSwiftResourceCandidatesForAsset_HandlesMultibyteCamelCaseParts(t *testing.T) {
	t.Parallel()
	candidates := swiftResourceCandidatesForAsset("primary_äpfel", "imageset")

	if !slices.Contains(candidates, "primaryÄpfel") {
		t.Fatalf("expected utf-8 aware camel candidate, got %#v", candidates)
	}
}
