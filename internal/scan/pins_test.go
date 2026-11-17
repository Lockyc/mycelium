package scan

import (
	"reflect"
	"testing"
)

func TestPinsCargoGitDeps(t *testing.T) {
	r := factsRepo(t, map[string]string{
		"Cargo.toml": "[workspace.dependencies]\nconfig-core = { git = \"ssh://git@github.com/lockyc/config-core.git\" }\n",
		"crates/x/Cargo.toml": "[package]\nname=\"x\"\n[dependencies]\n" +
			"shell-core = { git = \"https://github.com/Lockyc/shell-core\", rev = \"abc\" }\n" +
			"[target.'cfg(unix)'.dependencies]\nchrome-core = { git = \"https://github.com/lockyc/chrome-core\" }\n" +
			"[dev-dependencies]\nshell-core = { git = \"https://github.com/lockyc/shell-core\" }\n",
		"vendor/y/Cargo.toml": "[dependencies]\nz = { git = \"https://github.com/o/z\" }\n",
	})
	got := DetectPins(r, "HEAD")
	want := []string{"github.com/lockyc/chrome-core", "github.com/lockyc/config-core", "github.com/lockyc/shell-core"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v want %v", got, want)
	}
}

func TestPinsIgnoresRegistryDeps(t *testing.T) {
	r := factsRepo(t, map[string]string{
		"Cargo.toml": "[dependencies]\nserde = \"1\"\ntokio = { version = \"1\" }\n",
	})
	if got := DetectPins(r, "HEAD"); len(got) != 0 {
		t.Fatalf("got %v", got)
	}
}
