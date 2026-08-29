//go:build !windows

package syncer

func CfapiProviderInstalled() bool { return false }
