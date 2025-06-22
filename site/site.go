package site

import (
	"io"
	"os"
	"path/filepath"

	"github.com/adrg/xdg"
	"github.com/rizinorg/rz-pm/pkg"
)

const (
	SiteDirEnvVar = "RZPM_SITEDIR"
)

func SiteDir() string {
	if envVar := os.Getenv(SiteDirEnvVar); envVar != "" {
		return envVar
	}

	return filepath.Join(xdg.DataHome, "rz-pm", "site")
}

type Site interface {
	io.Closer
	ListAvailablePackages() ([]pkg.Package, error)
	ListInstalledPackages() ([]pkg.Package, error)
	IsPackageInstalled(pkg pkg.Package) bool
	GetPackage(name string) (pkg.Package, error)
	GetPackageFromFile(filename string) (pkg.Package, error)
	GetInstalledPackage(name string) (pkg.InstalledPackage, error)
	GetBaseDir() string
	GetArtifactsDir() string
	GetPkgConfigDir() string
	GetCMakeDir() string
	InstallPackage(pkg pkg.Package) error
	UninstallPackage(pkg pkg.Package) error
	CleanPackage(pkg pkg.Package) error
	Remove() error
	RizinVersion() string
}
