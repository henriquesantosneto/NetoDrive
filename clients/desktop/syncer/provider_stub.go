//go:build !windows

package syncer

func providerExe() string { return "" }

func cfapiProviderActive() bool { return false }
