package rizin

import (
	"os"
	"path/filepath"
	"testing"
)

// fakeRizinScript returns the contents of a script that mimics "rizin -H" output.
// It generates a shell script for Unix and a batch file for Windows.
func fakeRizinScript(isWindows bool) string {
	if isWindows {
		return `@echo off
if "%1"=="-H" (
echo RZ_VERSION=9.9.9-fake
echo RZ_PREFIX=/fake/prefix
echo RZ_EXTRA_PREFIX=/fake/extra
echo RZ_MAGICPATH=/fake/magic
echo RZ_INCDIR=/fake/include
echo RZ_LIBDIR=/fake/lib
echo RZ_SIGDB=/fake/sigdb
echo RZ_EXTRA_SIGDB=/fake/extra_sigdb
echo RZ_LIBEXT=dll
echo RZ_CONFIGHOME=/fake/config
echo RZ_DATAHOME=/fake/data
echo RZ_CACHEHOME=/fake/cache
echo RZ_LIB_PLUGINS=/fake/lib_plugins
echo RZ_EXTRA_PLUGINS=/fake/extra_plugins
echo RZ_USER_PLUGINS=/fake/user_plugins
echo RZ_IS_PORTABLE=0
)
`
	} else {
		return `#!/bin/bash
if [[ "$1" == "-H" ]]; then
cat <<EOF
RZ_VERSION=9.9.9-fake
RZ_PREFIX=/fake/prefix
RZ_EXTRA_PREFIX=/fake/extra
RZ_MAGICPATH=/fake/magic
RZ_INCDIR=/fake/include
RZ_LIBDIR=/fake/lib
RZ_SIGDB=/fake/sigdb
RZ_EXTRA_SIGDB=/fake/extra_sigdb
RZ_LIBEXT=so
RZ_CONFIGHOME=/fake/config
RZ_DATAHOME=/fake/data
RZ_CACHEHOME=/fake/cache
RZ_LIB_PLUGINS=/fake/lib_plugins
RZ_EXTRA_PLUGINS=/fake/extra_plugins
RZ_USER_PLUGINS=/fake/user_plugins
RZ_IS_PORTABLE=0
EOF
fi
`
	}
}

func TestGetRizinInfo_FakeExecutable(t *testing.T) {
	tmpDir := t.TempDir()
	isWindows := false
	exeName := "rizin"
	scriptMode := os.FileMode(0755)

	if os.PathSeparator == '\\' {
		isWindows = true
		exeName = "rizin.bat"
		// On Windows, .bat files are executable by default
		scriptMode = 0666
	}
	fakeRizinPath := filepath.Join(tmpDir, exeName)

	// Write the fake rizin script
	if err := os.WriteFile(fakeRizinPath, []byte(fakeRizinScript(isWindows)), scriptMode); err != nil {
		t.Fatalf("failed to write fake rizin: %v", err)
	}

	// Prepend tmpDir to PATH
	origPath := os.Getenv("PATH")
	t.Setenv("PATH", tmpDir+string(os.PathListSeparator)+origPath)

	info, err := GetRizinInfo()
	if err != nil {
		t.Fatalf("GetRizinInfo failed: %v", err)
	}

	// Check a few fields for correctness
	if info.Version != "9.9.9-fake" {
		t.Errorf("expected Version=9.9.9-fake, got %q", info.Version)
	}
	if info.Prefix != "/fake/prefix" {
		t.Errorf("expected Prefix=/fake/prefix, got %q", info.Prefix)
	}
	if isWindows {
		if info.LibExt != "dll" {
			t.Errorf("expected LibExt=dll, got %q", info.LibExt)
		}
	} else {
		if info.LibExt != "so" {
			t.Errorf("expected LibExt=.so, got %q", info.LibExt)
		}
	}
	if info.IsPortable != "0" {
		t.Errorf("expected IsPortable=0, got %q", info.IsPortable)
	}
}

func TestGetRizinInfo_NotInPath(t *testing.T) {
	// Remove rizin from PATH
	t.Setenv("PATH", "")

	_, err := GetRizinInfo()
	if err == nil {
		t.Fatal("expected error when rizin is not in PATH, got nil")
	}
	if want := "rizin does not seem to be installed"; err == nil || err.Error()[:len(want)] != want {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestGetRizinInfo_FailsToRun(t *testing.T) {
	tmpDir := t.TempDir()
	fakeRizinPath := filepath.Join(tmpDir, "rizin")

	// Write a script that always fails
	script := "#!/bin/sh\nexit 42\n"
	if err := os.WriteFile(fakeRizinPath, []byte(script), 0755); err != nil {
		t.Fatalf("failed to write fake rizin: %v", err)
	}

	origPath := os.Getenv("PATH")
	t.Setenv("PATH", tmpDir+string(os.PathListSeparator)+origPath)

	_, err := GetRizinInfo()
	if err == nil {
		t.Fatal("expected error when rizin fails to run, got nil")
	}
}
