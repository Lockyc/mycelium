package scan

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/lockyc/mycelium/internal/graph"
)

// factsRepo commits files (path -> body) into a fresh repo and returns it.
func factsRepo(t *testing.T, files map[string]string) Repo {
	t.Helper()
	dir := filepath.Join(t.TempDir(), "o", "r")
	mkWorking(t, dir)
	for p, body := range files {
		full := filepath.Join(dir, p)
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	run(t, dir, "add", "-A")
	run(t, dir, "-c", "user.email=t@t", "-c", "user.name=t", "commit", "-q", "--allow-empty", "-m", "x")
	return Repo{Dir: dir, Owner: "o", Name: "r"}
}

func TestFactsTauriApp(t *testing.T) {
	r := factsRepo(t, map[string]string{
		"src-tauri/tauri.conf.json": `{"plugins":{"updater":{"pubkey":"x"}}}`,
		"src-tauri/Cargo.toml":      "[package]\nname=\"a\"\n",
	})
	got := DetectFacts(r, "HEAD")
	want := graph.Facts{MacApp: true, Updater: "tauri", Ecosystems: []string{"rust"}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %+v want %+v", got, want)
	}
}

func TestFactsSwiftSparkle(t *testing.T) {
	r := factsRepo(t, map[string]string{
		"project.yml":    "targets:\n  App:\n    type: application\n    platform: macOS\n",
		"App/Info.plist": "<key>SUPublicEDKey</key><string>k</string>",
	})
	got := DetectFacts(r, "HEAD")
	want := graph.Facts{MacApp: true, Updater: "sparkle", Ecosystems: []string{"swift"}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %+v want %+v", got, want)
	}
}

func TestFactsElectron(t *testing.T) {
	r := factsRepo(t, map[string]string{
		"justfile":                              "install:\n\tcp -R dist/App.app /Applications/\n",
		"electron/electron-builder.config.json": `{"publish":{"provider":"generic"}}`,
		"electron/package.json":                 `{"name":"x"}`,
	})
	got := DetectFacts(r, "HEAD")
	want := graph.Facts{MacApp: true, Updater: "electron", Ecosystems: []string{"node"}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %+v want %+v", got, want)
	}
}

func TestFactsIOSOnlyXcodeIsNotMacApp(t *testing.T) {
	r := factsRepo(t, map[string]string{
		"ios/App/App.xcodeproj/project.pbxproj": "SDKROOT = iphoneos;\nproductType = \"com.apple.product-type.application\";\n",
	})
	if got := DetectFacts(r, "HEAD"); got.MacApp {
		t.Fatalf("iOS-only project reported as Mac app: %+v", got)
	}
}

func TestFactsMacXcodeprojIsMacApp(t *testing.T) {
	r := factsRepo(t, map[string]string{
		"App.xcodeproj/project.pbxproj": "SDKROOT = macosx;\nproductType = \"com.apple.product-type.application\";\n",
	})
	if got := DetectFacts(r, "HEAD"); !got.MacApp {
		t.Fatalf("macOS project not a Mac app: %+v", got)
	}
}

func TestFactsVersionSource(t *testing.T) {
	cases := []struct {
		name  string
		files map[string]string
		want  bool
	}{
		{"VERSION", map[string]string{"VERSION": "1.0.0\n"}, true},
		{"cargo workspace", map[string]string{"Cargo.toml": "[workspace.package]\nversion = \"1.0.0\"\n"}, true},
		{"package.json", map[string]string{"package.json": `{"version":"1.0.0"}`}, true},
		{"cargo package", map[string]string{"Cargo.toml": "[package]\nversion = \"1.0.0\"\n"}, true},
		{"dependency version only", map[string]string{"Cargo.toml": "[package]\nname = \"a\"\n[dependencies.foo]\nversion = \"1\"\n"}, false},
		{"tauri beside cargo", map[string]string{"src-tauri/tauri.conf.json": "{}", "src-tauri/Cargo.toml": "[package]\nversion = \"1.0.0\"\n"}, true},
		{"tauri inherits workspace", map[string]string{
			"Cargo.toml":                 "[workspace.package]\nversion = \"1.0.0\"\n",
			"crates/app/tauri.conf.json": "{}",
			"crates/app/Cargo.toml":      "[package]\nversion.workspace = true\n"}, true},
		{"tauri inherits nothing", map[string]string{
			"src-tauri/tauri.conf.json": "{}",
			"src-tauri/Cargo.toml":      "[package]\nversion.workspace = true\n"}, false},
		{"tauri conf version", map[string]string{"src-tauri/tauri.conf.json": `{"version":"1.0.0"}`}, true},
		{"none", map[string]string{"README.md": "x", "package.json": `{"name":"x"}`}, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := DetectFacts(factsRepo(t, c.files), "HEAD").VersionSource; got != c.want {
				t.Fatalf("VersionSource = %v, want %v", got, c.want)
			}
		})
	}
}

func TestFactsEcosystems(t *testing.T) {
	r := factsRepo(t, map[string]string{
		"go.mod":                      "module x\n",
		"package.json":                `{}`,
		"node_modules/dep/Cargo.toml": "",
		"vendor/dep/Package.swift":    "",
	})
	want := []string{"go", "node"}
	if got := DetectFacts(r, "HEAD").Ecosystems; !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v want %v", got, want)
	}
}

func TestFactsElectronYAML(t *testing.T) {
	r := factsRepo(t, map[string]string{
		"electron/electron-builder.yml": "appId: x\npublish:\n  provider: generic\n",
	})
	if got := DetectFacts(r, "HEAD").Updater; got != "electron" {
		t.Fatalf("Updater = %q", got)
	}
}

func TestFactsJustfileMacApp(t *testing.T) {
	cases := []struct {
		name, justfile string
		want           bool
	}{
		{"finds Chrome only", "chrome := \"/Applications/Google Chrome.app\"\ntest:\n\t{{chrome}}/x\n", false},
		{"find line mentions path", "run:\n\ttest -d \"/Applications/Google Chrome.app\" && echo ok\n", false},
		{"comment", "# cp build/X.app /Applications/\nbuild:\n\tmake\n", false},
		{"indented comment", "install:\n    # ditto a /Applications/\n\tmake\n", false},
		{"ditto", "install:\n\tditto build/X.app /Applications/X.app\n", true},
		{"cp", "install:\n\tcp -R build/X.app \"/Applications/{{app}}.app\"\n", true},
		{"installer script", "install:\n\tscripts/install-app.sh\n", true},
		{"verb is a whole word", "x:\n\techo /Applications/mvp\n", false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			r := factsRepo(t, map[string]string{"justfile": c.justfile})
			if got := DetectFacts(r, "HEAD").MacApp; got != c.want {
				t.Fatalf("MacApp = %v, want %v", got, c.want)
			}
		})
	}
}
