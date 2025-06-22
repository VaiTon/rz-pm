package pkg

import (
	"fmt"
)

type InstalledPackage struct {
	InstalledName  string    `json:"name"`
	InstalledFiles *[]string `json:"files"`
	RizinVersion   *string   `json:"rizin_version"`
}

var ErrInvalid = fmt.Errorf("cannot call this method on an installed package")

func (rp InstalledPackage) Name() string                       { return rp.InstalledName }
func (InstalledPackage) Version() string                       { return "" }
func (InstalledPackage) Description() string                   { return "" }
func (InstalledPackage) Summary() string                       { return "" }
func (InstalledPackage) Source() RizinPackageSource            { return RizinPackageSource{} }
func (InstalledPackage) Download(string) error                 { return ErrInvalid }
func (InstalledPackage) Build(BuildConfig) error               { return fmt.Errorf("cannot be called") }
func (InstalledPackage) Install(BuildConfig) ([]string, error) { return nil, ErrInvalid }
func (InstalledPackage) Uninstall(BuildConfig) error           { return ErrInvalid }
