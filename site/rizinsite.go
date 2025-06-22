package site

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/rizinorg/rz-pm/db"
	"github.com/rizinorg/rz-pm/pkg"
	"github.com/rizinorg/rz-pm/rizin"
	"github.com/rizinorg/rz-pm/utils"
)

const dbDir string = "rz-pm-db"
const artifactsDir string = "artifacts"
const installedFile string = "installed"

type RizinSite struct {
	Path              string
	Database          db.Database
	PkgConfigPath     string
	CMakePath         string
	installedPackages []pkg.InstalledPackage

	rzInfo rizin.RizinInfo
	lock   *SiteLock
}

func InitSite(path string, updateDB bool) (Site, error) {
	// create the filesystem structure
	dbSubdir := filepath.Join(path, dbDir)
	artifactsSubdir := filepath.Join(path, artifactsDir)
	installedFilePath := filepath.Join(path, installedFile)

	paths := []string{
		path,
		dbSubdir,
		artifactsSubdir,
	}

	for _, p := range paths {
		if err := os.MkdirAll(p, 0755); err != nil {
			return &RizinSite{}, fmt.Errorf("could not create %s: %w", p, err)
		}
	}

	// lock the site directory
	siteLock := newSiteLock(path)
	err := siteLock.Lock()
	if err != nil {
		if err == ErrSiteLocked {
			fmt.Println("Site directory is already locked, another instance of rz-pm might be running or the site directory is locked.")
			fmt.Println("If you are sure that no other instance is running, you can remove the lock file manually.")
			fmt.Println("Lock file is located at:", filepath.Join(path, "site.lock"))
			return &RizinSite{}, fmt.Errorf("can't operate on site directory %s: %w", path, err)
		}
	}

	cleanup := func(err error) (*RizinSite, error) {
		_ = siteLock.Unlock()
		return &RizinSite{}, err
	}

	rizinInfo, err := rizin.GetRizinInfo()
	if err != nil {
		return cleanup(fmt.Errorf("failed to get rizin info: %w", err))
	}

	installedPackages, err := getInstalledPackages(installedFilePath, rizinInfo.Version)
	if err != nil {
		return cleanup(fmt.Errorf("failed to get installed packages: %w", err))
	}

	d, err := db.InitDatabase(dbSubdir, rizinInfo.Version)
	if err != nil {
		return cleanup(fmt.Errorf("failed to initialize database: %w", err))
	}

	if updateDB {
		err = d.UpdateDatabase(rizinInfo.Version)
		if err != nil {
			return cleanup(fmt.Errorf("failed to update database: %w", err))
		}
	}

	pkgConfigPath, err := getPkgConfigPath(&rizinInfo)
	if err != nil {
		return cleanup(fmt.Errorf("failed to get pkg-config path: %w", err))
	}

	cmakePath, err := getCMakePath(&rizinInfo)
	if err != nil {
		return cleanup(fmt.Errorf("failed to get CMake path: %w", err))
	}

	s := RizinSite{
		Path:              path,
		Database:          d,
		PkgConfigPath:     pkgConfigPath,
		CMakePath:         cmakePath,
		installedPackages: installedPackages,
		rzInfo:            rizinInfo,
		lock:              siteLock,
	}

	return &s, nil
}

func (s *RizinSite) ListAvailablePackages() ([]pkg.Package, error) {
	res, err := s.Database.ListAvailablePackages()
	if err != nil {
		return []pkg.Package{}, err
	}

	for i := range s.installedPackages {
		_, err := s.Database.GetPackage(s.installedPackages[i].InstalledName)
		if err != nil {
			res = append(res, s.installedPackages[i])
		}
	}

	return res, nil
}

func (s *RizinSite) ListInstalledPackages() ([]pkg.Package, error) {
	installedPackages := make([]pkg.Package, len(s.installedPackages))
	for i := range s.installedPackages {
		pkg, err := s.Database.GetPackage(s.installedPackages[i].InstalledName)
		if err != nil {
			installedPackages[i] = s.installedPackages[i]
		} else {
			installedPackages[i] = pkg
		}
	}
	return installedPackages, nil
}

func (s *RizinSite) RizinVersion() string {
	return s.rzInfo.Version
}

func (s *RizinSite) IsPackageInstalled(pkg pkg.Package) bool {
	name := pkg.Name()
	_, err := s.GetInstalledPackage(name)
	return err == nil
}

func (s *RizinSite) GetPackage(name string) (pkg.Package, error) {
	return s.Database.GetPackage(name)
}

func (s *RizinSite) GetPackageFromFile(filename string) (pkg.Package, error) {
	return pkg.ParsePackageFile(filename)
}

func (s *RizinSite) GetBaseDir() string {
	return s.Path
}

func (s *RizinSite) GetArtifactsDir() string {
	return filepath.Join(s.Path, artifactsDir)
}

func (s *RizinSite) GetPkgConfigDir() string {
	return s.PkgConfigPath
}

func (s *RizinSite) GetCMakeDir() string {
	return s.CMakePath
}

func (s *RizinSite) InstallPackage(p pkg.Package) error {
	if s.IsPackageInstalled(p) {
		return fmt.Errorf("package %s already installed", p.Name())
	}

	files, err := p.Install(s.buildConfig())
	if err != nil {
		return err
	}

	minorVersion := utils.GetMajorMinorVersion(s.RizinVersion())
	s.installedPackages = append(s.installedPackages, pkg.InstalledPackage{
		InstalledName:  p.Name(),
		InstalledFiles: &files,
		RizinVersion:   &minorVersion,
	})
	installedFilePath := filepath.Join(s.Path, installedFile)
	return updateInstalledPackages(installedFilePath, s.installedPackages)
}

func (s *RizinSite) UninstallPackage(pkg pkg.Package) error {
	if !s.IsPackageInstalled(pkg) {
		return fmt.Errorf("package %s not installed", pkg.Name())
	}

	installedPackage, err := s.GetInstalledPackage(pkg.Name())
	if err != nil {
		return err
	}

	if installedPackage.InstalledFiles == nil {
		// NOTE: kept for compatibility with v0.1.9
		err = pkg.Uninstall(s.buildConfig())
		if err != nil {
			return err
		}
	} else {
		fmt.Printf("Uninstalling %s...\n", pkg.Name())
		for _, file := range *installedPackage.InstalledFiles {
			os.RemoveAll(file)
		}

	}

	s.installedPackages = removePackageFromSlice(s.installedPackages, pkg.Name())
	fmt.Printf("Package %s uninstalled.\n", pkg.Name())

	installedFilePath := filepath.Join(s.Path, installedFile)
	return updateInstalledPackages(installedFilePath, s.installedPackages)
}

func (s *RizinSite) CleanPackage(pkg pkg.Package) error {
	pkgArtifactsPath := filepath.Join(s.GetArtifactsDir(), pkg.Name(), pkg.Version())
	_, err := os.Stat(pkgArtifactsPath)
	if err != nil {
		return fmt.Errorf("package %s does not have any build artifacts", pkg.Name())
	}

	err = os.RemoveAll(pkgArtifactsPath)
	if err != nil {
		return fmt.Errorf("failed to remove build artifacts for package %s", pkg.Name())
	}

	return nil
}

func (s *RizinSite) Remove() error {
	return os.RemoveAll(s.Path)
}

func (s *RizinSite) Close() error {
	if s.lock == nil {
		panic("site lock is nil, cannot close")
	}

	err := s.lock.Unlock()
	if err != nil {
		return fmt.Errorf("failed to unlock site: %w", err)
	}

	return nil
}

func (s *RizinSite) buildConfig() pkg.BuildConfig {
	return pkg.BuildConfig{
		ArtifactsDir: s.GetArtifactsDir(),
		PkgConfigDir: s.GetPkgConfigDir(),
		CMakeDir:     s.GetCMakeDir(),
	}
}

func getPkgConfigPath(info *rizin.RizinInfo) (string, error) {
	libPath := info.LibDir

	pkgConfigPath := filepath.Join(libPath, "pkgconfig")
	_, err := os.Stat(pkgConfigPath)
	if os.IsNotExist(err) {
		return "", nil
	} else if err != nil {
		return "", err
	}
	return pkgConfigPath, nil
}

func getCMakePath(info *rizin.RizinInfo) (string, error) {
	libPath := info.LibDir

	cmakePath := filepath.Join(libPath, "cmake")
	_, err := os.Stat(cmakePath)
	if os.IsNotExist(err) {
		return "", nil
	} else if err != nil {
		return "", err
	}
	return cmakePath, nil
}

func getInstalledPackages(path string, rizinVersion string) ([]pkg.InstalledPackage, error) {
	_, err := os.Stat(path)
	if os.IsNotExist(err) {
		return []pkg.InstalledPackage{}, nil
	}

	by, err := os.ReadFile(path)
	if err != nil {
		return []pkg.InstalledPackage{}, err
	}

	var v []pkg.InstalledPackage
	err = json.Unmarshal(by, &v)
	if err != nil {
		var vs []string
		err = json.Unmarshal(by, &vs)
		if err != nil {
			return []pkg.InstalledPackage{}, err
		}

		v = []pkg.InstalledPackage{}
		for _, s := range vs {
			if s != "" {
				v = append(v, pkg.InstalledPackage{
					InstalledName:  s,
					InstalledFiles: nil,
					RizinVersion:   nil,
				})
			}
		}
	}

	version := utils.GetMajorMinorVersion(rizinVersion)
	for i := range v {
		if v[i].RizinVersion == nil {
			v[i].RizinVersion = &version
		}
	}

	return v, nil
}

func updateInstalledPackages(path string, packages []pkg.InstalledPackage) error {
	by, err := json.Marshal(packages)
	if err != nil {
		return err
	}
	err = os.WriteFile(path, by, fs.FileMode(0622))
	if err != nil {
		return err
	}
	return err
}

func removePackageFromSlice(sl []pkg.InstalledPackage, name string) []pkg.InstalledPackage {
	for i := range sl {
		if sl[i].InstalledName == name {
			ret := make([]pkg.InstalledPackage, 0)
			if i > 0 {
				ret = append(ret, sl[:i]...)
			}
			if i < len(sl)-1 {
				ret = append(ret, sl[i+1:]...)
			}
			return ret
		}
	}
	return sl
}

func (s *RizinSite) GetInstalledPackage(name string) (pkg.InstalledPackage, error) {
	for _, v := range s.installedPackages {
		if v.InstalledName == name {
			return v, nil
		}
	}
	return pkg.InstalledPackage{}, fmt.Errorf("installed package %s not found", name)
}
