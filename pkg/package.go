package pkg

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v2"
)

type BuildSystem string

const (
	Meson BuildSystem = "meson"
)

type RizinPackageSource struct {
	URL            string
	Hash           string
	BuildSystem    BuildSystem `yaml:"build_system"`
	BuildArguments []string    `yaml:"build_arguments"`
	Directory      string
}

type BuildConfig struct {
	ArtifactsDir string // Directory where build artifacts will be stored
	PkgConfigDir string // Directory for pkg-config files
	CMakeDir     string // Directory for CMake files
}

type Package interface {
	Name() string                                 // Name returns the name of the package
	Version() string                              // Version returns the version of the package
	Summary() string                              // Summary returns a short summary of the package
	Description() string                          // Description returns a longer description of the package
	Source() RizinPackageSource                   // Source returns the source information of the package
	Download(baseArtifactsPath string) error      // Download downloads the source code of the package and extracts it in the provided path
	Build(config BuildConfig) error               // Build builds the package if a source is provided
	Install(config BuildConfig) ([]string, error) // Install installs the package after building it, returning a list of installed files
	Uninstall(config BuildConfig) error           // Uninstall uninstalls the package, removing installed files
}

func ParsePackageFile(path string) (Package, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return RizinPackage{}, err
	}

	var p RizinPackage
	err = yaml.Unmarshal(content, &p)
	if err != nil {
		return RizinPackage{}, err
	}

	if p.PackageName == "" || p.PackageVersion == "" || p.PackageSummary == "" {
		return RizinPackage{}, fmt.Errorf("wrong file plugin format: name, version, and summary are mandatory")
	}
	if p.PackageSource != nil {
		if p.PackageSource.URL == "" || p.PackageSource.BuildSystem == "" {
			return RizinPackage{}, fmt.Errorf("wrong file plugin format: Source URL and Build System are mandatory")
		}
		if !p.isGitRepo() && p.PackageSource.Hash == "" {
			return RizinPackage{}, fmt.Errorf("wrong file plugin format: Source Hash is mandatory for non-git plugins")
		} else if p.isGitRepo() && p.PackageSource.Hash != "" {
			return RizinPackage{}, fmt.Errorf("wrong file plugin format: Source Hash should not be used for git plugins")
		}
	}
	return p, nil
}
