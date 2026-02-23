package assets

import (
	"os"
	"path/filepath"
	"testing"
)

func TestScan_FindsUsedAndUnusedAssets(t *testing.T) {
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

func TestScan_FindsSwiftUIImageResourceReference(t *testing.T) {
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

func TestScan_FindsSwiftDotSymbolReference(t *testing.T) {
	root := t.TempDir()
	catalog := filepath.Join(root, "App", "Assets.xcassets")
	if err := os.MkdirAll(filepath.Join(catalog, "betaSubscriptionIcon.imageset"), 0o755); err != nil {
		t.Fatalf("mkdir asset set: %v", err)
	}
	swiftPath := filepath.Join(root, "App", "Subscription.swift")
	if err := os.WriteFile(swiftPath, []byte("return .betaSubscriptionIcon"), 0o644); err != nil {
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

func TestScan_FindsSwiftDotSymbolReference_ForDashedAssetNameCamelCase(t *testing.T) {
	root := t.TempDir()
	catalog := filepath.Join(root, "App", "Assets.xcassets")
	if err := os.MkdirAll(filepath.Join(catalog, "FSM-Onboarding-2.imageset"), 0o755); err != nil {
		t.Fatalf("mkdir asset set: %v", err)
	}
	swiftPath := filepath.Join(root, "App", "PageViewController.swift")
	if err := os.WriteFile(swiftPath, []byte("let images: [ImageResource] = [.fsmOnboarding2]"), 0o644); err != nil {
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
