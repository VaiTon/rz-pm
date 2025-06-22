package site

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/rizinorg/rz-pm/pkg"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func containsPackage(packages []pkg.Package, name string) bool {
	for _, rp := range packages {
		if rp.Name() == name {
			return true
		}
	}
	return false
}

func TestEmptySite(t *testing.T) {
	tmpPath, err := os.MkdirTemp(os.TempDir(), "rzpmtest")
	require.NoError(t, err, "temp path should be created")
	defer os.RemoveAll(tmpPath)
	site, err := InitSite(tmpPath, true)
	require.NoError(t, err, "site should be initialized in tmpPath %s", err)
	assert.Equal(t, tmpPath, site.GetBaseDir(), "site path should be tmpPath")
	_, err = os.Stat(filepath.Join(tmpPath, "rz-pm-db"))
	assert.NoError(t, err, "rz-pm database directory should be there")
	_, err = os.Stat(filepath.Join(tmpPath, "rz-pm-db", "README.md"))
	assert.NoError(t, err, "rz-pm-db repository should be downloaded")
	_, err = os.Stat(filepath.Join(tmpPath, "rz-pm-db", "db"))
	assert.NoError(t, err, "rz-pm-db repository should be downloaded 2")
}

func TestExistingSite(t *testing.T) {
	tmpPath, err := os.MkdirTemp(os.TempDir(), "rzpmtest")
	require.Nil(t, err, "temp path should be created")
	defer os.RemoveAll(tmpPath)
	site, err := InitSite(tmpPath, true)
	require.Nil(t, err, "site should be initialized when dir is empty")
	site.Close()
	site, err = InitSite(tmpPath, true)
	assert.Nil(t, err, "site should be initialized even when dir is already initialized")
	site.Close()
	_, err = os.Stat(filepath.Join(tmpPath, "rz-pm-db", "README.md"))
	assert.Nil(t, err, "rz-pm-db repository should be downloaded")
	_, err = os.Stat(filepath.Join(tmpPath, "rz-pm-db", "db"))
	assert.Nil(t, err, "rz-pm-db repository should be downloaded 2")
}

func TestLockedSite(t *testing.T) {
	tmpPath, err := os.MkdirTemp(os.TempDir(), "rzpmtest")
	require.Nil(t, err, "temp path should be created")
	defer os.RemoveAll(tmpPath)
	// create a new site
	site, err := InitSite(tmpPath, true)
	require.Nil(t, err, "site should be initialized")
	// create a new site with the same path
	site2, err := InitSite(tmpPath, true)
	assert.True(t, errors.Is(err, ErrSiteLocked), "site should be locked when another instance is running")
	site.Close()
	// try to create a new site with the same path again
	site2, err = InitSite(tmpPath, true)
	assert.Nil(t, err, "site should be initialized after the first instance is closed")
	site2.Close()
}

func TestListPackages(t *testing.T) {
	tmpPath, err := os.MkdirTemp(os.TempDir(), "rzpmtest")
	require.Nil(t, err, "temp path should be created")
	defer os.RemoveAll(tmpPath)
	site, err := InitSite(tmpPath, true)
	require.Nil(t, err, "site should be initialized when dir is empty")

	packages, err := site.ListAvailablePackages()
	assert.Nil(t, err, "no errors while retrieving packages")
	assert.True(t, len(packages) > 0, "there should be at least one package in the database")
	assert.True(t, containsPackage(packages, "jsdec"), "jsdec package should be present in the database")
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

	p, err := pkg.ParsePackageFile(tmpFile.Name())
	require.NoError(t, err, "no errors in parsing the above package file")
	assert.Equal(t, "simple", p.Name())
	assert.Equal(t, "0.0.1", p.Version())
	assert.Equal(t, "simple description", p.Summary())
	assert.Equal(t, "https://github.com/rizinorg/jsdec", p.Source().URL)
	assert.Equal(t, "0f966e3c2c649cafa21c4466b783330c2b21baea", p.Source().Hash)
	assert.Equal(t, pkg.Meson, p.Source().BuildSystem)
	assert.Contains(t, p.Source().BuildArguments, "-Dstandalone=false")
	assert.Equal(t, "jsdec-0.7.0/", p.Source().Directory)
}

type FakePackage struct {
	myName string
}

func (fp FakePackage) Name() string                              { return fp.myName }
func (fp FakePackage) Version() string                           { return "" }
func (fp FakePackage) Summary() string                           { return "" }
func (fp FakePackage) Description() string                       { return "" }
func (fp FakePackage) Source() pkg.RizinPackageSource            { return pkg.RizinPackageSource{} }
func (fp FakePackage) Download(string) error                     { return nil }
func (fp FakePackage) Build(pkg.BuildConfig) error               { return nil }
func (fp FakePackage) Install(pkg.BuildConfig) ([]string, error) { return nil, nil }
func (fp FakePackage) Uninstall(pkg.BuildConfig) error           { return nil }

func TestListInstalledPackages(t *testing.T) {
	tmpPath, err := os.MkdirTemp(os.TempDir(), "rzpmtest")
	require.Nil(t, err, "temp path should be created")
	defer os.RemoveAll(tmpPath)
	site, err := InitSite(tmpPath, true)
	require.Nil(t, err, "site should be initialized when dir is empty")

	pkg := FakePackage{myName: "jsdec"}

	err = site.InstallPackage(pkg)
	require.NoError(t, err)

	packages, err := site.ListAvailablePackages()
	assert.NoError(t, err, "no errors while retrieving packages")
	assert.True(t, len(packages) > 0, "there should be at least one package in the database")

	installedPackages, err := site.ListInstalledPackages()
	assert.NoError(t, err, "no errors while retrieving installed packages")
	assert.Len(t, installedPackages, 1, "there should be just one package installed")
	assert.Equal(t, "jsdec", installedPackages[0].Name(), "jsdec package should be installed")
}
