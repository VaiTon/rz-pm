package pkg

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDownloadSimplePackage(t *testing.T) {
	p := RizinPackage{
		PackageName:        "simple",
		PackageDescription: "simple description",
		PackageVersion:     "0.0.1",
		PackageSource: &RizinPackageSource{
			URL:            "https://github.com/rizinorg/jsdec/archive/refs/tags/v0.4.0.tar.gz",
			Hash:           "5afe9a823c1c31ccf641dc1667a092418cd84f5cb9865730580783ca7c44e93d",
			BuildSystem:    "meson",
			Directory:      "p",
			BuildArguments: []string{"-Djsc_folder=.."},
		},
	}

	tmpPath, err := os.MkdirTemp(os.TempDir(), "rzpmtest-artifacts")
	require.NoError(t, err, "temp path should be created")
	defer os.RemoveAll(tmpPath)

	err = p.Download(tmpPath)
	assert.NoError(t, err, "simple package should be downloaded")
	_, err = os.Stat(filepath.Join(tmpPath, "simple", "0.0.1", "jsdec-0.4.0"))
	assert.NoError(t, err, "jsdec release should be downloaded and extracted")
	_, err = os.Stat(filepath.Join(tmpPath, "simple", "0.0.1", "jsdec-0.4.0", "p"))
	assert.NoError(t, err, "jsdec/p should be there")
}

func TestWrongHash(t *testing.T) {
	p := RizinPackage{
		PackageName:        "simple",
		PackageDescription: "simple description",
		PackageVersion:     "0.0.1",
		PackageSource: &RizinPackageSource{
			URL:            "https://github.com/rizinorg/jsdec/archive/refs/tags/v0.8.0.tar.gz",
			Hash:           "sha256:2b2587dd117d48b284695416a7349a21c4dd30fbe75cc5890ed74945c9b474aa",
			BuildSystem:    "meson",
			Directory:      "p",
			BuildArguments: []string{"-Djsc_folder=.."},
		},
	}

	tmpPath, err := os.MkdirTemp(os.TempDir(), "rzpmtest")
	require.Nil(t, err, "temp path should be created")
	defer os.RemoveAll(tmpPath)

	installPath, err := os.MkdirTemp(os.TempDir(), "rzpmtest-install")
	require.Nil(t, err, "install path should be created")
	defer os.RemoveAll(installPath)

	err = p.Download(tmpPath)
	assert.ErrorIs(t, err, ErrRizinPackageWrongHash, "wrong hash should be detected")
}

// tarGzDir creates a .tar.gz archive at destFile containing the contents of srcDir.
func tarGzDir(destFile, srcDir string) error {
	f, err := os.Create(destFile)
	if err != nil {
		return err
	}
	defer f.Close()

	gw := gzip.NewWriter(f)
	defer gw.Close()
	tw := tar.NewWriter(gw)
	defer tw.Close()

	return filepath.Walk(srcDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		relPath, err := filepath.Rel(srcDir, path)
		if err != nil {
			return err
		}
		if relPath == "." {
			return nil
		}
		header, err := tar.FileInfoHeader(info, path)
		if err != nil {
			return err
		}
		header.Name = relPath
		if err := tw.WriteHeader(header); err != nil {
			return err
		}
		if info.Mode().IsRegular() {
			f, err := os.Open(path)
			if err != nil {
				return err
			}
			defer f.Close()
			_, err = io.Copy(tw, f)
			return err
		}
		return nil
	})
}

func calcFileSha256(t *testing.T, name string) string {
	f, err := os.Open(name)
	if err != nil {
		t.Fatalf("failed to open file: %v", err)
	}
	defer f.Close()

	content, err := io.ReadAll(f)
	require.NoError(t, err, "file should be read")

	h := sha256.Sum256(content)
	return fmt.Sprintf("%x", h)
}

func serveDirAsTarGz(t *testing.T, ctx context.Context, dir string) (url string, hash string, err error) {
	tmpFile, err := os.CreateTemp("", "rzpmtest-*.tar.gz")
	if err != nil {
		return "", "", fmt.Errorf("failed to create temp file: %w", err)
	}

	err = tarGzDir(tmpFile.Name(), dir)
	if err != nil {
		return "", "", fmt.Errorf("failed to create tar.gz: %w", err)
	}

	hash = calcFileSha256(t, tmpFile.Name())

	// Open an HTTP server and serve the file at a random URL
	mux := http.NewServeMux()
	fileName := filepath.Base(tmpFile.Name())
	mux.HandleFunc("/"+fileName, func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, tmpFile.Name())
	})

	server := &http.Server{
		Addr:    "127.0.0.1:0",
		Handler: mux,
	}

	ln, err := net.Listen("tcp", server.Addr)
	if err != nil {
		return "", "", fmt.Errorf("failed to listen: %w", err)
	}

	go func() {
		<-ctx.Done()
		server.Shutdown(context.Background())
		tmpFile.Close()
	}()

	go server.Serve(ln)
	url = fmt.Sprintf("http://%s/%s", ln.Addr().String(), fileName)
	return url, hash, nil
}

func createTestPackage(t *testing.T, ctx context.Context) RizinPackage {
	pwd, err := os.Getwd()
	require.NoError(t, err, "current working directory should be retrieved")

	dir := filepath.Join(filepath.Dir(pwd), "simpleplugin")
	require.DirExists(t, dir, "test package directory should exist")

	serveURL, hash, err := serveDirAsTarGz(t, ctx, dir)
	require.NoError(t, err, "failed to serve test package directory as tar.gz")

	simplePackage := RizinPackage{
		PackageName:        "simple",
		PackageDescription: "simple description",
		PackageVersion:     "0.0.1",
		PackageSource: &RizinPackageSource{
			URL:            serveURL,
			Hash:           hash,
			BuildSystem:    "meson",
			Directory:      "",
			BuildArguments: []string{"-Drizin_plugdir="},
		},
	}

	return simplePackage
}

func TestInstallSimplePackage(t *testing.T) {
	log.SetOutput(os.Stderr)
	p := createTestPackage(t, context.Background())

	tmpPath, err := os.MkdirTemp(os.TempDir(), "rzpmtest-build")
	require.NoError(t, err, "temp path should be created")
	fmt.Printf("Temporary path for build: %s\n", tmpPath)
	defer os.RemoveAll(tmpPath)

	pluginsPath, err := os.MkdirTemp(os.TempDir(), "rzpmtest-install")
	require.NoError(t, err, "install path should be created")
	fmt.Printf("Temporary path for plugins: %s\n", pluginsPath)
	defer os.RemoveAll(pluginsPath)
	p.PackageSource.BuildArguments[0] += pluginsPath

	err = p.Download(tmpPath)
	require.NoError(t, err, "package should be downloaded")

	installedFiles, err := p.Install(BuildConfig{ArtifactsDir: tmpPath})
	require.NoError(t, err, "The plugin should be built and installed without errors")

	files, err := os.ReadDir(pluginsPath)
	fmt.Printf("Installed files: %v\n", installedFiles)
	fmt.Printf("Found files in pluginsPath: %v\n", files)

	require.NoError(t, err, "pluginsPath should be read")
	require.True(t, len(files) >= 1, "there should be one plugin installed")

	// check that every file in pluginsPath is in installedFiles
	installedFileMap := make(map[string]bool)
	for _, file := range installedFiles {
		name := filepath.Base(file)
		installedFileMap[name] = true
	}

	for _, file := range files {
		assert.Contains(t, installedFileMap, file.Name(), "installed files should match the files in pluginsPath")
	}

	file := files[0]
	if runtime.GOOS == "windows" {
		assert.Contains(t, file.Name(), "plugin.dll", "the name of the plugin should contain 'plugin.dll'")
	} else if runtime.GOOS == "darwin" {
		assert.Contains(t, file.Name(), "libplugin.dylib", "the name of the plugin should contain 'plugin.dylib'")
	} else {
		assert.Contains(t, file.Name(), "libplugin.so", "the name of the plugin should contain 'plugin.so'")
	}

	for _, file := range files {
		assert.Contains(t, file.Name(), "plugin", "the name of the plugin should contain 'plugin'")
	}

	for _, file := range installedFiles {
		assert.Contains(t, file, "plugin", "the name of the plugin should contain 'plugin'")
	}
}

func TestUninstallSimplePackage(t *testing.T) {
	log.SetOutput(os.Stderr)
	p := createTestPackage(t, context.Background())

	tmpPath, err := os.MkdirTemp(os.TempDir(), "rzpmtest")
	require.NoError(t, err, "temp path should be created")
	defer os.RemoveAll(tmpPath)

	pluginsPath, err := os.MkdirTemp(os.TempDir(), "rzpmtest-install")
	require.NoError(t, err, "install path should be created")
	defer os.RemoveAll(pluginsPath)
	p.PackageSource.BuildArguments[0] += pluginsPath

	err = p.Download(tmpPath)
	require.NoError(t, err, "package should be downloaded")

	buildConfig := BuildConfig{
		ArtifactsDir: tmpPath,
		PkgConfigDir: filepath.Join(pluginsPath, "pkg-config-dir"),
		CMakeDir:     filepath.Join(pluginsPath, "cmake-dir"),
	}

	_, err = p.Install(buildConfig)
	assert.NoError(t, err, "The plugin should be built and installed without errors")

	err = p.Uninstall(buildConfig)
	assert.NoError(t, err, "The plugin should be uninstalled without errors")

	files, err := os.ReadDir(pluginsPath)
	require.NoError(t, err, "pluginsPath should be read")
	require.Len(t, files, 0, "there should be one plugins installed")
}

func TestDownloadGitPackage(t *testing.T) {
	p := RizinPackage{
		PackageName:        "simple-git",
		PackageDescription: "simple-git description",
		PackageVersion:     "dev",
		PackageSource: &RizinPackageSource{
			URL:            "https://github.com/rizinorg/jsdec.git",
			BuildSystem:    "meson",
			Directory:      "",
			BuildArguments: []string{"-Dstandalone=false"},
		},
	}

	tmpPath, err := os.MkdirTemp(os.TempDir(), "rzpmtest")
	require.NoError(t, err, "temp path should be created")
	// defer os.RemoveAll(tmpPath)

	err = p.Download(tmpPath)
	assert.NoError(t, err, "simple package should be downloaded")
	_, err = os.Stat(filepath.Join(tmpPath, "simple-git", "dev", "jsdec"))
	assert.NoError(t, err, "simple-git(jsdec) dir should be there")
	_, err = os.Stat(filepath.Join(tmpPath, "simple-git", "dev", "jsdec", ".git"))
	assert.NoError(t, err, "simple-git(jsdec) master branch should have been git cloned")
	_, err = os.Stat(filepath.Join(tmpPath, "simple-git", "dev", "jsdec", "c"))
	assert.NoError(t, err, "simple-git(jsdec)c should be there")
}

func TestGoodPackageFormat(t *testing.T) {
	tmpFile, err := os.CreateTemp("", "package-format")
	require.NoError(t, err, "temporary file should be created")
	defer tmpFile.Close()

	tmpFile.WriteString(`name: simple
version: 0.0.1
summary: simple description
source:
  url: https://github.com/rizinorg/jsdec
  hash: 0f966e3c2c649cafa21c4466b783330c2b21baea
  build_system: meson
  build_arguments:
    - -Dstandalone=false
  directory: jsdec-0.7.0/
`)

	p, err := ParsePackageFile(tmpFile.Name())
	require.NoError(t, err, "no errors in parsing the above package file")
	assert.Equal(t, "simple", p.Name())
	assert.Equal(t, "0.0.1", p.Version())
	assert.Equal(t, "simple description", p.Summary())
	assert.Equal(t, "https://github.com/rizinorg/jsdec", p.Source().URL)
	assert.Equal(t, "0f966e3c2c649cafa21c4466b783330c2b21baea", p.Source().Hash)
	assert.Equal(t, Meson, p.Source().BuildSystem)
	assert.Contains(t, p.Source().BuildArguments, "-Dstandalone=false")
	assert.Equal(t, "jsdec-0.7.0/", p.Source().Directory)
}

func TestWrongPackageFormat(t *testing.T) {
	tmpFile, err := os.CreateTemp("", "package-format")
	require.NoError(t, err, "temporary file should be created")
	defer tmpFile.Close()

	f1 := `version: 0.0.1
summary: simple description
source:
  url: https://github.com/rizinorg/jsdec/archive/refs/tags/v0.7.0.tar.gz
  hash: sha256:2b2587dd117d48b284695416a7349a21c4dd30fbe75cc5890ed74945c9b474ea
  build_system: meson
  build_arguments:
    - -Dstandalone=false
  directory: jsdec-0.7.0/
`

	f2 := `name: simple
summary: simple description
source:
  url: https://github.com/rizinorg/jsdec/archive/refs/tags/v0.7.0.tar.gz
  hash: sha256:2b2587dd117d48b284695416a7349a21c4dd30fbe75cc5890ed74945c9b474ea
  build_system: meson
  build_arguments:
    - -Dstandalone=false
  directory: jsdec-0.7.0/
`

	f3 := `name: simple
version: 0.0.1
summary: simple description
`

	tmpFile.WriteString(f1)

	_, err = ParsePackageFile(tmpFile.Name())
	assert.Error(t, err, "missing name should fail parsing")

	tmpFile.Truncate(0)
	tmpFile.WriteString(f2)

	_, err = ParsePackageFile(tmpFile.Name())
	assert.Error(t, err, "missing version should fail parsing")

	tmpFile.Truncate(0)
	tmpFile.WriteString(f3)

	_, err = ParsePackageFile(tmpFile.Name())
	assert.Error(t, err, "missing source should fail parsing")
}
